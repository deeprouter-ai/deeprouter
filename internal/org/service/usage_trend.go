package service

import (
	"cmp"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
)

// Enterprise Org P10 (meta-repo docs/enterprise-org-prd.md §6, D46): the usage
// trend — the usage report's spending drawn over time, one line per department
// or per model; or, narrowed to one member or one key, that one's line (D49).
// It is the same log lines, read through the same reach (usageWithin) and
// measured the same way (usageSpentSQL), cut once more by time; so what a
// trend adds up to is what the report says for that period.
//
// A day is the viewer's day. The log keeps Unix seconds, so the database sums
// by day with nothing but arithmetic every dialect has — the seconds moved by
// the zone's offset, less their remainder by a day — and weeks, months and
// years are folded out of days here, on the calendar.

// What a usage trend can cut time into.
const (
	TrendByDay   = "day"
	TrendByWeek  = "week"
	TrendByMonth = "month"
	TrendByYear  = "year"
)

// trendSeriesShown is how many lines a trend draws. Whatever spent less is
// summed into one more line, so the chart stays readable and still adds up.
const trendSeriesShown = 8

// secondsPerDay is the length of the days the database sums by.
const secondsPerDay = 24 * 60 * 60

// trendBuckets maps each way of cutting time to the first day of the bucket a
// day falls in. Days are dates at midnight UTC: calendar dates, not moments.
var trendBuckets = map[string]func(day time.Time) time.Time{
	TrendByDay: func(day time.Time) time.Time { return day },
	// A week starts on Monday.
	TrendByWeek:  func(day time.Time) time.Time { return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7)) },
	TrendByMonth: func(day time.Time) time.Time { return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC) },
	TrendByYear:  func(day time.Time) time.Time { return time.Date(day.Year(), 1, 1, 0, 0, 0, 0, time.UTC) },
}

// trendSteps maps each way of cutting time to the bucket after a given one.
var trendSteps = map[string]func(bucket time.Time) time.Time{
	TrendByDay:   func(bucket time.Time) time.Time { return bucket.AddDate(0, 0, 1) },
	TrendByWeek:  func(bucket time.Time) time.Time { return bucket.AddDate(0, 0, 7) },
	TrendByMonth: func(bucket time.Time) time.Time { return bucket.AddDate(0, 1, 0) },
	TrendByYear:  func(bucket time.Time) time.Time { return bucket.AddDate(1, 0, 0) },
}

// UsageTrendQuery is what a usage trend is asked for: the question a usage
// report takes, and how to cut time.
type UsageTrendQuery struct {
	UsageQuery
	// Bucket is the stretch of time each point stands for: a TrendBy… value.
	Bucket string
	// Timezone is the IANA name of the zone whose calendar the buckets
	// follow: the viewer's, so that a day in the trend is a day to them.
	Timezone string
}

// UsageTrendSeries is one line of a trend: what a department, a model, a member
// or a key spent, bucket by bucket.
type UsageTrendSeries struct {
	// Id is the department, the member or the key. A model has only a name.
	// Each is named as it is called today, a dissolved or deleted one too.
	Id   int    `json:"id"`
	Name string `json:"name"`
	// Other marks the line that stands for everything beyond the ones drawn.
	Other bool `json:"other"`
	// Quota is what the line spent over the whole period, refunds taken off.
	Quota int64 `json:"quota"`
	// Points are what it spent in each bucket, in the order of the trend's
	// Buckets. One can be negative: a refund counts where it was given.
	Points []int64 `json:"points"`
}

// UsageTrend is an organization's spending over a period, bucket by bucket,
// as far as the caller may see it.
type UsageTrend struct {
	GroupBy string `json:"group_by"`
	Bucket  string `json:"bucket"`
	// Scope says how much of the organization the trend covers, as a usage
	// report does.
	Scope string `json:"scope"`
	// Buckets name each stretch of time by its first day on the viewer's
	// calendar, as "2026-10-05": the oldest first, none in between left out.
	// They start where the first usage of the period is and run to its end.
	Buckets []string `json:"buckets"`
	// Total is everything spent in the period, which is what the series add
	// up to and what the usage report says for the same question.
	Total int64 `json:"total"`
	// Series are sorted by what they spent, the most first; the one that
	// stands for the rest comes last.
	Series []UsageTrendSeries `json:"series"`
}

// zoneStretch is a stretch of time over which a zone keeps one offset from
// UTC: between two changes of its clocks.
type zoneStretch struct {
	// from and until bound the stretch in Unix seconds, both included.
	from, until int64
	// offset is how many seconds the zone is ahead of UTC throughout.
	offset int
}

// zoneStretches cuts a period wherever the zone's clocks are put forward or
// back, so that each part can be summed by day with a single offset.
func zoneStretches(zone *time.Location, from int64, until int64) []zoneStretch {
	var stretches []zoneStretch
	for at := from; at <= until; {
		moment := time.Unix(at, 0).In(zone)
		_, offset := moment.Zone()
		last := until
		if _, changes := moment.ZoneBounds(); !changes.IsZero() && changes.Unix() <= until {
			last = changes.Unix() - 1
		}
		stretches = append(stretches, zoneStretch{from: at, until: last, offset: offset})
		at = last + 1
	}
	return stretches
}

// trendGroup is what a line of a trend stands for: a department, a member or a
// key by its id, or a model by its name.
type trendGroup struct {
	id   int
	name string
}

