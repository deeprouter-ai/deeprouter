package service

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Warnings and anomaly alerts (PRD §4, §7.4). Both are scans a background task
// runs: a scan reads what requests left behind — a key's row, the usage log —
// decides, and writes rows to org_alerts. The one exception is WarnRefused,
// which the gateway calls after it has refused a request a key could not pay
// for: a refusal leaves nothing behind for a scan to read. Nothing here ever
// stops a request (PRD D13, red line 4). Telling people about the rows is
// alert_digest.go.

// The windows of the anomaly rules. PRD §4 fixes them; what an organization
// may change is in orgmodel.AlertSettings.
const (
	// alertRecentWindow is "a day" in the rules: the last 24 hours, so that a
	// rule works the same at one in the morning as at noon.
	alertRecentWindow = 24 * time.Hour
	// alertBaselineDays is how many days before that a key's usual spending is
	// averaged over.
	alertBaselineDays = 7
	// alertLearningPeriod is how long a key has to have been with its holder
	// before an anomaly rule says anything about it.
	alertLearningPeriod = 7 * 24 * time.Hour
	// alertAddressMemory is how far back an address counts as one the key has
	// been used from.
	alertAddressMemory = 30 * 24 * time.Hour
	// alertAddressesShown caps the addresses one alert lists.
	alertAddressesShown = 5
	// usageBucketSeconds is the grain usage is summed in when it has to be
	// told apart by the hour of day: every real time zone is a multiple of 15
	// minutes away from every other.
	usageBucketSeconds = 900
	// scanChunk bounds how many ids go into one IN list.
	scanChunk = 500
)

// MonthlyUsage answers how many requests a key has made this calendar month.
// The gateway keeps that count itself, in Redis or in memory (internal/quota),
// so whoever runs the scan supplies it.
type MonthlyUsage func(keyID int) (int, error)

// ScanAllowances raises a warning for every organization key that has reached
// one of its organization's warning levels (PRD §4, D18): of the quota it was
// given, or of the requests it may make in a month. since narrows the look to
// the keys used from then on — zero looks at all of them — and the answer is
// how many warnings were raised.
//
// A key is warned about once per level and cycle. The cycle of the monthly
// limit is the calendar month. The cycle of the quota is what the key was
// given in all, used and remaining together: spending and refunds leave that
// sum alone, and giving the key more starts a new cycle.
func ScanAllowances(db *gorm.DB, since int64, now time.Time, monthly MonthlyUsage) (int, error) {
	var keys []platformmodel.Token
	if err := db.Select("id", "user_id", "org_id", "name", "remain_quota", "used_quota", "unlimited_quota", "monthly_limit").
		Where("org_id > ? AND accessed_time >= ? AND (unlimited_quota = ? OR monthly_limit > ?)", 0, since, false, 0).
		Order("id").Find(&keys).Error; err != nil {
		return 0, err
	}
	levelsOf := map[int][]int{}
	var reached []orgmodel.OrgAlert
	var failed error
	for i := range keys {
		key := &keys[i]
		levels, known := levelsOf[key.OrgId]
		if !known {
			settings, err := alertSettingsOf(db, key.OrgId)
			if err != nil {
				return 0, err
			}
			levels = settings.WarnAt
			levelsOf[key.OrgId] = levels
		}
		if !key.UnlimitedQuota {
			given := key.UsedQuota + key.RemainQuota
			if level := orgmodel.ReachedLevel(levels, key.UsedQuota, given); level > 0 {
				reached = append(reached, warningFor(key, orgmodel.AlertRuleQuota, "q"+strconv.Itoa(given), level, key.UsedQuota, given))
			}
		}
		if key.MonthlyLimit > 0 {
			used, err := monthly(key.Id)
			if err != nil {
				// One key's count being out of reach must not cost the
				// others their warnings.
				failed = errors.Join(failed, fmt.Errorf("monthly usage of key %d: %w", key.Id, err))
				continue
			}
			if level := orgmodel.ReachedLevel(levels, used, key.MonthlyLimit); level > 0 {
				reached = append(reached, warningFor(key, orgmodel.AlertRuleMonthly, now.Format("2006-01"), level, used, key.MonthlyLimit))
			}
		}
	}
	fresh, err := withoutRepeatedWarnings(db, reached)
	if err != nil {
		return 0, errors.Join(failed, err)
	}
	raised, err := raiseAlerts(db, fresh, now)
	return raised, errors.Join(failed, err)
}

