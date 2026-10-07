package service

// Tests of Enterprise Org on the billing path (meta-repo
// docs/enterprise-org-prd.md §7.4, card P7): a request made with an
// organization key is paid from the owner's balance — the company wallet —
// recorded under the member who made it, and stamped with the organization
// and the member's department. Each test that is about organization keys runs
// a personal key through the same steps where it can, because the other half
// of the requirement is that a personal account notices none of this.
//
// They use the package's in-memory database (TestMain in
// task_billing_test.go) and are named TestOrg… so both CI workflows select
// them.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cast of these tests. The owner's balance is the company wallet; the
// member holds an organization key; the bystander is a personal account with a
// key of its own, there to show that nothing about it changes.
const (
	walletOrgID        = 9
	walletDepartmentID = 7
	walletOwnerID      = 101
	walletMemberID     = 102
	walletBystanderID  = 103
	walletMemberKeyID  = 201
	walletBystanderKey = 202
	walletMemberKey    = "org-member-key"
	walletPersonalKey  = "personal-key"
)

// memberSpend is what TokenAuth resolves for the member's organization key.
var memberSpend = relaycommon.OrgSpend{OrgId: walletOrgID, OrgDepartmentId: walletDepartmentID, OrgWalletUserId: walletOwnerID}

// seedAccount inserts a user with a name of its own: usernames and referral
// codes are unique, which the package's seedUser does not allow for.
func seedAccount(t *testing.T, id int, name string, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: name, AffCode: name, Quota: quota, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(user).Error)
}

// seedWalletCast inserts the owner, the member with their organization key,
// and the bystander with their personal key. Each key has 5000 left.
func seedWalletCast(t *testing.T, ownerQuota int, memberQuota int, bystanderQuota int) {
	t.Helper()
	// The billing looks a key up by its value, through a column whose quoted
	// name the package's TestMain leaves unset.
	model.InitColumnNames()
	truncate(t)
	seedAccount(t, walletOwnerID, "owner", ownerQuota)
	seedAccount(t, walletMemberID, "member", memberQuota)
	seedAccount(t, walletBystanderID, "bystander", bystanderQuota)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id IN ?", []int{walletOwnerID, walletMemberID}).
		Updates(map[string]any{"org_id": walletOrgID, "department_id": walletDepartmentID}).Error)
	seedToken(t, walletMemberKeyID, walletMemberID, walletMemberKey, 5000)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", walletMemberKeyID).Update("org_id", walletOrgID).Error)
	seedToken(t, walletBystanderKey, walletBystanderID, walletPersonalKey, 5000)
}

// walletCtx returns a request context with nothing on it yet.
func walletCtx() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

// walletRelayInfo is a request priced at one quota unit per token.
func walletRelayInfo(userID int, tokenID int, tokenKey string, spend relaycommon.OrgSpend) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestId:       "req-" + tokenKey + "-" + time.Now().Format("150405.000000000"),
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		OriginModelName: "gpt-4o-mini",
		StartTime:       time.Now(),
		OrgSpend:        spend,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
		PriceData: types.PriceData{
			ModelRatio: 1, CompletionRatio: 1,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}
}

// memberRequest is a request made with the member's organization key.
func memberRequest() *relaycommon.RelayInfo {
	return walletRelayInfo(walletMemberID, walletMemberKeyID, walletMemberKey, memberSpend)
}

// bystanderRequest is a request made with the bystander's personal key.
func bystanderRequest() *relaycommon.RelayInfo {
	return walletRelayInfo(walletBystanderID, walletBystanderKey, walletPersonalKey, relaycommon.OrgSpend{})
}

// balances reads the three balances these tests watch.
func balances(t *testing.T) (owner int, member int, bystander int) {
	t.Helper()
	return getUserQuota(t, walletOwnerID), getUserQuota(t, walletMemberID), getUserQuota(t, walletBystanderID)
}

// eventually waits for something a goroutine does after the call returned.
func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- whose balance pays ------------------------------------------------------

