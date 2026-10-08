package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P8 (meta-repo docs/enterprise-org-prd.md §4): the two scans
// behind the card's acceptance items — warnings when a key reaches a share of
// what it was given, and the three anomaly rules. Every moment in these tests
// is told from scanNow, so that nothing depends on the day they run.

// scanNow is "now" for the scans: a Thursday noon in UTC.
var scanNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// dollar is one dollar in quota units: the default floor of the anomaly rules.
var dollar = int(common.QuotaPerUnit)

// noMonthlyUsage is the request counter of a gateway nobody has used.
func noMonthlyUsage(int) (int, error) { return 0, nil }

// monthlyUsageOf is a request counter that answers from a table.
func monthlyUsageOf(counts map[int]int) MonthlyUsage {
	return func(keyID int) (int, error) { return counts[keyID], nil }
}

// withUsageLog adds the usage log to a test database, which the anomaly scan
// reads and the other tests of this package have no use for.
func withUsageLog(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&platformmodel.Log{}))
}

// alertsOf returns an organization's alerts, oldest first.
func alertsOf(t *testing.T, db *gorm.DB, orgID int) []orgmodel.OrgAlert {
	t.Helper()
	var alerts []orgmodel.OrgAlert
	require.NoError(t, db.Where("org_id = ?", orgID).Order("id").Find(&alerts).Error)
	return alerts
}

// rulesOf lists what a set of alerts is about, as "rule" or "rule@level".
func rulesOf(alerts []orgmodel.OrgAlert) []string {
	rules := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		if alert.Level != 0 {
			rules = append(rules, fmt.Sprintf("%s@%d", alert.Rule, alert.Level))
			continue
		}
		rules = append(rules, alert.Rule)
	}
	return rules
}

// detailOf reads back the numbers an alert was raised with.
func detailOf(t *testing.T, alert orgmodel.OrgAlert) orgmodel.AlertDetail {
	t.Helper()
	var detail orgmodel.AlertDetail
	require.NoError(t, common.UnmarshalJsonStr(alert.Detail, &detail))
	return detail
}

// setAllowance writes what a key has used and has left, the way a settled
// request does.
func setAllowance(t *testing.T, db *gorm.DB, keyID int, used int, remain int) {
	t.Helper()
	require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", keyID).
		Updates(map[string]any{"used_quota": used, "remain_quota": remain}).Error)
}

// scanAllowances runs the warning scan over every key and returns what the
// organization's alert list holds afterwards.
func scanAllowances(t *testing.T, db *gorm.DB, orgID int, monthly MonthlyUsage) []orgmodel.OrgAlert {
	t.Helper()
	_, err := ScanAllowances(db, 0, scanNow, monthly)
	require.NoError(t, err)
	return alertsOf(t, db, orgID)
}

// logUsage writes one usage log line of an organization key: who used it,
// when, for how much and from where.
func logUsage(t *testing.T, db *gorm.DB, key platformmodel.Token, userID int, at time.Time, quota int, ip string) {
	t.Helper()
	require.NoError(t, db.Create(&platformmodel.Log{
		UserId: userID, TokenId: key.Id, OrgId: key.OrgId, Type: platformmodel.LogTypeConsume,
		CreatedAt: at.Unix(), Quota: quota, Ip: ip, ModelName: "gpt-4o-mini", TokenName: key.Name,
	}).Error)
}

// logUsualWeek gives a key the same spending on each of the seven days before
// the last one: what the spike and off-hours rules compare a day against.
func logUsualWeek(t *testing.T, db *gorm.DB, key platformmodel.Token, userID int, perDay int) {
	t.Helper()
	for day := 1; day <= alertBaselineDays; day++ {
		logUsage(t, db, key, userID, scanNow.Add(-time.Duration(day)*24*time.Hour-time.Hour), perDay, "")
	}
}

// scanAnomalies runs the anomaly scan at a moment and returns what the
// organization's alert list holds afterwards.
func scanAnomalies(t *testing.T, db *gorm.DB, orgID int, at time.Time) []orgmodel.OrgAlert {
	t.Helper()
	_, err := ScanAnomalies(db, db, at)
	require.NoError(t, err)
	return alertsOf(t, db, orgID)
}

// keyInSales founds an organization and hands a member of its Sales
// department a key that has long been theirs.
func keyInSales(t *testing.T, db *gorm.DB, name string) (testOrg, platformmodel.User, platformmodel.Token) {
	t.Helper()
	org := seedOrg(t, db, name)
	sales := departmentNamed(t, db, org.id, "Sales")
	staff := seedMemberIn(t, db, org, "staff-of-"+name, presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)
	return org, staff, seedKey(t, db, org, staff.Id, "Design tools")
}

// --- warnings: a share of what the key was given -----------------------------

