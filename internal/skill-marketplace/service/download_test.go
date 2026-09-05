package service_test

import (
	"sync"
	"testing"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testQuotaPerUnit = 500_000.0

func newDownloadSvc(db *gorm.DB) *mktsvc.DownloadService {
	return mktsvc.NewDownloadService(db, func() float64 { return testQuotaPerUnit })
}

// setupDownloadDB narrows the shared in-memory DB to a single connection.
// glebarez/sqlite gives every pooled connection its own ":memory:" database,
// so a second connection (as the concurrency test would open) sees no tables
// at all; one connection also serializes concurrent transactions the way a
// real PG would serialize them on the users row lock.
func setupDownloadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

// seedPaidSkill creates a published paid skill with an active packaged
// version and returns (skillID, versionID).
func seedPaidSkill(t *testing.T, db *gorm.DB, slug string, priceUSD float64) (int64, int64) {
	t.Helper()
	skillID := insertSkillRow(t, db, slug, "published", "code", false, 0, "2026-01-01 00:00:00")
	require.NoError(t, db.Exec(
		`UPDATE skills SET monetization_type = 'paid', price_usd = ? WHERE id = ?`, priceUSD, skillID).Error)
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")
	require.NoError(t, db.Exec(
		`UPDATE skill_versions SET package_zip = ? WHERE id = ?`, []byte("PK-fake-zip"), versionID).Error)
	require.NoError(t, db.Exec(
		`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID).Error)
	return skillID, versionID
}

func seedFreeSkill(t *testing.T, db *gorm.DB, slug string) (int64, int64) {
	t.Helper()
	skillID := insertSkillRow(t, db, slug, "published", "code", false, 0, "2026-01-01 00:00:00")
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")
	require.NoError(t, db.Exec(
		`UPDATE skill_versions SET package_zip = ? WHERE id = ?`, []byte("PK-fake-zip"), versionID).Error)
	require.NoError(t, db.Exec(
		`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID).Error)
	return skillID, versionID
}

func seedUser(t *testing.T, db *gorm.DB, id, quota int64) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO users (id, quota) VALUES (?, ?)`, id, quota).Error)
}

func userQuota(t *testing.T, db *gorm.DB, id int64) int64 {
	t.Helper()
	var q int64
	require.NoError(t, db.Raw(`SELECT quota FROM users WHERE id = ?`, id).Scan(&q).Error)
	return q
}

func purchaseCount(t *testing.T, db *gorm.DB, userID, skillID int64) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM skill_purchases WHERE user_id = ? AND skill_id = ?`, userID, skillID).Scan(&n).Error)
	return n
}

// ── Free skills ───────────────────────────────────────────────────────────────

func TestDownload_FreeSkill_NoPurchaseRecord(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)
	skillID, versionID := seedFreeSkill(t, db, "free-skill")

	result, err := svc.Download(42, "free-skill")
	require.NoError(t, err)
	assert.Equal(t, []byte("PK-fake-zip"), result.Zip)
	assert.Equal(t, "free-skill.zip", result.Filename)
	assert.False(t, result.PurchaseMade)
	assert.Equal(t, int64(0), purchaseCount(t, db, 42, skillID))

	var ues struct {
		VersionID int64
	}
	require.NoError(t, db.Raw(
		`SELECT version_id FROM user_enabled_skills WHERE user_id = 42 AND skill_id = ?`, skillID).Scan(&ues).Error)
	assert.Equal(t, versionID, ues.VersionID)
}

// ── Paid skills ───────────────────────────────────────────────────────────────

func TestDownload_PaidFirstPurchase_DeductsAndRecords(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 2_000_000)
	skillID, _ := seedPaidSkill(t, db, "paid-skill", 2.00)

	result, err := svc.Download(42, "paid-skill")
	require.NoError(t, err)
	assert.True(t, result.PurchaseMade)
	assert.Equal(t, int64(1_000_000), result.QuotaDeducted)
	assert.Equal(t, int64(1_000_000), userQuota(t, db, 42))
	assert.Equal(t, int64(1), purchaseCount(t, db, 42, skillID))
}

func TestDownload_PaidAlreadyOwned_NoSecondCharge(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 2_000_000)
	skillID, _ := seedPaidSkill(t, db, "paid-skill", 2.00)

	_, err := svc.Download(42, "paid-skill")
	require.NoError(t, err)

	result, err := svc.Download(42, "paid-skill")
	require.NoError(t, err)
	assert.False(t, result.PurchaseMade)
	assert.Equal(t, int64(1_000_000), userQuota(t, db, 42))
	assert.Equal(t, int64(1), purchaseCount(t, db, 42, skillID))
}

