package handler

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	referralmodel "github.com/QuantumNous/new-api/internal/referral/model"
	"github.com/QuantumNous/new-api/internal/skill/enums"
	"github.com/QuantumNous/new-api/internal/skill/errcodes"
	skillmodel "github.com/QuantumNous/new-api/internal/skill/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func purchaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testDownloadDB(t)
	require.NoError(t, db.AutoMigrate(&platformmodel.User{}, &referralmodel.ReferralRecord{}))
	return db
}

func TestPurchaseMarketplaceSkill_OneTimePaid_GrantsOnceAndEmitsPurchased(t *testing.T) {
	db := purchaseTestDB(t)
	SetDB(db)
	s := testSkill("buy-me", "published")
	s.RequiredPlan = enums.RequiredPlanPro
	s.MonetizationType = enums.MonetizationTypeOneTime
	s = createPublishedSkillWithActiveVersionFromSkill(t, db, s, "paid template")
	require.NoError(t, db.Create(&platformmodel.User{Id: 42, Username: "buyer", Quota: oneTimePurchaseQuotaCharge() * 2, Group: "default"}).Error)

	body := `{"idempotency_key":"purchase-key-1","entry_point":"paywall"}`
	c, w := testContext("/api/v1/marketplace/skills/buy-me/purchase")
	c.Params = gin.Params{{Key: "id", Value: "buy-me"}}
	c.Set("id", 42)
	c.Set("group", "default")
	c.Request.Body = io.NopCloser(strings.NewReader(body))
	PurchaseMarketplaceSkill(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"succeeded"`)
	assert.Contains(t, w.Body.String(), `"entitled":true`)

	c2, w2 := testContext("/api/v1/marketplace/skills/buy-me/purchase")
	c2.Params = gin.Params{{Key: "id", Value: "buy-me"}}
	c2.Set("id", 42)
	c2.Set("group", "default")
	c2.Request.Body = io.NopCloser(strings.NewReader(body))
	PurchaseMarketplaceSkill(c2)
	require.Equal(t, http.StatusOK, w2.Code)

	var user platformmodel.User
	require.NoError(t, db.First(&user, 42).Error)
	assert.Equal(t, oneTimePurchaseQuotaCharge(), user.Quota, "duplicate idempotency key must not charge twice")

	var entitlementCount, eventCount, orderCount int64
	require.NoError(t, db.Model(&skillmodel.SkillEntitlement{}).Where("user_id = ? AND skill_id = ?", 42, s.ID).Count(&entitlementCount).Error)
	require.NoError(t, db.Model(&skillmodel.SkillUsageEvent{}).Where("event_type = ? AND skill_id = ?", enums.SkillUsageEventTypePurchased, s.ID).Count(&eventCount).Error)
	require.NoError(t, db.Model(&skillmodel.SkillPurchaseOrder{}).Where("user_id = ? AND idempotency_key = ?", 42, "purchase-key-1").Count(&orderCount).Error)
	assert.Equal(t, int64(1), entitlementCount)
	assert.Equal(t, int64(1), eventCount)
	assert.Equal(t, int64(1), orderCount)

	var event skillmodel.SkillUsageEvent
	require.NoError(t, db.Where("event_type = ? AND skill_id = ?", enums.SkillUsageEventTypePurchased, s.ID).First(&event).Error)
	assert.Equal(t, enums.EntryPointPaywall, event.EntryPoint)
}

func TestPurchaseMarketplaceSkill_ReferralGrantFailureDoesNotRollbackPurchase(t *testing.T) {
	db := purchaseTestDB(t)
	SetDB(db)
	oldRewardKind := common.ReferralRewardKind
	common.ReferralRewardKind = "unsupported_for_test"
	t.Cleanup(func() {
		common.ReferralRewardKind = oldRewardKind
	})

	s := testSkill("referral-fail-buy", "published")
	s.RequiredPlan = enums.RequiredPlanPro
	s.MonetizationType = enums.MonetizationTypeOneTime
	s = createPublishedSkillWithActiveVersionFromSkill(t, db, s, "paid template")
	require.NoError(t, db.Create(&platformmodel.User{Id: 41, Username: "inviter", Quota: 0, Group: "default", AffCode: "INV41"}).Error)
	require.NoError(t, db.Create(&platformmodel.User{Id: 45, Username: "buyer4", Quota: oneTimePurchaseQuotaCharge() * 2, Group: "default"}).Error)
	require.NoError(t, db.Create(&referralmodel.ReferralRecord{
		InviterID:  41,
		InviteeID:  45,
		InviteCode: "INV41",
		Status:     referralmodel.ReferralStatusSignedUp,
	}).Error)

	c, w := testContext("/api/v1/marketplace/skills/referral-fail-buy/purchase")
	c.Params = gin.Params{{Key: "id", Value: "referral-fail-buy"}}
	c.Set("id", 45)
	c.Set("group", "default")
	c.Request.Body = io.NopCloser(strings.NewReader(`{"idempotency_key":"referral-failure-key"}`))
	PurchaseMarketplaceSkill(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"succeeded"`)
	assert.Contains(t, w.Body.String(), `"entitled":true`)

	var user platformmodel.User
	require.NoError(t, db.First(&user, 45).Error)
	assert.Equal(t, oneTimePurchaseQuotaCharge(), user.Quota, "referral grant failure must not roll back wallet debit")

	var order skillmodel.SkillPurchaseOrder
	require.NoError(t, db.Where("user_id = ? AND idempotency_key = ?", 45, "referral-failure-key").First(&order).Error)
	assert.Equal(t, skillmodel.SkillPurchaseStatusSucceeded, order.Status)

	var entitlementCount, eventCount int64
	require.NoError(t, db.Model(&skillmodel.SkillEntitlement{}).Where("user_id = ? AND skill_id = ?", 45, s.ID).Count(&entitlementCount).Error)
	require.NoError(t, db.Model(&skillmodel.SkillUsageEvent{}).Where("event_type = ? AND skill_id = ?", enums.SkillUsageEventTypePurchased, s.ID).Count(&eventCount).Error)
	assert.Equal(t, int64(1), entitlementCount)
	assert.Equal(t, int64(1), eventCount)

	var referral referralmodel.ReferralRecord
	require.NoError(t, db.Where("invitee_id = ?", 45).First(&referral).Error)
	assert.Equal(t, referralmodel.ReferralStatusSignedUp, referral.Status)
	assert.Nil(t, referral.RewardGrantedAt)
	assert.Empty(t, referral.ConversionReference)
}