// WarnRefused raises the quota warning for an organization key the gateway has
// just refused a request, because what the key had left was less than the
// request could cost (PRD D45). Nothing was spent, so no scan would ever see
// it; and for the key it is the end of its quota — a client that asks for a
// large max_tokens is turned away with a few percent unused — so the warning
// is the last level's, 100%, raised once per cycle like any other: a key
// already told it was used up is not told again, and one that is refused is
// not told about 100% afterwards. The answer is how many warnings were
// raised. A key with no limit, a key that could in fact have paid (the refusal
// was about something else), and an organization that has switched its
// warnings off get nothing.
func WarnRefused(db *gorm.DB, keyID int, needed int, now time.Time) (int, error) {
	var keys []platformmodel.Token
	if err := db.Select("id", "user_id", "org_id", "name", "remain_quota", "used_quota", "unlimited_quota").
		Where("id = ? AND org_id > ?", keyID, 0).Limit(1).Find(&keys).Error; err != nil {
		return 0, err
	}
	if len(keys) == 0 || keys[0].UnlimitedQuota || keys[0].RemainQuota >= needed {
		return 0, nil
	}
	key := &keys[0]
	settings, err := alertSettingsOf(db, key.OrgId)
	if err != nil {
		return 0, err
	}
	if len(settings.WarnAt) == 0 {
		return 0, nil
	}
	given := key.UsedQuota + key.RemainQuota
	warning := warningFor(key, orgmodel.AlertRuleQuota, "q"+strconv.Itoa(given), 100, key.UsedQuota, given)
	warning.Detail = alertDetailJSON(orgmodel.AlertDetail{Key: key.Name, Used: key.UsedQuota, Limit: given, Needed: needed})
	fresh, err := withoutRepeatedWarnings(db, []orgmodel.OrgAlert{warning})
	if err != nil {
		return 0, err
	}
	return raiseAlerts(db, fresh, now)
}

// warningFor words one warning about a key: which rule, which level of which
// cycle, and the two numbers behind it.
func warningFor(key *platformmodel.Token, rule string, cycle string, level int, used int, limit int) orgmodel.OrgAlert {
	return orgmodel.OrgAlert{
		OrgId:   key.OrgId,
		Rule:    rule,
		TokenId: key.Id,
		UserId:  key.UserId,
		Cycle:   cycle,
		Level:   level,
		Detail:  alertDetailJSON(orgmodel.AlertDetail{Key: key.Name, Used: used, Limit: limit}),
	}
}

// alertDetailJSON renders the numbers behind an alert for org_alerts.detail.
func alertDetailJSON(detail orgmodel.AlertDetail) string {
	encoded, err := common.Marshal(detail)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// withoutRepeatedWarnings drops the warnings a key has had already: one at the
// same level or a higher one, in the same cycle. A key that jumps straight to
// 100% is told once, not once per level on the way.
func withoutRepeatedWarnings(db *gorm.DB, reached []orgmodel.OrgAlert) ([]orgmodel.OrgAlert, error) {
	if len(reached) == 0 {
		return nil, nil
	}
	keyIDs := make([]int, 0, len(reached))
	for _, warning := range reached {
		keyIDs = append(keyIDs, warning.TokenId)
	}
	slices.Sort(keyIDs)
	keyIDs = slices.Compact(keyIDs)
	type cycle struct {
		keyID int
		rule  string
		cycle string
	}
	told := map[cycle]int{}
	for chunk := range slices.Chunk(keyIDs, scanChunk) {
		var rows []struct {
			TokenId   int
			Rule      string
			WarnCycle string
			Highest   int
		}
		if err := db.Model(&orgmodel.OrgAlert{}).
			Select("token_id, rule, warn_cycle, MAX(warn_level) AS highest").
			Where("token_id IN ? AND rule IN ?", chunk, orgmodel.WarningRules).
			Group("token_id, rule, warn_cycle").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			told[cycle{row.TokenId, row.Rule, row.WarnCycle}] = row.Highest
		}
	}
	fresh := make([]orgmodel.OrgAlert, 0, len(reached))
	for _, warning := range reached {
		if told[cycle{warning.TokenId, warning.Rule, warning.Cycle}] < warning.Level {
			fresh = append(fresh, warning)
		}
	}
	return fresh, nil
}