// Acceptance: key 的额度…使用率达到阈值（默认 80% 与 100% 两档）. One warning per
// level, however often the key is looked at.
func TestScanAllowances_WarnsOnceAtEachLevelOfTheQuota(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, key := keyInSales(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		scan := func() []orgmodel.OrgAlert { return scanAllowances(t, db, org.id, noMonthlyUsage) }

		require.Empty(t, scan(), "a quarter used: nothing to say")
		setAllowance(t, db, key.Id, 799, 201)
		require.Empty(t, scan(), "one short of 80%")

		setAllowance(t, db, key.Id, 800, 200)
		alerts := scan()
		require.Equal(t, []string{"quota@80"}, rulesOf(alerts))
		warning := alerts[0]
		assert.Equal(t, org.id, warning.OrgId)
		assert.Equal(t, key.Id, warning.TokenId)
		assert.Equal(t, staff.Id, warning.UserId, "the key's holder")
		assert.Equal(t, sales.Id, warning.DepartmentId, "the department the holder is in")
		assert.Equal(t, "q1000", warning.Cycle, "what the key was given in all")
		assert.Equal(t, scanNow.Unix(), warning.CreatedTime)
		assert.Zero(t, warning.NotifiedTime, "waiting to be sent")
		assert.Equal(t, orgmodel.AlertStateOpen, warning.State)
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Used: 800, Limit: 1000}, detailOf(t, warning))

		require.Len(t, scan(), 1, "looking again says nothing new")
		setAllowance(t, db, key.Id, 950, 50)
		require.Len(t, scan(), 1, "nor does spending more below the next level")

		setAllowance(t, db, key.Id, 1000, 0)
		alerts = scan()
		require.Equal(t, []string{"quota@80", "quota@100"}, rulesOf(alerts))
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Used: 1000, Limit: 1000}, detailOf(t, alerts[1]))

		// A last request can take a key below zero; that is still "used up".
		setAllowance(t, db, key.Id, 1100, -100)
		require.Len(t, scan(), 2)
	})
}

// A key that goes from little to nothing left between two looks is told once.
func TestScanAllowances_AKeyThatJumpsToTheLimitIsToldOnce(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, _, key := keyInSales(t, db, "Acme")
		setAllowance(t, db, key.Id, 1000, 0)
		require.Equal(t, []string{"quota@100"}, rulesOf(scanAllowances(t, db, org.id, noMonthlyUsage)))
		// A refund takes it back under the limit: it has heard about 80% in
		// effect, and is not told again on the way back up.
		setAllowance(t, db, key.Id, 850, 150)
		require.Len(t, scanAllowances(t, db, org.id, noMonthlyUsage), 1)
		setAllowance(t, db, key.Id, 1000, 0)
		require.Len(t, scanAllowances(t, db, org.id, noMonthlyUsage), 1)
	})
}

// "Once per cycle": giving the key more starts a new cycle, and the levels
// speak again.
func TestScanAllowances_GivingAKeyMoreStartsANewCycle(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, _, key := keyInSales(t, db, "Acme")
		scan := func() []orgmodel.OrgAlert { return scanAllowances(t, db, org.id, noMonthlyUsage) }
		setAllowance(t, db, key.Id, 1000, 0)
		require.Equal(t, []string{"quota@100"}, rulesOf(scan()))

		// An administrator gives it another 1000 through the keys page.
		_, err := UpdateKey(db, actorFor(t, db, org.owner.Id), key.Id, KeyPatch{RemainQuota: intPtr(1000)}, fixedModels(testCatalogue))
		require.NoError(t, err)
		require.Len(t, scan(), 1, "half of 2000 used: nothing to say")

		setAllowance(t, db, key.Id, 1600, 400)
		alerts := scan()
		require.Equal(t, []string{"quota@100", "quota@80"}, rulesOf(alerts))
		assert.Equal(t, "q2000", alerts[1].Cycle)
		setAllowance(t, db, key.Id, 2000, 0)
		require.Equal(t, []string{"quota@100", "quota@80", "quota@100"}, rulesOf(scan()))
	})
}

// Acceptance: …或月限额使用率达到阈值. The monthly limit counts requests, and its
// cycle is the calendar month.
func TestScanAllowances_WarnsAtEachLevelOfTheMonthlyLimit(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, _, key := keyInSales(t, db, "Acme")
		// The key has no quota to run out of, however much it has used.
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).
			Updates(map[string]any{"monthly_limit": 10, "unlimited_quota": true, "used_quota": 5000, "remain_quota": 0}).Error)
		requests := map[int]int{}
		scan := func(at time.Time) []orgmodel.OrgAlert {
			_, err := ScanAllowances(db, 0, at, monthlyUsageOf(requests))
			require.NoError(t, err)
			return alertsOf(t, db, org.id)
		}

		requests[key.Id] = 7
		require.Empty(t, scan(scanNow))
		requests[key.Id] = 8
		alerts := scan(scanNow)
		require.Equal(t, []string{"monthly@80"}, rulesOf(alerts))
		assert.Equal(t, "2026-10", alerts[0].Cycle)
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Used: 8, Limit: 10}, detailOf(t, alerts[0]))
		require.Len(t, scan(scanNow), 1)
		requests[key.Id] = 10
		require.Equal(t, []string{"monthly@80", "monthly@100"}, rulesOf(scan(scanNow)))

		// A new month: the counter starts over, and so do the warnings.
		november := scanNow.AddDate(0, 1, 0)
		requests[key.Id] = 3
		require.Len(t, scan(november), 2)
		requests[key.Id] = 9
		alerts = scan(november)
		require.Equal(t, []string{"monthly@80", "monthly@100", "monthly@80"}, rulesOf(alerts))
		assert.Equal(t, "2026-11", alerts[2].Cycle)
	})
}