func TestPlusUpgradeCreditUSD_ConfigurableFromOneTimeEntitlement(t *testing.T) {
	db := purchaseTestDB(t)
	s := testSkill("credit-skill", "published")
	s.MonetizationType = enums.MonetizationTypeOneTime
	s = createPublishedSkillWithActiveVersionFromSkill(t, db, s, "credit template")

	disabledCredit, err := skillmodel.PlusUpgradeCreditUSD(db, 42, false)
	require.NoError(t, err)
	assert.Equal(t, float64(0), disabledCredit)

	emptyCredit, err := skillmodel.PlusUpgradeCreditUSD(db, 42, true)
	require.NoError(t, err)
	assert.Equal(t, float64(0), emptyCredit)

	require.NoError(t, skillmodel.GrantOneTimeEntitlement(db, 42, 42, s.ID, "credit-order"))
	credit, err := skillmodel.PlusUpgradeCreditUSD(db, 42, true)
	require.NoError(t, err)
	assert.Equal(t, oneTimeSkillPurchaseAmountUSD, credit)
}

func TestPurchaseMarketplaceSkill_FailedPayment_GrantsNothing(t *testing.T) {
	db := purchaseTestDB(t)
	SetDB(db)
	s := testSkill("fail-buy", "published")
	s.RequiredPlan = enums.RequiredPlanPro
	s.MonetizationType = enums.MonetizationTypeOneTime
	s = createPublishedSkillWithActiveVersionFromSkill(t, db, s, "failed template")
	require.NoError(t, db.Create(&platformmodel.User{Id: 43, Username: "buyer2", Quota: oneTimePurchaseQuotaCharge() * 2, Group: "default"}).Error)

	c, w := testContext("/api/v1/marketplace/skills/fail-buy/purchase")
	c.Params = gin.Params{{Key: "id", Value: "fail-buy"}}
	c.Set("id", 43)
	c.Set("group", "default")
	c.Request.Body = io.NopCloser(strings.NewReader(`{"idempotency_key":"failed-key","payment_status":"failed"}`))
	PurchaseMarketplaceSkill(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"failed"`)
	assert.Contains(t, w.Body.String(), `"entitled":false`)

	var user platformmodel.User
	require.NoError(t, db.First(&user, 43).Error)
	assert.Equal(t, oneTimePurchaseQuotaCharge()*2, user.Quota)

	var entitlementCount, eventCount int64
	require.NoError(t, db.Model(&skillmodel.SkillEntitlement{}).Where("user_id = ? AND skill_id = ?", 43, s.ID).Count(&entitlementCount).Error)
	require.NoError(t, db.Model(&skillmodel.SkillUsageEvent{}).Where("event_type = ? AND skill_id = ?", enums.SkillUsageEventTypePurchased, s.ID).Count(&eventCount).Error)
	assert.Equal(t, int64(0), entitlementCount)
	assert.Equal(t, int64(0), eventCount)
}

func TestPurchaseMarketplaceSkill_IdempotencyKeyForDifferentSkillConflicts(t *testing.T) {
	db := purchaseTestDB(t)
	SetDB(db)
	first := testSkill("first-buy", "published")
	first.MonetizationType = enums.MonetizationTypeOneTime
	first = createPublishedSkillWithActiveVersionFromSkill(t, db, first, "first template")
	second := testSkill("second-buy", "published")
	second.MonetizationType = enums.MonetizationTypeOneTime
	second = createPublishedSkillWithActiveVersionFromSkill(t, db, second, "second template")
	require.NoError(t, db.Create(&platformmodel.User{Id: 44, Username: "buyer3", Quota: oneTimePurchaseQuotaCharge() * 3, Group: "default"}).Error)

	body := `{"idempotency_key":"shared-key"}`
	c, w := testContext("/api/v1/marketplace/skills/first-buy/purchase")
	c.Params = gin.Params{{Key: "id", Value: "first-buy"}}
	c.Set("id", 44)
	c.Set("group", "default")
	c.Request.Body = io.NopCloser(strings.NewReader(body))
	PurchaseMarketplaceSkill(c)
	require.Equal(t, http.StatusOK, w.Code)

	c2, w2 := testContext("/api/v1/marketplace/skills/second-buy/purchase")
	c2.Params = gin.Params{{Key: "id", Value: "second-buy"}}
	c2.Set("id", 44)
	c2.Set("group", "default")
	c2.Request.Body = io.NopCloser(strings.NewReader(body))
	PurchaseMarketplaceSkill(c2)

	require.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), string(errcodes.ErrSkillConflict))
	var secondEntitlements int64
	require.NoError(t, db.Model(&skillmodel.SkillEntitlement{}).Where("user_id = ? AND skill_id = ?", 44, second.ID).Count(&secondEntitlements).Error)
	assert.Equal(t, int64(0), secondEntitlements)
	assert.NotEmpty(t, first.ID)
}
