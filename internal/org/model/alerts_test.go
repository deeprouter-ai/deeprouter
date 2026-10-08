package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Enterprise Org P8 (meta-repo docs/enterprise-org-prd.md §4): the decisions
// behind warnings and alerts that need no database — which level a key has
// reached, whether a moment is within working hours, and which settings an
// organization may save.

// PRD D18: the defaults are 80% and 100%. A key is told about the highest
// level it has reached, and "reached" means at or above.
func TestReachedLevel(t *testing.T) {
	defaults := []int{80, 100}
	for _, c := range []struct {
		name   string
		levels []int
		used   int
		limit  int
		want   int
	}{
		{"nothing used", defaults, 0, 1000, 0},
		{"one short of the first level", defaults, 799, 1000, 0},
		{"exactly the first level", defaults, 800, 1000, 80},
		{"between the two", defaults, 999, 1000, 80},
		{"exactly the limit", defaults, 1000, 1000, 100},
		{"past the limit", defaults, 1300, 1000, 100},
		{"a limit that does not divide evenly", defaults, 8, 9, 80},
		{"and one short of it", defaults, 7, 9, 0},
		{"levels in any order", []int{100, 50, 80}, 600, 1000, 50},
		{"the highest level reached, wherever it stands in the list", []int{80, 50}, 900, 1000, 80},
		{"no levels at all", nil, 1000, 1000, 0},
		{"a key that was given nothing", defaults, 0, 0, 0},
		{"more refunded than spent", defaults, -5, 1000, 0},
		{"numbers that would overflow 32 bits", defaults, 2_000_000_000, 2_100_000_000, 80},
	} {
		assert.Equal(t, c.want, ReachedLevel(c.levels, c.used, c.limit), c.name)
	}
}

// Working hours are read on the organization's own clock, whatever zone the
// server is in.
func TestWorkHours_Covers(t *testing.T) {
	weekdays := []int{1, 2, 3, 4, 5}
	shanghai := WorkHours{Timezone: "Asia/Shanghai", Days: weekdays, Start: 9 * 60, End: 18 * 60}
	zone, err := shanghai.Location()
	require.NoError(t, err)
	at := func(value string) time.Time {
		moment, err := time.ParseInLocation("2006-01-02 15:04", value, zone)
		require.NoError(t, err)
		return moment
	}
	// 2026-10-09 is a Friday.
	assert.True(t, shanghai.Covers(at("2026-10-09 09:00"), zone), "the first minute of the day")
	assert.True(t, shanghai.Covers(at("2026-10-09 17:59"), zone), "the last minute of the day")
	assert.False(t, shanghai.Covers(at("2026-10-09 08:59"), zone), "a minute early")
	assert.False(t, shanghai.Covers(at("2026-10-09 18:00"), zone), "the end is not part of the day")
	assert.False(t, shanghai.Covers(at("2026-10-10 10:00"), zone), "Saturday morning")
	assert.False(t, shanghai.Covers(at("2026-10-11 10:00"), zone), "Sunday morning")
	assert.True(t, shanghai.Covers(at("2026-10-12 10:00"), zone), "Monday morning")

	// The same moment is working time in one zone and night in another: Friday
	// 10:00 in Shanghai is Thursday 22:00 in New York.
	newYork := WorkHours{Timezone: "America/New_York", Days: weekdays, Start: 9 * 60, End: 18 * 60}
	newYorkZone, err := newYork.Location()
	require.NoError(t, err)
	assert.True(t, shanghai.Covers(at("2026-10-09 10:00"), zone))
	assert.False(t, newYork.Covers(at("2026-10-09 10:00"), newYorkZone))

	// A zone that is not a whole number of hours away (Nepal, +05:45).
	kathmandu := WorkHours{Timezone: "Asia/Kathmandu", Days: weekdays, Start: 9 * 60, End: 18 * 60}
	kathmanduZone, err := kathmandu.Location()
	require.NoError(t, err)
	utc := time.Date(2026, 10, 9, 3, 14, 0, 0, time.UTC) // 08:59 in Kathmandu
	assert.False(t, kathmandu.Covers(utc, kathmanduZone))
	assert.True(t, kathmandu.Covers(utc.Add(time.Minute), kathmanduZone))

	// A whole day, weekends included.
	always := WorkHours{Timezone: "UTC", Days: []int{0, 1, 2, 3, 4, 5, 6}, Start: 0, End: 24 * 60}
	assert.True(t, always.Covers(time.Date(2026, 10, 11, 23, 59, 0, 0, time.UTC), time.UTC))
}

// PRD §4: the defaults an organization starts with.
func TestDefaultAlertSettings(t *testing.T) {
	defaults := DefaultAlertSettings()
	assert.Equal(t, []int{80, 100}, defaults.WarnAt)
	assert.Equal(t, 5, defaults.SpikeMultiple)
	assert.Equal(t, 50, defaults.OffHoursPercent)
	assert.Equal(t, int(common.QuotaPerUnit), defaults.MinSpend, "one dollar")
	assert.Nil(t, defaults.WorkHours, "the off-hours rule is off until working hours are set")
	normalized, ok := defaults.Normalized()
	require.True(t, ok, "the defaults are themselves acceptable")
	assert.Equal(t, defaults, normalized)
}