// The first acceptance item of the card: an organization key's spending comes
// out of the owner's balance, and the member's own balance is left alone.
func TestOrgWallet_AnOrganizationKeySpendsTheOwnersBalance(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	c, info := walletCtx(), memberRequest()

	require.Nil(t, PreConsumeBilling(c, 300, info))
	owner, member, bystander := balances(t)
	assert.Equal(t, []int{9700, 700, 4000}, []int{owner, member, bystander}, "the pre-consumed amount left the company wallet")
	assert.Equal(t, 4700, getTokenRemainQuota(t, walletMemberKeyID), "the key's own quota is counted as before")
	assert.Equal(t, 10000, info.UserQuota, "the balance the request was checked against is the wallet's")

	require.NoError(t, SettleBilling(c, info, 450))
	owner, member, bystander = balances(t)
	assert.Equal(t, []int{9550, 700, 4000}, []int{owner, member, bystander}, "the rest was settled on the company wallet")
	assert.Equal(t, 4550, getTokenRemainQuota(t, walletMemberKeyID))
	assert.Equal(t, 450, getTokenUsedQuota(t, walletMemberKeyID))
}

// The regression floor of the card: a personal key walks the same steps and
// pays from its own balance, exactly as before.
func TestOrgWallet_APersonalKeyStillSpendsItsOwnBalance(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	c, info := walletCtx(), bystanderRequest()

	require.Nil(t, PreConsumeBilling(c, 300, info))
	owner, member, bystander := balances(t)
	assert.Equal(t, []int{10000, 700, 3700}, []int{owner, member, bystander})
	assert.Equal(t, 4000, info.UserQuota)

	require.NoError(t, SettleBilling(c, info, 450))
	owner, member, bystander = balances(t)
	assert.Equal(t, []int{10000, 700, 3550}, []int{owner, member, bystander})
	assert.Equal(t, 4550, getTokenRemainQuota(t, walletBystanderKey))
}

// A request that fails after pre-consuming is refunded to whoever paid.
func TestOrgWallet_AFailedRequestIsRefundedToWhoeverPaid(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)

	c, info := walletCtx(), memberRequest()
	require.Nil(t, PreConsumeBilling(c, 300, info))
	info.Billing.Refund(c)
	eventually(t, "the company wallet to be refunded", func() bool { return getUserQuota(t, walletOwnerID) == 10000 })
	eventually(t, "the organization key to be refunded", func() bool { return getTokenRemainQuota(t, walletMemberKeyID) == 5000 })

	c, info = walletCtx(), bystanderRequest()
	require.Nil(t, PreConsumeBilling(c, 300, info))
	info.Billing.Refund(c)
	eventually(t, "the personal balance to be refunded", func() bool { return getUserQuota(t, walletBystanderID) == 4000 })

	owner, member, bystander := balances(t)
	assert.Equal(t, []int{10000, 700, 4000}, []int{owner, member, bystander}, "nobody gained or lost anything")
}

// "The member's own balance takes no part": a service account has none at
// all, and its key works as long as the company wallet can pay.
func TestOrgWallet_TheHoldersOwnBalanceIsNotAsked(t *testing.T) {
	seedWalletCast(t, 10000, 0, 4000)
	c, info := walletCtx(), memberRequest()

	require.Nil(t, PreConsumeBilling(c, 300, info))
	require.NoError(t, SettleBilling(c, info, 450))

	owner, member, _ := balances(t)
	assert.Equal(t, 9550, owner)
	assert.Equal(t, 0, member, "a holder with nothing is neither refused nor driven below zero")
}

// A rich wallet is trusted without pre-consuming, as a rich personal account
// is. The trust is the wallet's: the member here has nothing.
func TestOrgWallet_ATrustedRequestIsSettledOnTheCompanyWallet(t *testing.T) {
	rich := common.GetTrustQuota() + 1000
	seedWalletCast(t, rich, 0, 4000)
	c, info := walletCtx(), memberRequest()
	info.TokenUnlimited = true

	require.Nil(t, PreConsumeBilling(c, 300, info))
	assert.Equal(t, rich, getUserQuota(t, walletOwnerID), "nothing is taken up front from a wallet this full")
	assert.Equal(t, 0, info.FinalPreConsumedQuota)

	require.NoError(t, SettleBilling(c, info, 450))
	owner, member, _ := balances(t)
	assert.Equal(t, rich-450, owner)
	assert.Equal(t, 0, member)
}

