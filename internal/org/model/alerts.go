package model

import (
	"errors"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// The rules an alert can come from (org_alerts.rule). The first two are
// warnings about what a key was given; the other three are the anomaly rules
// of PRD §4. None of them ever stops a request (PRD D13).
const (
	AlertRuleQuota    = "quota"    // the key has used a share of its quota
	AlertRuleMonthly  = "monthly"  // the key has made a share of the requests it may make in a month
	AlertRuleSpike    = "spike"    // the key spent far more within a day than it usually does
	AlertRuleOffHours = "offhours" // the key spent a lot outside the organization's working hours
	AlertRuleNewIP    = "new_ip"   // the key was used from an address it had not been used from
)

// WarningRules are the rules a key's own holder is told about, besides the
// owner and the admins: they are about what the key has left.
var WarningRules = []string{AlertRuleQuota, AlertRuleMonthly}

// AnomalyRules are the rules that say a key is being used unlike before. Only
// the owner and the admins are told.
var AnomalyRules = []string{AlertRuleSpike, AlertRuleOffHours, AlertRuleNewIP}

// What an owner or admin made of an alert (org_alerts.state).
const (
	AlertStateOpen       = ""            // nobody has dealt with it yet
	AlertStateHandled    = "handled"     // looked into and dealt with
	AlertStateFalseAlarm = "false_alarm" // nothing was wrong
)

// AlertDetail is the numbers behind an alert, kept as JSON in
// org_alerts.detail. Each rule fills in its own fields; a number that is
// missing is zero.
type AlertDetail struct {
	// Key is the key's name when the alert was raised, so the alert still
	// reads right once the key is renamed or deleted.
	Key string `json:"key"`
	// Used and Limit belong to the two warnings: quota units used out of what
	// the key was given in all, or requests made this month out of the
	// monthly limit.
	Used  int `json:"used,omitempty"`
	Limit int `json:"limit,omitempty"`
	// Needed marks a quota warning raised because the gateway refused a
	// request (PRD D45): what that request had to have in hand, in quota
	// units, which was more than the key had left.
	Needed int `json:"needed,omitempty"`
	// Spent and DailyAverage belong to spike and offhours, in quota units:
	// what the key spent in the last 24 hours — outside working hours only,
	// for offhours — and what it spent per day over the 7 days before.
	Spent        int `json:"spent,omitempty"`
	DailyAverage int `json:"daily_average,omitempty"`
	// Ips and KnownIps belong to new_ip: the addresses that are new, and how
	// many others the key had been used from in the 30 days before.
	Ips      []string `json:"ips,omitempty"`
	KnownIps int      `json:"known_ips,omitempty"`
}

// AlertSettings is what an organization may change about its warnings and
// alerts (PRD §4). The zero value is not a setting: start from
// DefaultAlertSettings.
type AlertSettings struct {
	// WarnAt lists the shares of a key's quota or monthly limit, in percent, at
	// which the key's holder and the admins are told. Empty means no warnings.
	WarnAt []int `json:"warn_at"`
	// SpikeMultiple is how many times its daily average of the week before a
	// key has to spend within a day to count as a spike.
	SpikeMultiple int `json:"spike_multiple"`
	// OffHoursPercent is the share of that daily average a key has to spend
	// outside working hours within a day to be reported.
	OffHoursPercent int `json:"off_hours_percent"`
	// MinSpend is the floor under both of those rules, in quota units: a key
	// that spent less than this is not reported, however unusual that was.
	MinSpend int `json:"min_spend"`
	// WorkHours is when the organization works. The off-hours rule is off
	// until an owner or admin sets it.
	WorkHours *WorkHours `json:"work_hours"`
}

// WorkHours is the working week of an organization.
type WorkHours struct {
	Timezone string `json:"timezone"` // an IANA zone name, such as "Asia/Shanghai"
	Days     []int  `json:"days"`     // the working days: 0 is Sunday, 6 is Saturday
	Start    int    `json:"start"`    // when a working day starts, in minutes after midnight
	End      int    `json:"end"`      // when it ends, likewise; later than Start
}

// The ranges a setting is accepted in.
const (
	maxWarnLevels      = 5
	maxSpikeMultiple   = 1000
	maxOffHoursPercent = 1000
	minutesPerDay      = 24 * 60
)

// DefaultAlertSettings returns what an organization starts with (PRD §4, D18):
// warnings at 80% and 100%, a spike at 5 times the daily average, off-hours
// spending at half of it, nothing below one dollar, and no working hours.
func DefaultAlertSettings() AlertSettings {
	return AlertSettings{
		WarnAt:          []int{80, 100},
		SpikeMultiple:   5,
		OffHoursPercent: 50,
		MinSpend:        int(common.QuotaPerUnit),
	}
}

// ParseAlertSettings reads what organizations.alert_settings stores. An
// organization that never changed anything stores nothing and gets the
// defaults, and so does one whose stored settings no longer pass the checks.
func ParseAlertSettings(stored string) AlertSettings {
	if stored == "" {
		return DefaultAlertSettings()
	}
	var settings AlertSettings
	if err := common.UnmarshalJsonStr(stored, &settings); err != nil {
		return DefaultAlertSettings()
	}
	normalized, ok := settings.Normalized()
	if !ok {
		return DefaultAlertSettings()
	}
	return normalized
}

// Normalized returns the settings in the one form they are stored in — levels
// and days in order and without duplicates — and reports false when a value is
// out of range.
func (s AlertSettings) Normalized() (AlertSettings, bool) {
	levels := slices.Clone(s.WarnAt)
	slices.Sort(levels)
	levels = slices.Compact(levels)
	if len(levels) > maxWarnLevels {
		return AlertSettings{}, false
	}
	for _, level := range levels {
		if level < 1 || level > 100 {
			return AlertSettings{}, false
		}
	}
	if levels == nil {
		levels = []int{}
	}
	if s.SpikeMultiple < 2 || s.SpikeMultiple > maxSpikeMultiple ||
		s.OffHoursPercent < 1 || s.OffHoursPercent > maxOffHoursPercent ||
		s.MinSpend < 0 {
		return AlertSettings{}, false
	}
	normalized := AlertSettings{
		WarnAt:          levels,
		SpikeMultiple:   s.SpikeMultiple,
		OffHoursPercent: s.OffHoursPercent,
		MinSpend:        s.MinSpend,
	}
	if s.WorkHours != nil {
		hours, ok := s.WorkHours.normalized()
		if !ok {
			return AlertSettings{}, false
		}
		normalized.WorkHours = &hours
	}
	return normalized, true
}

// normalized returns the working week with its days in order and without
// duplicates, and reports false when it names an unknown zone, no day at all,
// or hours that do not make a day.
func (w WorkHours) normalized() (WorkHours, bool) {
	if _, err := w.Location(); err != nil {
		return WorkHours{}, false
	}
	days := slices.Clone(w.Days)
	slices.Sort(days)
	days = slices.Compact(days)
	if len(days) == 0 || days[0] < 0 || days[len(days)-1] > 6 {
		return WorkHours{}, false
	}
	if w.Start < 0 || w.End > minutesPerDay || w.Start >= w.End {
		return WorkHours{}, false
	}
	return WorkHours{Timezone: w.Timezone, Days: days, Start: w.Start, End: w.End}, true
}

// errUnnamedZone means working hours came without a time zone of their own.
var errUnnamedZone = errors.New("working hours need a named time zone")

// Location returns the zone the working hours are told in. "Local" and the
// empty name are refused: they would mean wherever the server happens to run.
func (w WorkHours) Location() (*time.Location, error) {
	if w.Timezone == "" || w.Timezone == "Local" {
		return nil, errUnnamedZone
	}
	return time.LoadLocation(w.Timezone)
}

// Covers reports whether a moment falls within working hours, read on a clock
// in zone — which is what Location returns.
func (w WorkHours) Covers(moment time.Time, zone *time.Location) bool {
	local := moment.In(zone)
	minute := local.Hour()*60 + local.Minute()
	return slices.Contains(w.Days, int(local.Weekday())) && minute >= w.Start && minute < w.End
}

// ReachedLevel returns the highest of the levels — shares in percent — that
// used out of limit has reached, or 0 when it has reached none of them.
func ReachedLevel(levels []int, used int, limit int) int {
	if limit <= 0 {
		return 0
	}
	reached := 0
	for _, level := range levels {
		if level > reached && int64(used)*100 >= int64(level)*int64(limit) {
			reached = level
		}
	}
	return reached
}