// A key with both limits is warned about each on its own.
func TestScanAllowances_QuotaAndMonthlyLimitAreSeparateWarnings(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, _, key := keyInSales(t, db, "Acme")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("monthly_limit", 100).Error)
		setAllowance(t, db, key.Id, 900, 100)
		alerts := scanAllowances(t, db, org.id, monthlyUsageOf(map[int]int{key.Id: 100}))
		require.Equal(t, []string{"quota@80", "monthly@100"}, rulesOf(alerts))
	})
}

// What has no limit has no share of it to reach, and what is not an
// organization's key is none of this scan's business.
func TestScanAllowances_LeavesAloneWhatHasNoLimitOrNoOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, unlimited := keyInSales(t, db, "Acme")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", unlimited.Id).
			Updates(map[string]any{"unlimited_quota": true, "used_quota": 5000, "remain_quota": 0}).Error)

		// A key that was never given anything is "used up" from the start.
		empty := seedKey(t, db, org, staff.Id, "Empty")
		setAllowance(t, db, empty.Id, 0, 0)

		// A deleted key that had run out.
		deleted := seedKey(t, db, org, staff.Id, "Deleted")
		setAllowance(t, db, deleted.Id, 1000, 0)
		require.NoError(t, db.Delete(&platformmodel.Token{}, deleted.Id).Error)

		// A personal account's key, used up and with a monthly limit reached.
		loner := seedUser(t, db, "loner", common.RoleCommonUser)
		personal := platformmodel.Token{
			UserId: loner.Id, Name: "personal", Key: "personal-key-value", Status: common.TokenStatusEnabled,
			AccessedTime: scanNow.Unix(), ExpiredTime: -1, UsedQuota: 1000, RemainQuota: 0, MonthlyLimit: 5,
		}
		require.NoError(t, db.Create(&personal).Error)

		raised, err := ScanAllowances(db, 0, scanNow, monthlyUsageOf(map[int]int{personal.Id: 5, unlimited.Id: 99}))
		require.NoError(t, err)
		assert.Zero(t, raised)
		var total int64
		require.NoError(t, db.Model(&orgmodel.OrgAlert{}).Count(&total).Error)
		assert.Zero(t, total)
	})
}

// "since" is what keeps a pass a minute from reading every key there is: only
// the keys used from then on are looked at.
func TestScanAllowances_LooksOnlyAtKeysUsedSince(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, idle := keyInSales(t, db, "Acme")
		busy := seedKey(t, db, org, staff.Id, "Busy")
		setAllowance(t, db, idle.Id, 1000, 0)
		setAllowance(t, db, busy.Id, 1000, 0)
		lastUsed := scanNow.Add(-10 * time.Second).Unix()
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", busy.Id).Update("accessed_time", lastUsed).Error)

		_, err := ScanAllowances(db, lastUsed+1, scanNow, noMonthlyUsage)
		require.NoError(t, err)
		require.Empty(t, alertsOf(t, db, org.id), "neither was used after that moment")

		_, err = ScanAllowances(db, lastUsed, scanNow, noMonthlyUsage)
		require.NoError(t, err)
		alerts := alertsOf(t, db, org.id)
		require.Len(t, alerts, 1)
		assert.Equal(t, busy.Id, alerts[0].TokenId)

		// Zero looks at every key: what the first pass after a start does.
		_, err = ScanAllowances(db, 0, scanNow, noMonthlyUsage)
		require.NoError(t, err)
		require.Len(t, alertsOf(t, db, org.id), 2)
	})
}

// Acceptance: …（…可配）. The levels are each organization's own.
func TestScanAllowances_UsesEachOrganizationsOwnLevels(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme, _, acmeKey := keyInSales(t, db, "Acme")
		globex, _, globexKey := keyInSales(t, db, "Globex")
		initech, _, initechKey := keyInSales(t, db, "Initech")
		early := orgmodel.DefaultAlertSettings()
		early.WarnAt = []int{50, 90}
		_, err := UpdateAlertSettings(db, actorFor(t, db, acme.owner.Id), early)
		require.NoError(t, err)
		never := orgmodel.DefaultAlertSettings()
		never.WarnAt = []int{}
		_, err = UpdateAlertSettings(db, actorFor(t, db, initech.owner.Id), never)
		require.NoError(t, err)

		for _, key := range []platformmodel.Token{acmeKey, globexKey} {
			setAllowance(t, db, key.Id, 600, 400)
		}
		setAllowance(t, db, initechKey.Id, 1000, 0)
		_, err = ScanAllowances(db, 0, scanNow, noMonthlyUsage)
		require.NoError(t, err)

		assert.Equal(t, []string{"quota@50"}, rulesOf(alertsOf(t, db, acme.id)), "Acme asked to hear at 50%")
		assert.Empty(t, alertsOf(t, db, globex.id), "Globex is on the defaults: 60% is short of 80%")
		assert.Empty(t, alertsOf(t, db, initech.id), "Initech switched its warnings off")
	})
}