// The sessionless settlement — realtime messages, the violation fee, the
// fallback of SettleBilling — takes from and returns to the same wallet.
func TestOrgWallet_SettlingWithoutASessionUsesTheSameWallet(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)

	require.NoError(t, PostConsumeQuota(memberRequest(), 200, 0, false))
	require.NoError(t, PostConsumeQuota(memberRequest(), -80, 0, false))
	require.NoError(t, PostConsumeQuota(bystanderRequest(), 200, 0, false))
	require.NoError(t, PostConsumeQuota(bystanderRequest(), -80, 0, false))

	owner, member, bystander := balances(t)
	assert.Equal(t, []int{9880, 700, 3880}, []int{owner, member, bystander})
	assert.Equal(t, 4880, getTokenRemainQuota(t, walletMemberKeyID))
}

// A realtime session is charged message by message; each charge checks and
// takes from the company wallet, whatever the holder has.
func TestOrgWallet_ARealtimeMessageSpendsTheCompanyWallet(t *testing.T) {
	seedWalletCast(t, 10000000, 0, 10000000)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", walletMemberKeyID).Update("remain_quota", 1000000).Error)
	usage := &dto.RealtimeUsage{}
	usage.InputTokenDetails.TextTokens = 40
	usage.OutputTokenDetails.TextTokens = 20

	info := memberRequest()
	info.UsingGroup = "default"
	require.NoError(t, PreWssConsumeQuota(walletCtx(), info, usage))

	spent := getTokenUsedQuota(t, walletMemberKeyID)
	require.Positive(t, spent, "the message cost something")
	owner, member, bystander := balances(t)
	assert.Equal(t, []int{10000000 - spent, 0, 10000000}, []int{owner, member, bystander})
}

// --- an empty wallet ---------------------------------------------------------

// The third acceptance item: when the company wallet cannot pay, every key of
// the organization is refused — the key of a member who has plenty themself
// as much as any other — and the refusal does not say what the company has.
func TestOrgWallet_AnEmptyCompanyWalletRefusesTheOrganizationsKeys(t *testing.T) {
	require.NoError(t, i18n.Init())
	cases := []struct {
		name        string
		ownerQuota  int
		memberQuota int
	}{
		{"nothing left, the member has plenty", 0, 9000},
		{"overdrawn", -250, 9000},
		{"less than this request needs", 137, 9000},
		{"nothing left, and the holder is a service account", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedWalletCast(t, tc.ownerQuota, tc.memberQuota, 4000)
			c, info := walletCtx(), memberRequest()

			apiErr := PreConsumeBilling(c, 300, info)

			require.NotNil(t, apiErr, "the request is refused")
			assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode(), "under the code a client already knows")
			assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
			assert.Equal(t, "Your organization's balance has run out. Ask the owner of your organization to add credit.", apiErr.Error())
			assert.False(t, strings.ContainsAny(apiErr.Error(), "0123456789"), "the company's balance is not the member's to read: %s", apiErr.Error())
			assert.Nil(t, info.Billing)

			owner, member, bystander := balances(t)
			assert.Equal(t, []int{tc.ownerQuota, tc.memberQuota, 4000}, []int{owner, member, bystander}, "nothing was taken from anyone")
			assert.Equal(t, 5000, getTokenRemainQuota(t, walletMemberKeyID), "nor from the key")
		})
	}
}