// raiseAlerts writes alerts to the organization's list, each stamped with the
// department its key's holder is in right now, and returns how many.
func raiseAlerts(db *gorm.DB, alerts []orgmodel.OrgAlert, now time.Time) (int, error) {
	if len(alerts) == 0 {
		return 0, nil
	}
	holderIDs := make([]int, 0, len(alerts))
	for _, alert := range alerts {
		holderIDs = append(holderIDs, alert.UserId)
	}
	slices.Sort(holderIDs)
	holderIDs = slices.Compact(holderIDs)
	departmentOf := make(map[int]int, len(holderIDs))
	for chunk := range slices.Chunk(holderIDs, scanChunk) {
		var holders []platformmodel.User
		if err := db.Select("id", "department_id").Where("id IN ?", chunk).Find(&holders).Error; err != nil {
			return 0, err
		}
		for _, holder := range holders {
			departmentOf[holder.Id] = holder.DepartmentId
		}
	}
	for i := range alerts {
		alerts[i].DepartmentId = departmentOf[alerts[i].UserId]
		alerts[i].CreatedTime = now.Unix()
	}
	if err := db.CreateInBatches(&alerts, 100).Error; err != nil {
		return 0, err
	}
	return len(alerts), nil
}

// ScanAnomalies checks the keys of every organization against the three
// anomaly rules of PRD §4 — a spike in spending, spending outside working
// hours, an unfamiliar address — and raises an alert for each hit. It reads
// the usage log, which may live in a database of its own, and answers how many
// alerts were raised. An organization that cannot be scanned does not stop the
// others.
func ScanAnomalies(db *gorm.DB, logDB *gorm.DB, now time.Time) (int, error) {
	var orgs []orgmodel.Organization
	if err := db.Select("id", "alert_settings").Order("id").Find(&orgs).Error; err != nil {
		return 0, err
	}
	raised := 0
	var failed error
	for _, org := range orgs {
		alerts, err := anomaliesOf(db, logDB, org.Id, orgmodel.ParseAlertSettings(org.AlertSettings), now)
		if err == nil {
			var n int
			n, err = raiseAlerts(db, alerts, now)
			raised += n
		}
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("organization %d: %w", org.Id, err))
		}
	}
	return raised, failed
}

// keyUser names whose use of a key a number is about. A key's history is read
// under its current holder only: what the one before spent is not what this
// one usually spends, and where they worked is not where this one does.
type keyUser struct {
	keyID  int
	userID int
}