// One key whose request counter cannot be read does not cost the others their
// warnings — and the failure is reported, not swallowed.
func TestScanAllowances_AnUnreadableCounterDoesNotStopTheOthers(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, first := keyInSales(t, db, "Acme")
		second := seedKey(t, db, org, staff.Id, "Second")
		for _, key := range []platformmodel.Token{first, second} {
			require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("monthly_limit", 10).Error)
		}
		setAllowance(t, db, first.Id, 900, 100)
		unreachable := errors.New("redis is away")
		raised, err := ScanAllowances(db, 0, scanNow, func(keyID int) (int, error) {
			if keyID == first.Id {
				return 0, unreachable
			}
			return 10, nil
		})
		require.ErrorIs(t, err, unreachable)
		assert.Equal(t, 2, raised)
		assert.Equal(t, []string{"quota@80", "monthly@100"}, rulesOf(alertsOf(t, db, org.id)))
	})
}

// --- a key refused before 100% -------------------------------------------------

// PRD D45: a key the gateway turned away because what it had left was less
// than the request could cost is warned about as if it had reached 100% — with
// the numbers of the refusal — once per cycle, like any other warning.
func TestWarnRefused_IsTheLastWarningOfTheCycle(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, key := keyInSales(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		setAllowance(t, db, key.Id, 930, 70)

		raised, err := WarnRefused(db, key.Id, 120, scanNow)
		require.NoError(t, err)
		assert.Equal(t, 1, raised)
		alerts := alertsOf(t, db, org.id)
		require.Equal(t, []string{"quota@100"}, rulesOf(alerts))
		warning := alerts[0]
		assert.Equal(t, key.Id, warning.TokenId)
		assert.Equal(t, staff.Id, warning.UserId, "the key's holder")
		assert.Equal(t, sales.Id, warning.DepartmentId)
		assert.Equal(t, "q1000", warning.Cycle, "the same cycle the scan would use")
		assert.Equal(t, scanNow.Unix(), warning.CreatedTime)
		assert.Zero(t, warning.NotifiedTime, "waiting to be sent")
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Used: 930, Limit: 1000, Needed: 120}, detailOf(t, warning))

		raised, err = WarnRefused(db, key.Id, 150, scanNow.Add(time.Minute))
		require.NoError(t, err)
		assert.Zero(t, raised, "refused again: told already")
		require.Len(t, scanAllowances(t, db, org.id, noMonthlyUsage), 1, "and the scan has nothing to add: 80% is below what was told")

		// Smaller requests still get through, and the key does run out.
		setAllowance(t, db, key.Id, 1000, 0)
		require.Len(t, scanAllowances(t, db, org.id, noMonthlyUsage), 1, "100% was told, as the refusal")

		// Given more, it starts over — and a refusal in the new cycle speaks again.
		_, err = UpdateKey(db, actorFor(t, db, org.owner.Id), key.Id, KeyPatch{RemainQuota: intPtr(1000)}, fixedModels(testCatalogue))
		require.NoError(t, err)
		setAllowance(t, db, key.Id, 1900, 100)
		raised, err = WarnRefused(db, key.Id, 300, scanNow.Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, raised)
		alerts = alertsOf(t, db, org.id)
		require.Equal(t, []string{"quota@100", "quota@100"}, rulesOf(alerts))
		assert.Equal(t, "q2000", alerts[1].Cycle)
	})
}

// A refusal after the 80% warning is the next level; one after the key was
// already told it is used up is nothing new.
func TestWarnRefused_FollowsWhatTheScanHasSaid(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, first := keyInSales(t, db, "Acme")
		second := seedKey(t, db, org, staff.Id, "Second")
		setAllowance(t, db, first.Id, 850, 150)
		setAllowance(t, db, second.Id, 1000, 0)
		require.Equal(t, []string{"quota@80", "quota@100"}, rulesOf(scanAllowances(t, db, org.id, noMonthlyUsage)))

		raised, err := WarnRefused(db, first.Id, 200, scanNow)
		require.NoError(t, err)
		assert.Equal(t, 1, raised, "80% was told; the refusal is the 100%")
		raised, err = WarnRefused(db, second.Id, 1, scanNow)
		require.NoError(t, err)
		assert.Zero(t, raised, "a key told it was used up is not told again when it is refused")
		assert.Equal(t, []string{"quota@80", "quota@100", "quota@100"}, rulesOf(alertsOf(t, db, org.id)))
	})
}