// A personal account that runs out is told so in the platform's own words,
// balance included — it is their balance.
func TestOrgWallet_AnEmptyPersonalBalanceIsRefusedAsBefore(t *testing.T) {
	seedWalletCast(t, 10000, 700, 0)

	apiErr := PreConsumeBilling(walletCtx(), 300, bystanderRequest())
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.True(t, strings.HasPrefix(apiErr.Error(), "用户额度不足, 剩余额度: "), apiErr.Error())

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", walletBystanderID).Update("quota", 137).Error)
	apiErr = PreConsumeBilling(walletCtx(), 300, bystanderRequest())
	require.NotNil(t, apiErr)
	assert.True(t, strings.HasPrefix(apiErr.Error(), "预扣费额度失败, 用户剩余额度: "), apiErr.Error())
}

// orgWalletError changes one error of an organization key and nothing else.
func TestOrgWallet_OnlyTheBalanceErrorOfAnOrganizationKeyIsReworded(t *testing.T) {
	require.NoError(t, i18n.Init())
	c := walletCtx()
	short := types.NewErrorWithStatusCode(assert.AnError, types.ErrorCodeInsufficientUserQuota, http.StatusForbidden)
	broken := types.NewError(assert.AnError, types.ErrorCodeQueryDataError)
	keyQuota := types.NewErrorWithStatusCode(assert.AnError, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden)

	assert.Same(t, short, orgWalletError(c, bystanderRequest(), short), "a personal key's error is passed on as it is")
	assert.Same(t, broken, orgWalletError(c, memberRequest(), broken), "so is an organization key's database error")
	assert.Same(t, keyQuota, orgWalletError(c, memberRequest(), keyQuota), "and its own key running out, which is the holder's to know")
	reworded := orgWalletError(c, memberRequest(), short)
	assert.NotSame(t, short, reworded)
	assert.True(t, types.IsSkipRetryError(reworded), "an empty wallet is not retried on another channel")
}

// --- never a subscription ----------------------------------------------------

// An organization key is paid from the company wallet even when its holder,
// or the owner, has a subscription that would otherwise be spent first.
func TestOrgWallet_AnOrganizationKeyNeverSpendsASubscription(t *testing.T) {
	// Created once: the package's database outlives the test, and SQLite's
	// migrator cannot read back the decimal column of a table it made itself.
	if !model.DB.Migrator().HasTable(&model.SubscriptionPlan{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPlan{}, &model.SubscriptionPreConsumeRecord{}))
	}
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM subscription_plans")
		model.DB.Exec("DELETE FROM subscription_pre_consume_records")
	})
	seedWalletCast(t, 10000, 700, 4000)
	plan := &model.SubscriptionPlan{Id: 31, Title: "Monthly", TotalAmount: 100000, Enabled: true}
	require.NoError(t, model.DB.Create(plan).Error)
	subscribe := func(id int, userID int) {
		sub := &model.UserSubscription{
			Id: id, UserId: userID, PlanId: plan.Id, AmountTotal: 100000, Status: "active",
			StartTime: time.Now().Unix(), EndTime: time.Now().Add(30 * 24 * time.Hour).Unix(),
		}
		require.NoError(t, model.DB.Create(sub).Error)
	}
	subscribe(41, walletOwnerID)
	subscribe(42, walletMemberID)
	subscribe(43, walletBystanderID)

	// A personal account with a subscription spends it first, as it always did.
	c, info := walletCtx(), bystanderRequest()
	require.Nil(t, PreConsumeBilling(c, 300, info))
	assert.Equal(t, BillingSourceSubscription, info.BillingSource)
	assert.EqualValues(t, 300, getSubscriptionUsed(t, 43))
	assert.Equal(t, 4000, getUserQuota(t, walletBystanderID))

	// The organization key does not, whoever of the two has one.
	c, info = walletCtx(), memberRequest()
	require.Nil(t, PreConsumeBilling(c, 300, info))
	assert.Equal(t, BillingSourceWallet, info.BillingSource)
	assert.EqualValues(t, 0, getSubscriptionUsed(t, 41), "the owner's subscription is untouched")
	assert.EqualValues(t, 0, getSubscriptionUsed(t, 42), "and so is the member's")
	owner, member, _ := balances(t)
	assert.Equal(t, []int{9700, 700}, []int{owner, member})
}