func TestAlertSettings_NormalizedPutsListsInOrder(t *testing.T) {
	settings := DefaultAlertSettings()
	settings.WarnAt = []int{100, 50, 80, 50}
	settings.WorkHours = &WorkHours{Timezone: "Asia/Shanghai", Days: []int{5, 1, 3, 1}, Start: 540, End: 1080}
	normalized, ok := settings.Normalized()
	require.True(t, ok)
	assert.Equal(t, []int{50, 80, 100}, normalized.WarnAt)
	assert.Equal(t, []int{1, 3, 5}, normalized.WorkHours.Days)
	assert.Equal(t, []int{100, 50, 80, 50}, settings.WarnAt, "what was passed in is left alone")

	// No warnings at all is a setting, and it is stored as an empty list.
	settings.WarnAt = nil
	normalized, ok = settings.Normalized()
	require.True(t, ok)
	assert.Equal(t, []int{}, normalized.WarnAt)
}

func TestAlertSettings_NormalizedRefusesWhatIsOutOfRange(t *testing.T) {
	hours := func(change func(*WorkHours)) *WorkHours {
		w := &WorkHours{Timezone: "Asia/Shanghai", Days: []int{1, 2, 3, 4, 5}, Start: 540, End: 1080}
		change(w)
		return w
	}
	for name, change := range map[string]func(*AlertSettings){
		"a level of zero":               func(s *AlertSettings) { s.WarnAt = []int{0, 80} },
		"a level above one hundred":     func(s *AlertSettings) { s.WarnAt = []int{80, 101} },
		"six levels":                    func(s *AlertSettings) { s.WarnAt = []int{10, 20, 30, 40, 50, 60} },
		"a spike multiple of one":       func(s *AlertSettings) { s.SpikeMultiple = 1 },
		"an absurd spike multiple":      func(s *AlertSettings) { s.SpikeMultiple = 1001 },
		"an off-hours share of zero":    func(s *AlertSettings) { s.OffHoursPercent = 0 },
		"an absurd off-hours share":     func(s *AlertSettings) { s.OffHoursPercent = 1001 },
		"a negative floor":              func(s *AlertSettings) { s.MinSpend = -1 },
		"working hours without a zone":  func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Timezone = "" }) },
		"the server's own zone":         func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Timezone = "Local" }) },
		"a zone that does not exist":    func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Timezone = "Mars/Olympus_Mons" }) },
		"no working day":                func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Days = nil }) },
		"an eighth day of the week":     func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Days = []int{1, 7} }) },
		"a day before Sunday":           func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Days = []int{-1, 1} }) },
		"a day that ends when it began": func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.End = w.Start }) },
		"a day that ends before it began": func(s *AlertSettings) {
			s.WorkHours = hours(func(w *WorkHours) { w.Start, w.End = 1080, 540 })
		},
		"a start before midnight": func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.Start = -1 }) },
		"an end past midnight":    func(s *AlertSettings) { s.WorkHours = hours(func(w *WorkHours) { w.End = 24*60 + 1 }) },
	} {
		settings := DefaultAlertSettings()
		change(&settings)
		_, ok := settings.Normalized()
		assert.False(t, ok, name)
	}

	// The edges that are still acceptable.
	edges := DefaultAlertSettings()
	edges.WarnAt = []int{1, 100}
	edges.SpikeMultiple = 2
	edges.OffHoursPercent = 1
	edges.MinSpend = 0
	edges.WorkHours = &WorkHours{Timezone: "UTC", Days: []int{0, 6}, Start: 0, End: 24 * 60}
	_, ok := edges.Normalized()
	assert.True(t, ok)
}

// What is stored is read back as it was saved; nothing stored, or something
// that no longer passes the checks, reads as the defaults.
func TestParseAlertSettings(t *testing.T) {
	assert.Equal(t, DefaultAlertSettings(), ParseAlertSettings(""))
	assert.Equal(t, DefaultAlertSettings(), ParseAlertSettings("not json"))
	assert.Equal(t, DefaultAlertSettings(), ParseAlertSettings(`{"warn_at":[80],"spike_multiple":0,"off_hours_percent":50,"min_spend":1}`),
		"settings that are out of range are not half-applied")

	saved := AlertSettings{
		WarnAt: []int{50, 90}, SpikeMultiple: 3, OffHoursPercent: 20, MinSpend: 0,
		WorkHours: &WorkHours{Timezone: "Europe/Berlin", Days: []int{1, 2, 3, 4}, Start: 480, End: 1020},
	}
	stored, err := common.Marshal(saved)
	require.NoError(t, err)
	assert.Equal(t, saved, ParseAlertSettings(string(stored)))

	// An organization that switched its warnings off keeps them off.
	saved.WarnAt = []int{}
	stored, err = common.Marshal(saved)
	require.NoError(t, err)
	assert.Equal(t, saved, ParseAlertSettings(string(stored)))
}

// The two lists of rules name every rule once: a rule in neither would be
// raised and told to nobody.
func TestAlertRules_AreEitherAWarningOrAnAnomaly(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{AlertRuleQuota, AlertRuleMonthly, AlertRuleSpike, AlertRuleOffHours, AlertRuleNewIP},
		append(append([]string{}, WarningRules...), AnomalyRules...))
	for _, rule := range append(append([]string{}, WarningRules...), AnomalyRules...) {
		assert.LessOrEqual(t, len(rule), 16, "%s has to fit org_alerts.rule", rule)
	}
}