// What is not a refusal for lack of quota raises nothing: a key that could
// have paid, one with no limit, a personal key, and a key of an organization
// that has switched its warnings off.
func TestWarnRefused_LeavesAloneWhatWasNotRefusedForItsQuota(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, key := keyInSales(t, db, "Acme")
		setAllowance(t, db, key.Id, 500, 500)
		unlimited := seedKey(t, db, org, staff.Id, "Unlimited")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", unlimited.Id).
			Updates(map[string]any{"unlimited_quota": true, "used_quota": 5000, "remain_quota": 0}).Error)
		loner := seedUser(t, db, "loner", common.RoleCommonUser)
		personal := platformmodel.Token{
			UserId: loner.Id, Name: "personal", Key: "personal-key-value", Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UsedQuota: 990, RemainQuota: 10,
		}
		require.NoError(t, db.Create(&personal).Error)
		initech, _, silentKey := keyInSales(t, db, "Initech")
		never := orgmodel.DefaultAlertSettings()
		never.WarnAt = []int{}
		_, err := UpdateAlertSettings(db, actorFor(t, db, initech.owner.Id), never)
		require.NoError(t, err)
		setAllowance(t, db, silentKey.Id, 990, 10)

		for name, refusal := range map[string]struct {
			keyID  int
			needed int
		}{
			"it could have paid":      {key.Id, 300},
			"it has no limit":         {unlimited.Id, 300},
			"a personal key":          {personal.Id, 300},
			"warnings switched off":   {silentKey.Id, 300},
			"a key that is not there": {personal.Id + 1000, 300},
		} {
			raised, err := WarnRefused(db, refusal.keyID, refusal.needed, scanNow)
			require.NoError(t, err, name)
			assert.Zero(t, raised, name)
		}
		var total int64
		require.NoError(t, db.Model(&orgmodel.OrgAlert{}).Count(&total).Error)
		assert.Zero(t, total)
	})
}

// --- anomaly rule one: a spike ------------------------------------------------

// Acceptance: 日用量超 7 日均值 N 倍. "More than five times" means more than, the
// same key and rule speak once a day, and a day later they may speak again.
func TestScanAnomalies_ASpikeIsReportedAndThenQuietForADay(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		logUsualWeek(t, db, key, staff.Id, dollar)

		logUsage(t, db, key, staff.Id, scanNow.Add(-2*time.Hour), 5*dollar, "")
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "exactly five times the daily average is not more than five times")

		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 1, "")
		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(alerts))
		spike := alerts[0]
		assert.Equal(t, key.Id, spike.TokenId)
		assert.Equal(t, staff.Id, spike.UserId)
		assert.Equal(t, sales.Id, spike.DepartmentId)
		assert.Equal(t, scanNow.Unix(), spike.CreatedTime)
		assert.Zero(t, spike.NotifiedTime)
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Spent: 5*dollar + 1, DailyAverage: dollar}, detailOf(t, spike))

		require.Len(t, scanAnomalies(t, db, org.id, scanNow.Add(10*time.Minute)), 1, "the next pass says nothing new")
		require.Len(t, scanAnomalies(t, db, org.id, scanNow.Add(time.Hour)), 1, "nor does one an hour later, while it is still a spike")

		// A day on, and spending far above what is now the usual again.
		later := scanNow.Add(24*time.Hour + time.Minute)
		logUsage(t, db, key, staff.Id, later.Add(-time.Minute), 20*dollar, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleSpike}, rulesOf(scanAnomalies(t, db, org.id, later)))
	})
}

// Each rule keeps its own silence: a key that was reported for one thing today
// is still reported for another.
func TestScanAnomalies_OneRuleBeingQuietDoesNotSilenceAnother(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 3*dollar, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(scanAnomalies(t, db, org.id, scanNow)))

		// A few minutes later the key is used from somewhere.
		logUsage(t, db, key, staff.Id, scanNow.Add(5*time.Minute), 10, "192.0.2.77")
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleNewIP},
			rulesOf(scanAnomalies(t, db, org.id, scanNow.Add(10*time.Minute))))
	})
}

// The floor: however unusual, a day that cost less than a dollar is not
// reported — and a key that usually spends nothing is reported at the floor.
func TestScanAnomalies_ASpikeNeedsTheFloor(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), dollar-1, "")
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "a cent short of the floor")
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 1, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(scanAnomalies(t, db, org.id, scanNow)))
	})
}

// "Usual" is the seven days before the last one, and no further back: what a
// key spent a fortnight ago does not make today's spending look small.
func TestScanAnomalies_TheUsualIsTheWeekBeforeAndNoFurtherBack(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		logUsage(t, db, key, staff.Id, scanNow.Add(-8*24*time.Hour-time.Minute), 100*dollar, "")
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 2*dollar, "")
		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(alerts))
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Spent: 2 * dollar}, detailOf(t, alerts[0]), "nothing spent in the week before")

		// A minute later that old spending would have been within the week.
		again, againStaff, againKey := keyInSales(t, db, "Again")
		logUsage(t, db, againKey, againStaff.Id, scanNow.Add(-8*24*time.Hour+time.Minute), 100*dollar, "")
		logUsage(t, db, againKey, againStaff.Id, scanNow.Add(-time.Hour), 2*dollar, "")
		require.Empty(t, scanAnomalies(t, db, again.id, scanNow))
	})
}