// anomaliesOf applies the three rules to one organization's keys and returns
// the alerts to raise. A key is left alone while it has been with its holder
// for less than the learning period, and a rule that has spoken about a key
// stays quiet about it for a day.
func anomaliesOf(db *gorm.DB, logDB *gorm.DB, orgID int, settings orgmodel.AlertSettings, now time.Time) ([]orgmodel.OrgAlert, error) {
	recent := now.Add(-alertRecentWindow).Unix()
	baseline := now.Add(-alertRecentWindow - alertBaselineDays*24*time.Hour).Unix()
	usage := func() *gorm.DB {
		return logDB.Model(&platformmodel.Log{}).Where("org_id = ? AND type = ?", orgID, platformmodel.LogTypeConsume)
	}

	// The off-hours rule needs spending by the time of day; without working
	// hours one sum per key is enough.
	var zone *time.Location
	if settings.WorkHours != nil {
		zone, _ = settings.WorkHours.Location()
	}
	selected, grouped := "token_id, user_id, 0 AS bucket, SUM(quota) AS quota", "token_id, user_id"
	if zone != nil {
		bucket := fmt.Sprintf("created_at - created_at %% %d", usageBucketSeconds)
		selected, grouped = "token_id, user_id, "+bucket+" AS bucket, SUM(quota) AS quota", "token_id, user_id, "+bucket
	}
	var spending []struct {
		TokenId int
		UserId  int
		Bucket  int64
		Quota   int64
	}
	if err := usage().Select(selected).Where("created_at >= ?", recent).Group(grouped).Scan(&spending).Error; err != nil {
		return nil, err
	}
	if len(spending) == 0 {
		return nil, nil
	}
	spent, spentOffHours := map[keyUser]int64{}, map[keyUser]int64{}
	var keyIDs []int
	for _, row := range spending {
		who := keyUser{row.TokenId, row.UserId}
		spent[who] += row.Quota
		if zone != nil && !settings.WorkHours.Covers(time.Unix(row.Bucket, 0), zone) {
			spentOffHours[who] += row.Quota
		}
		keyIDs = append(keyIDs, row.TokenId)
	}
	slices.Sort(keyIDs)
	keyIDs = slices.Compact(keyIDs)

	var usual []struct {
		TokenId int
		UserId  int
		Quota   int64
	}
	if err := usage().Select("token_id, user_id, SUM(quota) AS quota").
		Where("created_at >= ? AND created_at < ?", baseline, recent).
		Group("token_id, user_id").Scan(&usual).Error; err != nil {
		return nil, err
	}
	weekBefore := make(map[keyUser]int64, len(usual))
	for _, row := range usual {
		weekBefore[keyUser{row.TokenId, row.UserId}] = row.Quota
	}

	recentAddresses, err := addressesUsed(usage().Where("created_at >= ?", recent))
	if err != nil {
		return nil, err
	}
	// The 30 days before are read only for the keys that have an address to
	// judge: it is the widest stretch of the usage log this scan touches.
	var addressed []int
	for who := range recentAddresses {
		addressed = append(addressed, who.keyID)
	}
	slices.Sort(addressed)
	addressed = slices.Compact(addressed)
	knownAddresses := map[keyUser][]string{}
	for chunk := range slices.Chunk(addressed, scanChunk) {
		known, err := addressesUsed(usage().Where("created_at >= ? AND created_at < ? AND token_id IN ?",
			now.Add(-alertAddressMemory).Unix(), recent, chunk))
		if err != nil {
			return nil, err
		}
		for who, addresses := range known {
			knownAddresses[who] = addresses
		}
	}

	var alerts []orgmodel.OrgAlert
	for chunk := range slices.Chunk(keyIDs, scanChunk) {
		var keys []platformmodel.Token
		if err := db.Select("id", "user_id", "org_id", "name", "created_time").
			Where("org_id = ? AND id IN ?", orgID, chunk).Order("id").Find(&keys).Error; err != nil {
			return nil, err
		}
		heldSince, err := lastHandovers(db, orgID, chunk)
		if err != nil {
			return nil, err
		}
		quiet, err := recentlyAlerted(db, orgID, chunk, recent)
		if err != nil {
			return nil, err
		}
		for i := range keys {
			key := &keys[i]
			if now.Unix()-max(key.CreatedTime, heldSince[key.Id]) < int64(alertLearningPeriod/time.Second) {
				continue
			}
			who := keyUser{key.Id, key.UserId}
			raise := func(rule string, detail orgmodel.AlertDetail) {
				if quiet[rule][key.Id] {
					return
				}
				detail.Key = key.Name
				alerts = append(alerts, orgmodel.OrgAlert{
					OrgId: orgID, Rule: rule, TokenId: key.Id, UserId: key.UserId, Detail: alertDetailJSON(detail),
				})
			}
			dailyAverage := weekBefore[who] / alertBaselineDays
			// Spent in a day against the daily average: compared as
			// spent*days against multiple*sum, so nothing is lost to rounding.
			if day := spent[who]; day > 0 && day >= int64(settings.MinSpend) &&
				day*alertBaselineDays > int64(settings.SpikeMultiple)*weekBefore[who] {
				raise(orgmodel.AlertRuleSpike, orgmodel.AlertDetail{Spent: int(day), DailyAverage: int(dailyAverage)})
			}
			if off := spentOffHours[who]; off > 0 && off >= int64(settings.MinSpend) &&
				off*alertBaselineDays*100 >= int64(settings.OffHoursPercent)*weekBefore[who] {
				raise(orgmodel.AlertRuleOffHours, orgmodel.AlertDetail{Spent: int(off), DailyAverage: int(dailyAverage)})
			}
			var unfamiliar []string
			for _, address := range recentAddresses[who] {
				if !slices.Contains(knownAddresses[who], address) {
					unfamiliar = append(unfamiliar, address)
				}
			}
			if len(unfamiliar) > 0 {
				raise(orgmodel.AlertRuleNewIP, orgmodel.AlertDetail{
					Ips:      unfamiliar[:min(len(unfamiliar), alertAddressesShown)],
					KnownIps: len(knownAddresses[who]),
				})
			}
		}
	}
	return alerts, nil
}