// --- the usage log -----------------------------------------------------------

// attachMemberSpend puts on a request what TokenAuth resolves for the member's
// organization key.
func attachMemberSpend(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserId, walletMemberID)
	common.SetContextKey(c, constant.ContextKeyTokenId, walletMemberKeyID)
	common.SetContextKey(c, constant.ContextKeyTokenKey, walletMemberKey)
	common.SetContextKey(c, constant.ContextKeyOrgId, memberSpend.OrgId)
	common.SetContextKey(c, constant.ContextKeyOrgDepartmentId, memberSpend.OrgDepartmentId)
	common.SetContextKey(c, constant.ContextKeyOrgWalletUserId, memberSpend.OrgWalletUserId)
}

// usedBy reads what a user is recorded as having spent and how often.
func usedBy(t *testing.T, userID int) (usedQuota int, requests int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("used_quota", "request_count").Where("id = ?", userID).First(&user).Error)
	return user.UsedQuota, user.RequestCount
}

// The request carries, from TokenAuth to the billing, whose balance pays: a
// RelayInfo built from the request has it, and one built from a request that
// had none answers with the user themself.
func TestOrgWallet_TheRequestCarriesWhoPays(t *testing.T) {
	c := walletCtx()
	attachMemberSpend(c)
	info := relaycommon.GenRelayInfoOpenAI(c, nil)
	assert.Equal(t, memberSpend, info.OrgSpend)
	assert.Equal(t, walletMemberID, info.UserId, "the request is still the member's")
	assert.Equal(t, walletOwnerID, info.WalletUserId())

	c = walletCtx()
	common.SetContextKey(c, constant.ContextKeyUserId, walletBystanderID)
	info = relaycommon.GenRelayInfoOpenAI(c, nil)
	assert.Equal(t, relaycommon.OrgSpend{}, info.OrgSpend)
	assert.Equal(t, walletBystanderID, info.WalletUserId())
}

// The second acceptance item, through the function every text request is
// settled by: the log line names the member and the key, and is stamped with
// the organization and the member's department. The amount is on the member's
// record and came out of the owner's balance.
func TestOrgWallet_TheUsageLogNamesTheMemberAndIsStamped(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	c := walletCtx()
	attachMemberSpend(c)
	c.Set("token_name", "Sales tools")
	info := relaycommon.GenRelayInfoOpenAI(c, nil)
	info.OriginModelName = "gpt-4o-mini"
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}
	info.PriceData = types.PriceData{ModelRatio: 1, CompletionRatio: 1, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}

	require.Nil(t, PreConsumeBilling(c, 100, info))
	PostTextConsumeQuota(c, info, testUsage(100, 50), nil)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, walletMemberID, log.UserId, "the usage is the member's")
	assert.Equal(t, walletMemberKeyID, log.TokenId)
	assert.Equal(t, "Sales tools", log.TokenName)
	assert.Equal(t, 150, log.Quota)
	assert.Equal(t, walletOrgID, log.OrgId)
	assert.Equal(t, walletDepartmentID, log.DepartmentId)

	owner, member, _ := balances(t)
	assert.Equal(t, []int{9850, 700}, []int{owner, member}, "paid by the company wallet")
	memberUsed, memberRequests := usedBy(t, walletMemberID)
	assert.Equal(t, []int{150, 1}, []int{memberUsed, memberRequests}, "counted on the member's record")
	ownerUsed, ownerRequests := usedBy(t, walletOwnerID)
	assert.Equal(t, []int{0, 0}, []int{ownerUsed, ownerRequests}, "and not on the owner's")
}