// Only what was spent counts as spending: a refund line, an error line and
// another account's personal usage are not a spike.
func TestScanAnomalies_OnlySpendingCounts(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		for _, kind := range []int{platformmodel.LogTypeRefund, platformmodel.LogTypeError, platformmodel.LogTypeManage} {
			require.NoError(t, db.Create(&platformmodel.Log{
				UserId: staff.Id, TokenId: key.Id, OrgId: org.id, Type: kind, CreatedAt: scanNow.Add(-time.Hour).Unix(), Quota: 50 * dollar,
			}).Error)
		}
		require.NoError(t, db.Create(&platformmodel.Log{
			UserId: staff.Id, TokenId: key.Id, OrgId: 0, Type: platformmodel.LogTypeConsume, CreatedAt: scanNow.Add(-time.Hour).Unix(), Quota: 50 * dollar,
		}).Error)
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow))
	})
}

// The learning period: a key says nothing until it has been with its holder
// for seven days — counted from when it was made, or from when it last
// changed hands. Rotating it does not start the count over.
func TestScanAnomalies_AKeyIsLeftAloneWhileItIsNewToItsHolder(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		owner := actorFor(t, db, org.owner.Id)
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 3*dollar, "203.0.113.5")
		madeAt := func(age time.Duration) {
			require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).
				Update("created_time", scanNow.Add(-age).Unix()).Error)
		}

		madeAt(7*24*time.Hour - time.Second)
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "a second short of seven days old")
		madeAt(7 * 24 * time.Hour)
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleNewIP}, rulesOf(scanAnomalies(t, db, org.id, scanNow)))

		// The same old key, but it came to this holder three days ago.
		require.NoError(t, db.Where("org_id = ?", org.id).Delete(&orgmodel.OrgAlert{}).Error)
		madeAt(400 * 24 * time.Hour)
		recordHandover := func(action string, age time.Duration) {
			require.NoError(t, RecordAudit(db, AuditEntry{OrgId: org.id, ActorUserId: org.owner.Id, Action: action, TargetType: orgmodel.AuditTargetKey, TargetId: key.Id}))
			require.NoError(t, db.Model(&orgmodel.OrgAuditLog{}).
				Where("org_id = ? AND action = ? AND target_id = ?", org.id, action, key.Id).
				Update("created_time", scanNow.Add(-age).Unix()).Error)
		}
		recordHandover(orgmodel.AuditKeyAssign, 3*24*time.Hour)
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "assigned three days ago")

		// Taking a key back is a change of hands as well.
		require.NoError(t, db.Where("org_id = ?", org.id).Delete(&orgmodel.OrgAuditLog{}).Error)
		recordHandover(orgmodel.AuditKeyReclaim, 3*24*time.Hour)
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "taken back three days ago")

		// A rotation long after the last change of hands changes nothing.
		require.NoError(t, db.Where("org_id = ?", org.id).Delete(&orgmodel.OrgAuditLog{}).Error)
		recordHandover(orgmodel.AuditKeyAssign, 30*24*time.Hour)
		_, err := RotateKey(db, owner, key.Id)
		require.NoError(t, err)
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleNewIP}, rulesOf(scanAnomalies(t, db, org.id, scanNow)))
	})
}

// A key's history is read under its current holder only: what the one before
// spent is not what this one usually spends.
func TestScanAnomalies_ThePreviousHoldersUsageIsNotTheBaseline(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, before, key := keyInSales(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		after := seedMemberIn(t, db, org, "second-holder", presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)
		// The one before spent a hundred dollars a day; this one, who has had
		// the key for a long time, spends six today.
		logUsualWeek(t, db, key, before.Id, 100*dollar)
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("user_id", after.Id).Error)
		logUsage(t, db, key, after.Id, scanNow.Add(-time.Hour), 6*dollar, "")

		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(alerts))
		assert.Equal(t, after.Id, alerts[0].UserId)
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Spent: 6 * dollar}, detailOf(t, alerts[0]), "no usual spending of their own")
	})
}

// --- anomaly rule two: outside working hours ---------------------------------

// workWeek sets an organization's working hours: Monday to Friday, nine to
// six, on the clock of the given zone.
func workWeek(t *testing.T, db *gorm.DB, org testOrg, zone string) {
	t.Helper()
	settings := orgmodel.DefaultAlertSettings()
	settings.WorkHours = &orgmodel.WorkHours{Timezone: zone, Days: []int{1, 2, 3, 4, 5}, Start: 9 * 60, End: 18 * 60}
	_, err := UpdateAlertSettings(db, actorFor(t, db, org.owner.Id), settings)
	require.NoError(t, err)
}

// Acceptance: 配置的工作时间外大量调用. The rule is off until working hours are
// set; then spending outside them is reported once it reaches half the daily
// average and the floor.
func TestScanAnomalies_SpendingOutsideWorkingHoursIsReported(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		logUsualWeek(t, db, key, staff.Id, 4*dollar)
		// Three in the morning, the night before: two dollars.
		logUsage(t, db, key, staff.Id, scanNow.Add(-9*time.Hour), 2*dollar, "")
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "no working hours set: the rule is off")

		workWeek(t, db, org, "UTC")
		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleOffHours}, rulesOf(alerts))
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Spent: 2 * dollar, DailyAverage: 4 * dollar}, detailOf(t, alerts[0]))
		require.Len(t, scanAnomalies(t, db, org.id, scanNow.Add(10*time.Minute)), 1, "once a day")
	})
}