// TrendUsage cuts what the organization spent over a period into buckets of
// time — days, weeks, months or years on the viewer's calendar — with one
// series per department, model, member or key. The caller is shown exactly
// the usage a report would show them, and naming a department outside their
// reach is refused with ErrForbidden. The biggest spenders get a series each
// and the rest share one, so the series always add up to Total.
func TrendUsage(db *gorm.DB, logDB *gorm.DB, actor *Actor, query UsageTrendQuery, now time.Time) (*UsageTrend, error) {
	column, drawn := usageColumns[query.GroupBy]
	bucketOf, cut := trendBuckets[query.Bucket]
	// "Local" and the empty name are refused: they would mean wherever the
	// server happens to run, which is nobody's calendar.
	zone, err := time.LoadLocation(query.Timezone)
	if !drawn || !cut || err != nil || query.Timezone == "" || query.Timezone == "Local" {
		return nil, ErrInvalidUsageQuery
	}
	reach, err := usageWithin(logDB, actor, query.UsageQuery)
	if err != nil {
		return nil, err
	}
	trend := &UsageTrend{GroupBy: query.GroupBy, Bucket: query.Bucket, Scope: reach.scope,
		Buckets: []string{}, Series: []UsageTrendSeries{}}
	// Each query below starts from the same lines.
	lines := reach.lines.Session(&gorm.Session{})

	var first sql.NullInt64
	if err := lines.Select("MIN(created_at)").Row().Scan(&first); err != nil {
		return nil, err
	}
	if !first.Valid {
		return trend, nil
	}
	// The trend runs to the end of the period, or to now when the period is
	// open or ends later: nothing has been spent in the future.
	until := now.Unix()
	if query.End != 0 && query.End < until {
		until = query.End
	}
	until = max(until, first.Int64)

	selected, grouped := column+" AS group_id", column
	if query.GroupBy == UsageByModel {
		selected = column + " AS group_name"
	}
	spent := map[trendGroup]map[time.Time]int64{}
	totals := map[trendGroup]int64{}
	var oldest, newest time.Time
	stretches := zoneStretches(zone, first.Int64, until)
	for i, stretch := range stretches {
		// The start of the viewer's day each line falls in, in seconds moved
		// by the zone's offset. Written out, not bound, so that the grouped
		// expression is the selected one to every database.
		day := fmt.Sprintf("(created_at + %d) - (created_at + %d) %% %d", stretch.offset, stretch.offset, secondsPerDay)
		within := lines.Where("created_at >= ?", stretch.from)
		// The last stretch ends where the period does — the lines are bound
		// by that already — so that it sums whatever the report sums.
		if i < len(stretches)-1 {
			within = within.Where("created_at <= ?", stretch.until)
		}
		var sums []struct {
			GroupId   int
			GroupName string
			DayStart  int64
			Quota     int64
		}
		if err := within.Select(selected + ", " + day + " AS day_start, " + usageSpentSQL).
			Group(grouped + ", " + day).Scan(&sums).Error; err != nil {
			return nil, err
		}
		for _, sum := range sums {
			of := trendGroup{sum.GroupId, sum.GroupName}
			bucket := bucketOf(time.Unix(sum.DayStart, 0).UTC())
			if spent[of] == nil {
				spent[of] = map[time.Time]int64{}
			}
			spent[of][bucket] += sum.Quota
			totals[of] += sum.Quota
			trend.Total += sum.Quota
			if oldest.IsZero() || bucket.Before(oldest) {
				oldest = bucket
			}
			if bucket.After(newest) {
				newest = bucket
			}
		}
	}
	if oldest.IsZero() {
		return trend, nil
	}
	// It runs there through the days since the last usage: a line that fell
	// to nothing has to be seen falling.
	end := time.Unix(until, 0).In(zone)
	if last := bucketOf(time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)); last.After(newest) {
		newest = last
	}
	var buckets []time.Time
	for bucket := oldest; !bucket.After(newest); bucket = trendSteps[query.Bucket](bucket) {
		buckets = append(buckets, bucket)
		trend.Buckets = append(trend.Buckets, bucket.Format(time.DateOnly))
	}

	groups := make([]trendGroup, 0, len(totals))
	for of := range totals {
		groups = append(groups, of)
	}
	slices.SortFunc(groups, func(a, b trendGroup) int {
		return cmp.Or(cmp.Compare(totals[b], totals[a]), cmp.Compare(a.id, b.id), cmp.Compare(a.name, b.name))
	})
	shown := min(len(groups), trendSeriesShown)
	named := make([]UsageRow, shown)
	for i, of := range groups[:shown] {
		named[i] = UsageRow{Id: of.id, Name: of.name}
	}
	if err := nameUsageRows(db, actor.OrgId, query.GroupBy, named, nil); err != nil {
		return nil, err
	}
	for i, of := range groups[:shown] {
		series := UsageTrendSeries{Id: of.id, Name: named[i].Name, Quota: totals[of], Points: make([]int64, len(buckets))}
		for at, bucket := range buckets {
			series.Points[at] = spent[of][bucket]
		}
		trend.Series = append(trend.Series, series)
	}
	if len(groups) > shown {
		rest := UsageTrendSeries{Other: true, Points: make([]int64, len(buckets))}
		for _, of := range groups[shown:] {
			rest.Quota += totals[of]
			for at, bucket := range buckets {
				rest.Points[at] += spent[of][bucket]
			}
		}
		trend.Series = append(trend.Series, rest)
	}
	return trend, nil
}