func TestDownload_PaidInsufficientBalance_NothingChanges(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 999_999)
	skillID, _ := seedPaidSkill(t, db, "paid-skill", 2.00)

	_, err := svc.Download(42, "paid-skill")
	assert.ErrorIs(t, err, mktsvc.ErrInsufficientQuota)
	var iqe *mktsvc.InsufficientQuotaError
	require.ErrorAs(t, err, &iqe, "402 carries the price details as data")
	assert.Equal(t, 2.00, iqe.PriceUSD)
	assert.Equal(t, int64(1_000_000), iqe.QuotaNeeded)
	assert.Equal(t, int64(999_999), userQuota(t, db, 42))
	assert.Equal(t, int64(0), purchaseCount(t, db, 42, skillID))

	var ues int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM user_enabled_skills WHERE user_id = 42`).Scan(&ues).Error)
	assert.Equal(t, int64(0), ues)
}

// TestDownload_ConcurrentDoubleBuy is the race the UNIQUE(user_id, skill_id)
// constraint exists for: two requests both pass the "not yet owned" check,
// both enter the purchase transaction, and only one may deduct.
func TestDownload_ConcurrentDoubleBuy_ChargesOnce(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 5_000_000)
	skillID, _ := seedPaidSkill(t, db, "paid-skill", 2.00)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Download(42, "paid-skill")
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, int64(4_000_000), userQuota(t, db, 42), "exactly one deduction")
	assert.Equal(t, int64(1), purchaseCount(t, db, 42, skillID))
}

// ── Availability ──────────────────────────────────────────────────────────────

func TestDownload_UnpublishedOrUnknown_NotAvailable(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)
	insertSkillRow(t, db, "still-draft", "draft", "code", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "gone", "deprecated", "code", false, 0, "2026-01-01 00:00:00")

	_, err := svc.Download(42, "still-draft")
	assert.ErrorIs(t, err, mktsvc.ErrSkillNotAvailable)
	_, err = svc.Download(42, "gone")
	assert.ErrorIs(t, err, mktsvc.ErrSkillNotAvailable)
	_, err = svc.Download(42, "never-existed")
	assert.ErrorIs(t, err, mktsvc.ErrSkillNotAvailable)
}

func TestDownload_MissingPackage_FailsBeforeCharging(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 2_000_000)
	skillID, versionID := seedPaidSkill(t, db, "broken", 2.00)
	require.NoError(t, db.Exec(`UPDATE skill_versions SET package_zip = NULL WHERE id = ?`, versionID).Error)

	_, err := svc.Download(42, "broken")
	assert.ErrorIs(t, err, mktsvc.ErrPackageMissing)
	assert.Equal(t, int64(2_000_000), userQuota(t, db, 42), "no charge for an undeliverable package")
	assert.Equal(t, int64(0), purchaseCount(t, db, 42, skillID))
}

func TestDownload_NoActiveVersion_PackageMissing(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)
	insertSkillRow(t, db, "no-version", "published", "code", false, 0, "2026-01-01 00:00:00")

	_, err := svc.Download(42, "no-version")
	assert.ErrorIs(t, err, mktsvc.ErrPackageMissing)
}

// ── Re-download / upsert ──────────────────────────────────────────────────────

func TestDownload_Redownload_UpdatesVersionID(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)
	skillID, v1 := seedFreeSkill(t, db, "evolving")

	_, err := svc.Download(42, "evolving")
	require.NoError(t, err)

	// A new version gets activated (as P2's ActivateVersion would do).
	v2 := insertVersion(t, db, skillID, "2.0.0", "active")
	require.NoError(t, db.Exec(`UPDATE skill_versions SET package_zip = ?, status = 'archived' WHERE id = ?`,
		[]byte("old"), v1).Error)
	require.NoError(t, db.Exec(`UPDATE skill_versions SET package_zip = ? WHERE id = ?`,
		[]byte("PK-v2"), v2).Error)
	require.NoError(t, db.Exec(`UPDATE skills SET active_version_id = ? WHERE id = ?`, v2, skillID).Error)

	result, err := svc.Download(42, "evolving")
	require.NoError(t, err)
	assert.Equal(t, []byte("PK-v2"), result.Zip)

	var rows []struct {
		VersionID int64
	}
	require.NoError(t, db.Raw(
		`SELECT version_id FROM user_enabled_skills WHERE user_id = 42 AND skill_id = ?`, skillID).Scan(&rows).Error)
	require.Len(t, rows, 1, "upsert must not create a second row")
	assert.Equal(t, v2, rows[0].VersionID)
}