// A personal key's log line carries no stamp, as before the columns existed.
func TestOrgWallet_APersonalKeysUsageLogIsNotStamped(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	c := walletCtx()
	common.SetContextKey(c, constant.ContextKeyUserId, walletBystanderID)
	common.SetContextKey(c, constant.ContextKeyTokenId, walletBystanderKey)
	common.SetContextKey(c, constant.ContextKeyTokenKey, walletPersonalKey)
	info := relaycommon.GenRelayInfoOpenAI(c, nil)
	info.OriginModelName = "gpt-4o-mini"
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}
	info.PriceData = types.PriceData{ModelRatio: 1, CompletionRatio: 1, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}

	require.Nil(t, PreConsumeBilling(c, 100, info))
	PostTextConsumeQuota(c, info, testUsage(100, 50), nil)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, walletBystanderID, log.UserId)
	assert.Equal(t, 150, log.Quota)
	assert.Zero(t, log.OrgId)
	assert.Zero(t, log.DepartmentId)
	assert.Equal(t, 3850, getUserQuota(t, walletBystanderID))
}

// --- asynchronous tasks ------------------------------------------------------

// A task submitted with an organization key remembers who paid, so a refund
// decided minutes later goes back to the company wallet — not to the member,
// who never paid for it — and its log line carries the same stamp.
func TestOrgWallet_ATaskIsRefundedToTheCompanyWallet(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	task := makeTask(walletMemberID, 1, 3000, walletMemberKeyID, BillingSourceWallet, 0)
	task.PrivateData.OrgSpend = memberSpend

	RefundTaskQuota(context.Background(), task, "upstream failed")

	owner, member, bystander := balances(t)
	assert.Equal(t, []int{13000, 700, 4000}, []int{owner, member, bystander})
	assert.Equal(t, 8000, getTokenRemainQuota(t, walletMemberKeyID), "the key gets its quota back as before")
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, walletMemberID, log.UserId)
	assert.Equal(t, walletOrgID, log.OrgId)
	assert.Equal(t, walletDepartmentID, log.DepartmentId)
}

// The final settlement of a task works on the same wallet in both directions,
// and what the member used is counted on the member.
func TestOrgWallet_ATaskIsSettledOnTheCompanyWallet(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	seedChannel(t, 1)
	task := makeTask(walletMemberID, 1, 3000, walletMemberKeyID, BillingSourceWallet, 0)
	task.PrivateData.OrgSpend = memberSpend

	RecalculateTaskQuota(context.Background(), task, 3400, "adaptor adjustment")

	owner, member, _ := balances(t)
	assert.Equal(t, []int{9600, 700}, []int{owner, member}, "the 400 more came out of the company wallet")
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, 400, log.Quota)
	assert.Equal(t, walletMemberID, log.UserId)
	assert.Equal(t, walletOrgID, log.OrgId)
	assert.Equal(t, walletDepartmentID, log.DepartmentId)
	memberUsed, _ := usedBy(t, walletMemberID)
	assert.Equal(t, 400, memberUsed)

	RecalculateTaskQuota(context.Background(), task, 3100, "adaptor adjustment")

	owner, member, _ = balances(t)
	assert.Equal(t, []int{9900, 700}, []int{owner, member}, "and the 300 less went back into it")
	log = getLastLog(t)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, walletOrgID, log.OrgId)
}

// A task made with a personal key is refunded to its user, and its log line
// carries no stamp.
func TestOrgWallet_APersonalTaskIsRefundedToItsUser(t *testing.T) {
	seedWalletCast(t, 10000, 700, 4000)
	task := makeTask(walletBystanderID, 1, 3000, walletBystanderKey, BillingSourceWallet, 0)

	RefundTaskQuota(context.Background(), task, "upstream failed")

	owner, member, bystander := balances(t)
	assert.Equal(t, []int{10000, 700, 7000}, []int{owner, member, bystander})
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Zero(t, log.OrgId)
	assert.Zero(t, log.DepartmentId)
}