// The same spending during working hours is nobody's business, and neither is
// a night's spending that is small beside what the key usually spends.
func TestScanAnomalies_WorkingHoursAndSmallNightsAreNotReported(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		day, staff, dayKey := keyInSales(t, db, "Daytime")
		workWeek(t, db, day, "UTC")
		logUsualWeek(t, db, dayKey, staff.Id, 4*dollar)
		logUsage(t, db, dayKey, staff.Id, scanNow.Add(-2*time.Hour), 2*dollar, "") // ten in the morning
		require.Empty(t, scanAnomalies(t, db, day.id, scanNow))

		night, nightStaff, nightKey := keyInSales(t, db, "Nightly")
		workWeek(t, db, night, "UTC")
		logUsualWeek(t, db, nightKey, nightStaff.Id, 4*dollar)
		logUsage(t, db, nightKey, nightStaff.Id, scanNow.Add(-9*time.Hour), 2*dollar-1, "")
		require.Empty(t, scanAnomalies(t, db, night.id, scanNow), "a cent short of half the daily average")

		floor, floorStaff, floorKey := keyInSales(t, db, "Floor")
		workWeek(t, db, floor, "UTC")
		logUsage(t, db, floorKey, floorStaff.Id, scanNow.Add(-9*time.Hour), dollar-1, "")
		require.Empty(t, scanAnomalies(t, db, floor.id, scanNow), "a cent short of the floor, with no usual spending at all")
	})
}

// Working hours are read on the organization's own clock, to the minute: the
// last minute before nine is night, nine sharp is not — also in a zone that is
// not a whole number of hours from the server's.
func TestScanAnomalies_WorkingHoursFollowTheOrganizationsClock(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		// Kathmandu is UTC+05:45: nine in the morning there is 03:15 UTC.
		nineSharp := time.Date(2026, 10, 8, 3, 15, 0, 0, time.UTC)

		early, earlyStaff, earlyKey := keyInSales(t, db, "Early")
		workWeek(t, db, early, "Asia/Kathmandu")
		logUsage(t, db, earlyKey, earlyStaff.Id, nineSharp.Add(-time.Second), dollar, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleOffHours}, rulesOf(scanAnomalies(t, db, early.id, scanNow)))

		punctual, punctualStaff, punctualKey := keyInSales(t, db, "Punctual")
		workWeek(t, db, punctual, "Asia/Kathmandu")
		logUsage(t, db, punctualKey, punctualStaff.Id, nineSharp, dollar, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike}, rulesOf(scanAnomalies(t, db, punctual.id, scanNow)))

		// The weekend is outside working hours whatever the time of day.
		// Looked at on Monday morning, a dollar spent on Sunday at noon.
		monday := time.Date(2026, 10, 12, 6, 0, 0, 0, time.UTC)
		weekend, weekendStaff, weekendKey := keyInSales(t, db, "Weekend")
		workWeek(t, db, weekend, "UTC")
		logUsage(t, db, weekendKey, weekendStaff.Id, monday.Add(-18*time.Hour), dollar, "")
		require.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleOffHours}, rulesOf(scanAnomalies(t, db, weekend.id, monday)))
	})
}

// --- anomaly rule three: an unfamiliar address ---------------------------------

// Acceptance: 该 key 30 天内未见过的 IP. An address the key has been used from in
// the 30 days before is familiar; any other is reported, once.
func TestScanAnomalies_AnUnfamiliarAddressIsReported(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		office, home := "203.0.113.5", "198.51.100.9"
		logUsage(t, db, key, staff.Id, scanNow.Add(-3*24*time.Hour), 10, office)
		logUsage(t, db, key, staff.Id, scanNow.Add(-29*24*time.Hour), 10, home)

		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 10, office)
		logUsage(t, db, key, staff.Id, scanNow.Add(-2*time.Hour), 10, home)
		logUsage(t, db, key, staff.Id, scanNow.Add(-3*time.Hour), 10, "") // a line that recorded no address
		require.Empty(t, scanAnomalies(t, db, org.id, scanNow), "both addresses are ones it has been used from")

		logUsage(t, db, key, staff.Id, scanNow.Add(-30*time.Minute), 10, "192.0.2.77")
		logUsage(t, db, key, staff.Id, scanNow.Add(-20*time.Minute), 10, "192.0.2.77")
		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleNewIP}, rulesOf(alerts))
		assert.Equal(t, orgmodel.AlertDetail{Key: "Design tools", Ips: []string{"192.0.2.77"}, KnownIps: 2}, detailOf(t, alerts[0]))
		require.Len(t, scanAnomalies(t, db, org.id, scanNow.Add(10*time.Minute)), 1, "once a day")

		// A day on the address is one the key has been used from.
		tomorrow := scanNow.Add(24*time.Hour + time.Minute)
		logUsage(t, db, key, staff.Id, tomorrow.Add(-time.Minute), 10, "192.0.2.77")
		require.Len(t, scanAnomalies(t, db, org.id, tomorrow), 1)
	})
}