// addressesUsed returns the addresses each key was used from by each of its
// users, in order, within the usage the query is narrowed to.
func addressesUsed(usage *gorm.DB) (map[keyUser][]string, error) {
	var rows []struct {
		TokenId int
		UserId  int
		Ip      string
	}
	if err := usage.Select("token_id, user_id, ip").Where("ip <> ?", "").
		Group("token_id, user_id, ip").Order("ip").Scan(&rows).Error; err != nil {
		return nil, err
	}
	addresses := map[keyUser][]string{}
	for _, row := range rows {
		who := keyUser{row.TokenId, row.UserId}
		addresses[who] = append(addresses[who], row.Ip)
	}
	return addresses, nil
}

// lastHandovers returns when each key last changed hands — was assigned or
// taken back — for the keys that ever did. A key's learning period starts
// over then: to its new holder it is a new key, and it was given a new value
// (PRD §3). Rotating a key does not start it over.
func lastHandovers(db *gorm.DB, orgID int, keyIDs []int) (map[int]int64, error) {
	var rows []struct {
		TargetId int
		MovedAt  int64
	}
	if err := db.Model(&orgmodel.OrgAuditLog{}).
		Select("target_id, MAX(created_time) AS moved_at").
		Where("org_id = ? AND target_type = ? AND action IN ? AND target_id IN ?",
			orgID, orgmodel.AuditTargetKey, []string{orgmodel.AuditKeyAssign, orgmodel.AuditKeyReclaim}, keyIDs).
		Group("target_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	movedAt := make(map[int]int64, len(rows))
	for _, row := range rows {
		movedAt[row.TargetId] = row.MovedAt
	}
	return movedAt, nil
}

// recentlyAlerted returns, per anomaly rule, the keys it has raised an alert
// about since the given moment — whatever became of that alert. The same key
// and rule speak once a day (PRD §4).
func recentlyAlerted(db *gorm.DB, orgID int, keyIDs []int, since int64) (map[string]map[int]bool, error) {
	var rows []struct {
		TokenId int
		Rule    string
	}
	if err := db.Model(&orgmodel.OrgAlert{}).Select("token_id, rule").
		Where("org_id = ? AND rule IN ? AND created_time > ? AND token_id IN ?", orgID, orgmodel.AnomalyRules, since, keyIDs).
		Group("token_id, rule").Scan(&rows).Error; err != nil {
		return nil, err
	}
	quiet := map[string]map[int]bool{}
	for _, row := range rows {
		if quiet[row.Rule] == nil {
			quiet[row.Rule] = map[int]bool{}
		}
		quiet[row.Rule][row.TokenId] = true
	}
	return quiet, nil
}