// What a task remembers survives the database, and a personal task's stored
// data gains nothing: the three fields are left out when they are empty.
func TestOrgWallet_ATaskKeepsWhoPaidInItsPrivateData(t *testing.T) {
	truncate(t)
	orgTask := makeTask(walletMemberID, 1, 3000, walletMemberKeyID, BillingSourceWallet, 0)
	orgTask.TaskID = "task_org"
	orgTask.PrivateData.OrgSpend = memberSpend
	require.NoError(t, orgTask.Insert())
	personalTask := makeTask(walletBystanderID, 1, 3000, walletBystanderKey, BillingSourceWallet, 0)
	personalTask.TaskID = "task_personal"
	require.NoError(t, personalTask.Insert())

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_org").First(&stored).Error)
	assert.Equal(t, memberSpend, stored.PrivateData.OrgSpend)
	assert.Equal(t, walletOwnerID, taskWalletUserId(&stored))
	assert.Equal(t, walletMemberKeyID, stored.PrivateData.TokenId, "next to what it already kept")

	var raw string
	require.NoError(t, model.DB.Raw("SELECT private_data FROM tasks WHERE task_id = ?", "task_personal").Scan(&raw).Error)
	assert.NotContains(t, raw, "org_")
	var personal model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_personal").First(&personal).Error)
	assert.Equal(t, walletBystanderID, taskWalletUserId(&personal))
}

// --- the low-balance reminder ------------------------------------------------

// sentNotice is one reminder the code under test tried to send.
type sentNotice struct {
	userID int
	email  string
	notice dto.Notify
}

// captureWalletNotices replaces the sender for the length of a test and
// returns what was handed to it.
func captureWalletNotices(t *testing.T) func() []sentNotice {
	t.Helper()
	var mu sync.Mutex
	var sent []sentNotice
	real := sendWalletNotice
	sendWalletNotice = func(userID int, email string, _ dto.UserSetting, notice dto.Notify) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, sentNotice{userID: userID, email: email, notice: notice})
		return nil
	}
	t.Cleanup(func() { sendWalletNotice = real })
	return func() []sentNotice {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentNotice{}, sent...)
	}
}

// approached reports whether the platform's own low-balance reminder — the one
// a personal account gets about its own balance — was started for a user. It
// leaves a trace in the notification limiter before anything is sent.
func approached(userID int) bool {
	prefix := strconv.Itoa(userID) + ":"
	found := false
	notifyLimitStore.Range(func(key, _ any) bool {
		found = strings.HasPrefix(key.(string), prefix)
		return !found
	})
	return found
}

// forgetApproaches empties the notification limiter, so each test reads only
// what it caused.
func forgetApproaches() {
	notifyLimitStore.Range(func(key, _ any) bool {
		notifyLimitStore.Delete(key)
		return true
	})
}

// seedWalletWatchers turns the cast into a real organization: the owner, an
// admin who reads Chinese and is told through Bark, and the member, who is
// staff. The owner asks to be warned below 5000.
func seedWalletWatchers(t *testing.T) (adminID int) {
	t.Helper()
	require.NoError(t, i18n.Init())
	require.NoError(t, orgmodel.Migrate(model.DB))
	t.Cleanup(func() { model.DB.Exec("DELETE FROM organizations") })
	forgetApproaches()
	require.NoError(t, model.DB.Create(&orgmodel.Organization{Id: walletOrgID, Name: "Acme", OwnerUserId: walletOwnerID}).Error)
	roleID := func(name string) int {
		id, err := orgmodel.PresetRoleID(model.DB, name)
		require.NoError(t, err)
		return id
	}
	adminID = 104
	seedAccount(t, adminID, "admin", 0)
	set := func(userID int, role string, email string, setting string) {
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"org_id": walletOrgID, "role_id": roleID(role), "department_id": walletDepartmentID,
			"email": email, "setting": setting,
		}).Error)
	}
	set(walletOwnerID, orgmodel.RoleOwner, "owner@acme.test", `{"quota_warning_threshold":5000}`)
	set(adminID, orgmodel.RoleAdmin, "admin@acme.test", `{"language":"zh","notify_type":"bark"}`)
	set(walletMemberID, orgmodel.RoleStaff, "member@acme.test", `{}`)
	return adminID
}