// What counts as familiar: this key, this holder, the last 30 days. An address
// seen longer ago, on another key, or under the holder before is not.
func TestScanAnomalies_WhatMakesAnAddressFamiliar(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		address := "203.0.113.5"

		forgotten, staff, key := keyInSales(t, db, "Forgotten")
		logUsage(t, db, key, staff.Id, scanNow.Add(-31*24*time.Hour), 10, address)
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 10, address)
		alerts := scanAnomalies(t, db, forgotten.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleNewIP}, rulesOf(alerts), "last seen 31 days ago")
		assert.Zero(t, detailOf(t, alerts[0]).KnownIps)

		otherKey, staff, key := keyInSales(t, db, "OtherKey")
		second := seedKey(t, db, otherKey, staff.Id, "Second")
		logUsage(t, db, second, staff.Id, scanNow.Add(-3*24*time.Hour), 10, address)
		logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 10, address)
		alerts = scanAnomalies(t, db, otherKey.id, scanNow)
		require.Equal(t, []string{orgmodel.AlertRuleNewIP}, rulesOf(alerts), "the holder's other key has been there, this one has not")
		assert.Equal(t, key.Id, alerts[0].TokenId)

		handedOn, before, key := keyInSales(t, db, "HandedOn")
		after := seedMember(t, db, handedOn, "after-of-HandedOn", orgmodel.RoleStaff)
		logUsage(t, db, key, before.Id, scanNow.Add(-3*24*time.Hour), 10, address)
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("user_id", after.Id).Error)
		logUsage(t, db, key, after.Id, scanNow.Add(-time.Hour), 10, address)
		require.Equal(t, []string{orgmodel.AlertRuleNewIP}, rulesOf(scanAnomalies(t, db, handedOn.id, scanNow)),
			"where the holder before worked is not where this one does")
	})
}

// One alert names the new addresses, five at most, however many there are.
func TestScanAnomalies_AnAlertListsAtMostFiveAddresses(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		for last := 1; last <= 8; last++ {
			logUsage(t, db, key, staff.Id, scanNow.Add(-time.Hour), 10, fmt.Sprintf("192.0.2.%d", last))
		}
		alerts := scanAnomalies(t, db, org.id, scanNow)
		require.Len(t, alerts, 1)
		assert.Equal(t, []string{"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4", "192.0.2.5"}, detailOf(t, alerts[0]).Ips)
	})
}

// --- across the three rules ----------------------------------------------------

// One organization's usage never raises an alert in another, and an
// organization nobody used today is not looked into at all.
func TestScanAnomalies_KeepsOrganizationsApart(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		acme, acmeStaff, acmeKey := keyInSales(t, db, "Acme")
		globex, globexStaff, globexKey := keyInSales(t, db, "Globex")
		idle, _, _ := keyInSales(t, db, "Idle")
		logUsage(t, db, acmeKey, acmeStaff.Id, scanNow.Add(-time.Hour), 3*dollar, "203.0.113.5")
		logUsage(t, db, globexKey, globexStaff.Id, scanNow.Add(-time.Hour), 10, "")

		raised, err := ScanAnomalies(db, db, scanNow)
		require.NoError(t, err)
		assert.Equal(t, 2, raised)
		assert.Equal(t, []string{orgmodel.AlertRuleSpike, orgmodel.AlertRuleNewIP}, rulesOf(alertsOf(t, db, acme.id)))
		assert.Empty(t, alertsOf(t, db, globex.id))
		assert.Empty(t, alertsOf(t, db, idle.id))
	})
}

// PRD D13: alerts notify and never block. Whatever the scans find, the key is
// exactly what it was — still switched on, with the value and the quota it had.
func TestScans_NeverTouchTheKey(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org, staff, key := keyInSales(t, db, "Acme")
		workWeek(t, db, org, "UTC")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("monthly_limit", 10).Error)
		setAllowance(t, db, key.Id, 900, 100)
		logUsage(t, db, key, staff.Id, scanNow.Add(-9*time.Hour), 50*dollar, "192.0.2.77")
		before := reloadKey(t, db, key.Id)
		holder := reloadUser(t, db, staff.Id)

		_, err := ScanAllowances(db, 0, scanNow, monthlyUsageOf(map[int]int{key.Id: 10}))
		require.NoError(t, err)
		_, err = ScanAnomalies(db, db, scanNow)
		require.NoError(t, err)
		_, err = WarnRefused(db, key.Id, 200, scanNow)
		require.NoError(t, err)

		require.Equal(t, []string{"quota@80", "monthly@100", orgmodel.AlertRuleSpike, orgmodel.AlertRuleOffHours, orgmodel.AlertRuleNewIP, "quota@100"},
			rulesOf(alertsOf(t, db, org.id)), "every rule fired")
		assert.Equal(t, before, reloadKey(t, db, key.Id), "and the key is untouched")
		assert.Equal(t, common.TokenStatusEnabled, reloadKey(t, db, key.Id).Status)
		assert.Equal(t, holder, reloadUser(t, db, staff.Id), "as is its holder")
	})
}