// The reminder about a company wallet running low goes to the owner and the
// admins, each in their own language and for their own channel — and never to
// the member whose request crossed the line.
func TestOrgWallet_ALowWalletIsReportedToTheOwnerAndTheAdmins(t *testing.T) {
	seedWalletCast(t, 4200, 3, 4000)
	adminID := seedWalletWatchers(t)
	sent := captureWalletNotices(t)

	checkAndSendQuotaNotify(memberRequest(), 150, 0)

	eventually(t, "both watchers to be told", func() bool { return len(sent()) == 2 })
	time.Sleep(50 * time.Millisecond)
	notices := sent()
	require.Len(t, notices, 2, "nobody else is told")
	byUser := map[int]sentNotice{}
	for _, notice := range notices {
		byUser[notice.userID] = notice
	}
	require.Contains(t, byUser, walletOwnerID)
	require.Contains(t, byUser, adminID)
	require.NotContains(t, byUser, walletMemberID)
	assert.False(t, approached(walletMemberID), "nor is the member sent the reminder a personal account gets")

	owner := byUser[walletOwnerID]
	assert.Equal(t, "owner@acme.test", owner.email)
	assert.Equal(t, dto.NotifyTypeQuotaExceed, owner.notice.Type)
	assert.Equal(t, "Your organization's balance is running low", owner.notice.Title)
	assert.Contains(t, owner.notice.Content, "<a href=", "mail gets a link it can show as one")
	assert.Contains(t, owner.notice.Content, "/console/topup")
	// The balance quoted is the wallet's as it stands — not the member's, and
	// not whatever the request that crossed the line happened to carry.
	assert.Contains(t, owner.notice.Content, logger.FormatQuota(4200))
	assert.Empty(t, owner.notice.Values, "the text is complete: nothing is left to fill in")

	admin := byUser[adminID]
	assert.Equal(t, "企业钱包余额即将用尽", admin.notice.Title, "in the language the admin saved")
	assert.NotContains(t, admin.notice.Content, "<", "Bark shows plain text")
	assert.Contains(t, admin.notice.Content, "/console/topup")
	assert.Contains(t, admin.notice.Content, logger.FormatQuota(4200))
}

// "Low" is what the wallet's holder asked to be warned at; above it nobody
// hears anything, whatever the member's own balance is.
func TestOrgWallet_AWalletAboveTheOwnersThresholdIsNotReported(t *testing.T) {
	seedWalletCast(t, 5000, 3, 4000)
	seedWalletWatchers(t)
	sent := captureWalletNotices(t)

	checkAndSendQuotaNotify(memberRequest(), 150, 0)

	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, sent())
	// The member's own balance is 3, far below any threshold: were their
	// request handled as a personal account's, they would be written to.
	assert.False(t, approached(walletMemberID))
}

// A personal account that runs low is reminded as it always was: about its
// own balance, through the platform's own reminder, and nobody else hears.
func TestOrgWallet_APersonalAccountIsRemindedAsBefore(t *testing.T) {
	seedWalletCast(t, 4200, 3, 500)
	seedWalletWatchers(t)
	sent := captureWalletNotices(t)
	info := bystanderRequest()
	info.UserQuota = 500

	checkAndSendQuotaNotify(info, 150, 0)

	eventually(t, "the personal account to be reminded", func() bool { return approached(walletBystanderID) })
	assert.Empty(t, sent(), "the organization's reminder has nothing to do with it")
	assert.False(t, approached(walletOwnerID))
}

// Without a threshold of the owner's own the platform's default decides.
func TestOrgWallet_TheDefaultThresholdAppliesWhenTheOwnerSetNone(t *testing.T) {
	seedWalletCast(t, common.QuotaRemindThreshold-1, 3, 4000)
	seedWalletWatchers(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", walletOwnerID).Update("setting", "").Error)
	sent := captureWalletNotices(t)

	checkAndSendQuotaNotify(memberRequest(), 1, 0)
	eventually(t, "the watchers to be told", func() bool { return len(sent()) == 2 })

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", walletOwnerID).Update("quota", common.QuotaRemindThreshold).Error)
	checkAndSendQuotaNotify(memberRequest(), 1, 0)
	time.Sleep(150 * time.Millisecond)
	assert.Len(t, sent(), 2, "at the threshold itself nothing more is sent")
}
