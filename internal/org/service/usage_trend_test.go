package service

import (
	"fmt"
	"testing"
	"time"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P10 (meta-repo docs/enterprise-org-prd.md §6, D46), both
// acceptance items of the card: "owner/admin 可查看用量随时间变化的折线图，按部门或模型
// 分线，粒度可选日 / 周 / 月 / 年，支持时间段筛选，以金额计" and "趋势图的可见范围与报表一致：
// manager 只见所管部门，readonly 可见全公司，staff 不可见；越权查询返回 403". What the page
// draws is this answer; the 403 itself is pinned in router/org_usage_test.go.

// afterTheWeek is a "now" well past the week the reports of these tests cover.
var afterTheWeek = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// trendOf asks for a trend, failing the test when it is refused.
func trendOf(t *testing.T, db *gorm.DB, actor *Actor, query UsageTrendQuery, now time.Time) *UsageTrend {
	t.Helper()
	trend, err := TrendUsage(db, db, actor, query, now)
	require.NoError(t, err)
	return trend
}

// weekBy is a trend over the test week, day by day on a UTC calendar.
func weekBy(groupBy string) UsageTrendQuery {
	return UsageTrendQuery{
		UsageQuery: UsageQuery{GroupBy: groupBy, Start: reportStart.Unix(), End: reportEnd.Unix()},
		Bucket:     TrendByDay, Timezone: "UTC",
	}
}

// line is a series of a trend that stands for one department or model.
func line(id int, name string, quota int64, points ...int64) UsageTrendSeries {
	return UsageTrendSeries{Id: id, Name: name, Quota: quota, Points: points}
}

// Acceptance: 按部门或模型分线 … 支持时间段筛选，以金额计. A trend is the report's rows
// drawn over time: it starts on the day of the first usage, runs to the end of
// the period through the days nothing was spent, and adds up to what the
// report says for the same question.
func TestTrendUsage_DrawsDepartmentsAndModelsDayByDay(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		// Money given back on a later day than it was paid.
		logSpending(t, db, c.samKey, c.sam, spending{at: day(6), model: "MiniMax-H3", quota: 30, refund: true})
		// Usage around the period, and another company's on the same days.
		logSpending(t, db, c.samKey, c.sam,
			spending{at: reportStart.Add(-time.Hour), model: "gpt-4o", quota: 9000},
			spending{at: reportEnd.Add(time.Hour), model: "gpt-4o", quota: 7000})
		newCompany(t, db, "Other")
		days := []string{"2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07"}

		byDepartment := trendOf(t, db, c.owner, weekBy(UsageByDepartment), afterTheWeek)
		require.Equal(t, &UsageTrend{
			GroupBy: UsageByDepartment, Bucket: TrendByDay, Scope: orgmodel.ScopeOrg, Buckets: days, Total: 1120,
			Series: []UsageTrendSeries{
				// 240 paid and 40 given back on the 4th; 30 more given back on
				// the 6th, where the line dips below nothing.
				line(c.sales.Id, "Sales", 570, 300, 100, 200, 0, -30, 0),
				line(c.product.Id, "Product", 500, 500, 0, 0, 0, 0, 0),
				line(c.general.Id, "General", 50, 0, 0, 0, 50, 0, 0),
			},
		}, byDepartment, "the biggest spender first")

		byModel := trendOf(t, db, c.owner, weekBy(UsageByModel), afterTheWeek)
		require.Equal(t, days, byModel.Buckets)
		require.Equal(t, []UsageTrendSeries{
			line(0, "gpt-4o", 800, 800, 0, 0, 0, 0, 0),
			line(0, "MiniMax-H3", 170, 0, 0, 200, 0, -30, 0),
			line(0, "gpt-4o-mini", 100, 0, 100, 0, 0, 0, 0),
			line(0, "claude-sonnet-5", 50, 0, 0, 0, 50, 0, 0),
		}, byModel.Series)

		// Whichever way it is cut, it is the report's total.
		report := usageOver(t, db, c.owner, UsageByDepartment)
		require.EqualValues(t, 1120, report.Total.Quota)
		require.Equal(t, report.Total.Quota, byDepartment.Total)
		require.Equal(t, report.Total.Quota, byModel.Total)

		// One department of the owner's choosing.
		inSales := weekBy(UsageByModel)
		inSales.DepartmentId = c.sales.Id
		require.EqualValues(t, 570, trendOf(t, db, c.owner, inSales, afterTheWeek).Total)

		// A period left open runs from the first usage there is to today — and
		// so does one that ends later than today: nothing is spent tomorrow.
		today := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
		open := UsageTrendQuery{UsageQuery: UsageQuery{GroupBy: UsageByDepartment}, Bucket: TrendByDay, Timezone: "UTC"}
		untilNow := trendOf(t, db, c.owner, open, today)
		require.Equal(t, []string{"2026-09-30", "2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05",
			"2026-10-06", "2026-10-07", "2026-10-08"}, untilNow.Buckets)
		require.EqualValues(t, 1120+9000+7000, untilNow.Total)
		open.End = today.AddDate(10, 0, 0).Unix()
		require.Equal(t, untilNow, trendOf(t, db, c.owner, open, today))
		// Asked by a clock that runs behind the one that stamped the last
		// lines, it still adds up to what the report says: all of them.
		open.End = 0
		behind := trendOf(t, db, c.owner, open, day(3))
		require.Equal(t, untilNow.Total, behind.Total)
		require.Equal(t, untilNow.Buckets, behind.Buckets)

		// A stretch nobody used anything in has nothing to draw.
		quiet := weekBy(UsageByModel)
		quiet.Start, quiet.End = day(6).Add(time.Hour).Unix(), reportEnd.Unix()
		require.Equal(t, &UsageTrend{GroupBy: UsageByModel, Bucket: TrendByDay, Scope: orgmodel.ScopeOrg,
			Buckets: []string{}, Series: []UsageTrendSeries{}}, trendOf(t, db, c.owner, quiet, afterTheWeek))
	})
}

// Acceptance: 粒度可选日 / 周 / 月 / 年. The database sums by day; weeks — from
// Monday — months and years are days folded on the calendar.
func TestTrendUsage_FoldsDaysIntoWeeksMonthsAndYears(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		noon := func(year int, month time.Month, date int) time.Time {
			return time.Date(year, month, date, 12, 0, 0, 0, time.UTC)
		}
		logSpending(t, db, c.patKey, c.pat,
			spending{at: noon(2026, 9, 28), model: "long-run", quota: 1}, // a Monday
			spending{at: noon(2026, 10, 4), model: "long-run", quota: 2}, // the Sunday of that week
			spending{at: noon(2026, 10, 5), model: "long-run", quota: 4}, // the Monday after
			spending{at: noon(2026, 11, 30), model: "long-run", quota: 8},
			spending{at: noon(2026, 12, 31), model: "long-run", quota: 16},
			spending{at: noon(2027, 1, 1), model: "long-run", quota: 32}, // a Friday
		)
		// The trend of that one model, from its first day to Sunday 3 January.
		pointsBy := func(bucket string) ([]string, []int64) {
			trend := trendOf(t, db, c.owner, UsageTrendQuery{
				UsageQuery: UsageQuery{GroupBy: UsageByModel, Start: noon(2026, 9, 28).Unix(), End: noon(2027, 1, 3).Unix(), DepartmentId: c.product.Id},
				Bucket:     bucket, Timezone: "UTC",
			}, noon(2027, 3, 1))
			require.Equal(t, bucket, trend.Bucket)
			// pat's own week of the fixture is on another model.
			for _, series := range trend.Series {
				if series.Name == "long-run" {
					require.EqualValues(t, 63, series.Quota, bucket)
					return trend.Buckets, series.Points
				}
			}
			t.Fatalf("no series for the model by %s", bucket)
			return nil, nil
		}

		days, byDay := pointsBy(TrendByDay)
		require.Len(t, days, 98)
		require.Equal(t, "2026-09-28", days[0])
		require.Equal(t, "2027-01-03", days[97])
		require.EqualValues(t, 2, byDay[6])
		require.EqualValues(t, 32, byDay[95])

		weeks, byWeek := pointsBy(TrendByWeek)
		require.Equal(t, []string{"2026-09-28", "2026-10-05", "2026-10-12", "2026-10-19", "2026-10-26", "2026-11-02", "2026-11-09",
			"2026-11-16", "2026-11-23", "2026-11-30", "2026-12-07", "2026-12-14", "2026-12-21", "2026-12-28"}, weeks)
		require.Equal(t, []int64{3, 4, 0, 0, 0, 0, 0, 0, 0, 8, 0, 0, 0, 48}, byWeek,
			"Sunday closes a week, Monday opens the next; the week of the new year spans both years")

		months, byMonth := pointsBy(TrendByMonth)
		require.Equal(t, []string{"2026-09-01", "2026-10-01", "2026-11-01", "2026-12-01", "2027-01-01"}, months)
		require.Equal(t, []int64{1, 6, 8, 16, 32}, byMonth)

		years, byYear := pointsBy(TrendByYear)
		require.Equal(t, []string{"2026-01-01", "2027-01-01"}, years)
		require.Equal(t, []int64{31, 32}, byYear)
	})
}

// A day in a trend is the viewer's day, not the server's: the same four lines
// fall on different days in Shanghai, in New York and in Sydney — where the
// clocks went forward in the middle of them, and the Sunday had 23 hours.
func TestTrendUsage_ADayIsTheViewersDay(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		org := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, org.owner.Id)
		key := seedKey(t, db, org, org.owner.Id, "the owner's key")
		utc := func(date int, hour int, minute int) time.Time {
			return time.Date(2026, 10, date, hour, minute, 0, 0, time.UTC)
		}
		logSpending(t, db, key, org.owner,
			spending{at: utc(2, 23, 30), model: "gpt-4o", quota: 1},
			spending{at: utc(3, 13, 30), model: "gpt-4o", quota: 2},
			// Sydney went from UTC+10 to UTC+11 at 16:00 UTC on the 3rd.
			spending{at: utc(4, 13, 30), model: "gpt-4o", quota: 4},
			spending{at: utc(4, 18, 0), model: "gpt-4o", quota: 8},
		)
		now := utc(6, 0, 0)
		for zone, want := range map[string]struct {
			days   []string
			points []int64
		}{
			"UTC":              {[]string{"2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06"}, []int64{1, 2, 12, 0, 0}},
			"Asia/Shanghai":    {[]string{"2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06"}, []int64{3, 4, 8, 0}},
			"America/New_York": {[]string{"2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05"}, []int64{1, 2, 12, 0}},
			"Australia/Sydney": {[]string{"2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06"}, []int64{3, 0, 12, 0}},
		} {
			trend := trendOf(t, db, owner, UsageTrendQuery{UsageQuery: UsageQuery{GroupBy: UsageByModel}, Bucket: TrendByDay, Timezone: zone}, now)
			require.Equal(t, want.days, trend.Buckets, zone)
			require.Len(t, trend.Series, 1, zone)
			require.Equal(t, want.points, trend.Series[0].Points, zone)

			// Each point is what a report says for that day of the viewer's:
			// the period a page asks for, from midnight to the last second.
			place, err := time.LoadLocation(zone)
			require.NoError(t, err)
			for i, name := range trend.Buckets {
				date, err := time.ParseInLocation(time.DateOnly, name, place)
				require.NoError(t, err)
				report, err := ReportUsage(db, db, owner, UsageQuery{
					GroupBy: UsageByModel, Start: date.Unix(), End: date.AddDate(0, 0, 1).Unix() - 1,
				})
				require.NoError(t, err)
				require.Equal(t, report.Total.Quota, trend.Series[0].Points[i], "%s on %s", zone, name)
			}
		}
	})
}

// Acceptance: 趋势图的可见范围与报表一致：manager 只见所管部门，readonly 可见全公司 — and
// whoever reads no report is answered with their own usage, as the report
// answers them. Every kind of member asks for both trends and is shown exactly
// what the report shows them.
func TestTrendUsage_ShowsEachRoleWhatTheReportShowsThem(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		c := newCast(t, db, "Acme")
		staff := reloadUser(t, db, c.actors[kindStaff].UserId)
		manager := reloadUser(t, db, c.actors[kindManager].UserId)
		inProduct := seedMemberIn(t, db, c.org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		spend := func(user platformmodel.User, model string, quota int) {
			logSpending(t, db, seedKey(t, db, c.org, user.Id, "key of "+user.Username), user, spending{at: day(2), model: model, quota: quota})
		}
		spend(staff, "gpt-4o", 1)                                       // in Sales
		spend(manager, "claude-sonnet-5", 2)                            // in Sales
		spend(inProduct, "gpt-4o", 4)                                   // in Product
		spend(reloadUser(t, db, c.actors[kindBot].UserId), "gpt-4o", 8) // in the default department
		spend(reloadUser(t, db, c.actors[kindHROps].UserId), "o-1", 16) // in the default department
		spend(reloadUser(t, db, c.org.owner.Id), "claude-sonnet-5", 32) // in the default department

		for _, kind := range allKinds {
			for _, groupBy := range []string{UsageByDepartment, UsageByModel} {
				report := usageOver(t, db, c.actors[kind], groupBy)
				trend := trendOf(t, db, c.actors[kind], weekBy(groupBy), afterTheWeek)
				require.Equal(t, report.Scope, trend.Scope, "%s by %s", kind, groupBy)
				require.Equal(t, report.Total.Quota, trend.Total, "%s by %s", kind, groupBy)
				// Fewer groups than lines: each row of the report is a series.
				require.Len(t, trend.Series, len(report.Rows), "%s by %s", kind, groupBy)
				for i, row := range report.Rows {
					require.Equal(t, line(row.Id, row.Name, row.Quota, row.Quota, 0, 0, 0, 0, 0), trend.Series[i], "%s by %s", kind, groupBy)
				}
			}
		}
		// What that comes to, spelled out for the roles the card names.
		quotaOf := func(kind string) int64 {
			return trendOf(t, db, c.actors[kind], weekBy(UsageByDepartment), afterTheWeek).Total
		}
		for _, kind := range []string{kindOwner, kindAdmin, kindReadonly, kindITOps, kindFinance} {
			require.EqualValues(t, 63, quotaOf(kind), kind)
		}
		require.Equal(t, []UsageTrendSeries{line(c.sales.Id, "Sales", 3, 3, 0, 0, 0, 0, 0)},
			trendOf(t, db, c.actors[kindManager], weekBy(UsageByDepartment), afterTheWeek).Series, "the manager of Sales")
		for kind, own := range map[string]int64{kindStaff: 1, kindHROps: 16, kindKeyDesk: 0, kindBot: 8} {
			require.Equal(t, own, quotaOf(kind), kind)
		}
	})
}

// Acceptance: 越权查询返回 403. Naming a department is asking for that department's
// trend: outside the caller's reach it is refused, never drawn empty.
func TestTrendUsage_RefusesADepartmentOutOfReach(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		c := newCast(t, db, "Acme")
		staff := reloadUser(t, db, c.actors[kindStaff].UserId)
		logSpending(t, db, seedKey(t, db, c.org, staff.Id, "sales key"), staff, spending{at: day(2), model: "gpt-4o", quota: 5})
		inProduct := seedMemberIn(t, db, c.org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		logSpending(t, db, seedKey(t, db, c.org, inProduct.Id, "product key"), inProduct, spending{at: day(2), model: "gpt-4o", quota: 7})
		in := func(kind string, groupBy string, departmentID int) (*UsageTrend, error) {
			query := weekBy(groupBy)
			query.DepartmentId = departmentID
			return TrendUsage(db, db, c.actors[kind], query, afterTheWeek)
		}

		for _, groupBy := range []string{UsageByDepartment, UsageByModel} {
			for _, kind := range []string{kindOwner, kindAdmin, kindReadonly, kindFinance} {
				trend, err := in(kind, groupBy, c.product.Id)
				require.NoError(t, err, "%s by %s", kind, groupBy)
				require.EqualValues(t, 7, trend.Total, "%s by %s", kind, groupBy)
			}
			trend, err := in(kindManager, groupBy, c.sales.Id)
			require.NoError(t, err, groupBy)
			require.EqualValues(t, 5, trend.Total, groupBy)
			trend, err = in(kindManager, groupBy, c.product.Id)
			require.ErrorIs(t, err, ErrForbidden, groupBy)
			require.Nil(t, trend, groupBy)
			for _, kind := range []string{kindStaff, kindHROps, kindKeyDesk, kindBot} {
				for _, department := range []int{c.sales.Id, c.product.Id} {
					trend, err := in(kind, groupBy, department)
					require.ErrorIs(t, err, ErrForbidden, "%s by %s", kind, groupBy)
					require.Nil(t, trend, "%s by %s", kind, groupBy)
				}
			}
		}
	})
}

// PRD D49: a row of the report by member or by key opens as a trend of its own
// — the lines that row was summed from, drawn over time, so it adds up to the
// row. With no member or key named, every row is a line, as by department.
func TestTrendUsage_DrawsOneMemberOrOneKeyFromItsRow(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")

		byMember := trendOf(t, db, c.owner, weekBy(UsageByMember), afterTheWeek)
		require.Equal(t, []UsageTrendSeries{
			line(c.sam.Id, "sam-of-Acme", 600, 300, 100, 200, 0, 0, 0),
			line(c.pat.Id, "pat-of-Acme", 500, 500, 0, 0, 0, 0, 0),
			line(c.bot.Id, "CI Pipeline", 50, 0, 0, 0, 50, 0, 0),
		}, byMember.Series, "every member a line, the biggest spender first")
		byKey := trendOf(t, db, c.owner, weekBy(UsageByKey), afterTheWeek)
		require.Equal(t, []UsageTrendSeries{
			line(c.samKey.Id, "sam's key", 600, 300, 100, 200, 0, 0, 0),
			line(c.patKey.Id, "pat's key", 500, 500, 0, 0, 0, 0, 0),
			line(c.botKey.Id, "pipeline key", 50, 0, 0, 0, 50, 0, 0),
		}, byKey.Series)
		require.EqualValues(t, 1150, byMember.Total)
		require.Equal(t, byMember.Total, byKey.Total)

		// One member: their line alone, from their first usage on, and it is
		// their row of the report.
		sam := weekBy(UsageByMember)
		sam.UserId = c.sam.Id
		samsLine := trendOf(t, db, c.owner, sam, afterTheWeek)
		require.Equal(t, &UsageTrend{
			GroupBy: UsageByMember, Bucket: TrendByDay, Scope: orgmodel.ScopeOrg, Total: 600,
			Buckets: []string{"2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07"},
			Series:  []UsageTrendSeries{line(c.sam.Id, "sam-of-Acme", 600, 300, 100, 200, 0, 0, 0)},
		}, samsLine)
		require.Equal(t, usageOver(t, db, c.owner, UsageByMember).Rows[0].Quota, samsLine.Total)
		// One key.
		pipeline := weekBy(UsageByKey)
		pipeline.TokenId = c.botKey.Id
		require.Equal(t, &UsageTrend{
			GroupBy: UsageByKey, Bucket: TrendByDay, Scope: orgmodel.ScopeOrg, Total: 50,
			Buckets: []string{"2026-10-05", "2026-10-06", "2026-10-07"},
			Series:  []UsageTrendSeries{line(c.botKey.Id, "pipeline key", 50, 50, 0, 0)},
		}, trendOf(t, db, c.owner, pipeline, afterTheWeek))

		// The report takes the same narrowing: one member's usage by model is
		// what their line is made of.
		samByModel, err := ReportUsage(db, db, c.owner, UsageQuery{
			GroupBy: UsageByModel, Start: reportStart.Unix(), End: reportEnd.Unix(), UserId: c.sam.Id,
		})
		require.NoError(t, err)
		require.Equal(t, []UsageRow{
			usageRow(0, "gpt-4o", 1, 300), usageRow(0, "MiniMax-H3", 1, 200), usageRow(0, "gpt-4o-mini", 1, 100),
		}, samByModel.Rows)
		require.EqualValues(t, 600, samByModel.Total.Quota)

		// Someone who spent nothing in the period has nothing to draw.
		nobody := weekBy(UsageByMember)
		nobody.UserId = c.org.owner.Id
		require.Equal(t, &UsageTrend{GroupBy: UsageByMember, Bucket: TrendByDay, Scope: orgmodel.ScopeOrg,
			Buckets: []string{}, Series: []UsageTrendSeries{}}, trendOf(t, db, c.owner, nobody, afterTheWeek))
	})
}

// The line of one member is made of the lines of theirs the caller may see: a
// manager asking about someone who spent in two departments is drawn the part
// in theirs, and about someone outside them nothing at all — not a 403, since
// no row of theirs ever named that member.
func TestTrendUsage_DrawsOneMemberWithinTheCallersReach(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		c := newCast(t, db, "Acme")
		staff := reloadUser(t, db, c.actors[kindStaff].UserId)
		key := seedKey(t, db, c.org, staff.Id, "staff key")
		logSpending(t, db, key, staff, spending{at: day(2), model: "gpt-4o", quota: 1})
		// Moved to Product, they go on spending — there.
		staff.DepartmentId = c.product.Id
		logSpending(t, db, key, staff, spending{at: day(3), model: "gpt-4o", quota: 2})
		inProduct := seedMemberIn(t, db, c.org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		logSpending(t, db, seedKey(t, db, c.org, inProduct.Id, "product key"), inProduct, spending{at: day(2), model: "gpt-4o", quota: 4})

		of := func(kind string, userID int) *UsageTrend {
			query := weekBy(UsageByMember)
			query.UserId = userID
			return trendOf(t, db, c.actors[kind], query, afterTheWeek)
		}
		bothDays := []UsageTrendSeries{line(staff.Id, staff.Username, 3, 1, 2, 0, 0, 0, 0)}
		require.Equal(t, bothDays, of(kindOwner, staff.Id).Series)
		// The manager of Sales is drawn the day spent in Sales, and nothing of
		// a member who was never there.
		manager := of(kindManager, staff.Id)
		require.Equal(t, orgmodel.ScopeDept, manager.Scope)
		require.Equal(t, []UsageTrendSeries{line(staff.Id, staff.Username, 1, 1, 0, 0, 0, 0, 0)}, manager.Series)
		require.Empty(t, of(kindManager, inProduct.Id).Series)
		// Whoever reads no report is drawn their own line, and nobody else's.
		require.Equal(t, bothDays, of(kindStaff, staff.Id).Series)
		require.Empty(t, of(kindStaff, inProduct.Id).Series)

		// The same by key.
		byKey := weekBy(UsageByKey)
		byKey.TokenId = key.Id
		require.EqualValues(t, 3, trendOf(t, db, c.actors[kindOwner], byKey, afterTheWeek).Total)
		require.EqualValues(t, 1, trendOf(t, db, c.actors[kindManager], byKey, afterTheWeek).Total)
		require.EqualValues(t, 3, trendOf(t, db, c.actors[kindStaff], byKey, afterTheWeek).Total)
		require.EqualValues(t, 2, trendOf(t, db, c.actors[kindReadonly], UsageTrendQuery{
			UsageQuery: UsageQuery{GroupBy: UsageByKey, Start: reportStart.Unix(), End: reportEnd.Unix(), TokenId: key.Id, DepartmentId: c.product.Id},
			Bucket:     TrendByDay, Timezone: "UTC",
		}, afterTheWeek).Total, "narrowed to a department as well: what the key spent there")
	})
}

// A chart of forty lines reads as none. The eight that spent the most get a
// line each and the rest share one, so the lines still add up to the total.
func TestTrendUsage_DrawsTheBiggestAndSumsTheRest(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		// Ten models on two days, each spending its number on the first day
		// and ten times that on the second.
		spendOn := func(models int) UsageTrendQuery {
			for n := 1; n <= models; n++ {
				name := fmt.Sprintf("model-%02d", n)
				logSpending(t, db, c.patKey, c.pat,
					spending{at: day(10), model: name, quota: n}, spending{at: day(11), model: name, quota: 10 * n})
			}
			return UsageTrendQuery{
				UsageQuery: UsageQuery{GroupBy: UsageByModel, Start: day(10).Unix(), End: day(11).Unix()},
				Bucket:     TrendByDay, Timezone: "UTC",
			}
		}
		exactly := trendOf(t, db, c.owner, spendOn(trendSeriesShown), afterTheWeek)
		require.Len(t, exactly.Series, trendSeriesShown)
		for _, series := range exactly.Series {
			require.False(t, series.Other, "eight groups are eight lines, with nothing left over")
		}

		// The same again doubles every model, and adds a ninth and a tenth.
		trend := trendOf(t, db, c.owner, spendOn(10), afterTheWeek)
		require.Equal(t, []string{"2026-10-10", "2026-10-11"}, trend.Buckets)
		require.Len(t, trend.Series, trendSeriesShown+1)
		// model-08 spent 2 × 8 × 11, the most; model-09 and model-10 only once,
		// which puts model-10 level with model-05 — and after it, by name.
		require.Equal(t, line(0, "model-08", 176, 16, 160), trend.Series[0])
		require.Equal(t, line(0, "model-05", 110, 10, 100), trend.Series[3])
		require.Equal(t, line(0, "model-10", 110, 10, 100), trend.Series[4])
		require.Equal(t, line(0, "model-03", 66, 6, 60), trend.Series[7])
		require.Equal(t, UsageTrendSeries{Other: true, Quota: 22 + 44, Points: []int64{2 + 4, 20 + 40}}, trend.Series[8],
			"the two that spent the least, summed")
		var drawn int64
		for _, series := range trend.Series {
			drawn += series.Quota
		}
		require.Equal(t, trend.Total, drawn)
		require.Equal(t, usageTotal(t, db, c.owner, trend), trend.Total)
	})
}

// usageTotal is what the report says for the period a trend of two days covers.
func usageTotal(t *testing.T, db *gorm.DB, actor *Actor, trend *UsageTrend) int64 {
	t.Helper()
	report, err := ReportUsage(db, db, actor, UsageQuery{GroupBy: trend.GroupBy, Start: day(10).Unix(), End: day(11).Unix()})
	require.NoError(t, err)
	return report.Total.Quota
}

// A trend covers the past, so it goes on drawing a department that was
// dissolved, and calls every department what it is called today — the log
// keeps the names of the moment, and a renamed one would be drawn twice.
func TestTrendUsage_GoesOnNamingWhatIsGone(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		require.NoError(t, DeleteDepartment(db, c.owner, c.product.Id))
		require.NoError(t, RenameDepartment(db, c.owner, c.sales.Id, "Field Sales"))

		series := trendOf(t, db, c.owner, weekBy(UsageByDepartment), afterTheWeek).Series
		require.Len(t, series, 3)
		require.Equal(t, line(c.sales.Id, "Field Sales", 600, 300, 100, 200, 0, 0, 0), series[0])
		require.Equal(t, line(c.product.Id, "Product", 500, 500, 0, 0, 0, 0, 0), series[1])
	})
}

func TestTrendUsage_RefusesAQuestionThatMakesNoSense(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		asked := func(change func(*UsageTrendQuery)) UsageTrendQuery {
			query := weekBy(UsageByModel)
			change(&query)
			return query
		}
		for name, query := range map[string]UsageTrendQuery{
			"no grouping":            asked(func(q *UsageTrendQuery) { q.GroupBy = "" }),
			"a column of the log":    asked(func(q *UsageTrendQuery) { q.GroupBy = "model_name" }),
			"no bucket":              asked(func(q *UsageTrendQuery) { q.Bucket = "" }),
			"by the hour":            asked(func(q *UsageTrendQuery) { q.Bucket = "hour" }),
			"no time zone":           asked(func(q *UsageTrendQuery) { q.Timezone = "" }),
			"the server's time zone": asked(func(q *UsageTrendQuery) { q.Timezone = "Local" }),
			"an unknown time zone":   asked(func(q *UsageTrendQuery) { q.Timezone = "Mars/Olympus_Mons" }),
			"an offset, not a zone":  asked(func(q *UsageTrendQuery) { q.Timezone = "+08:00" }),
			"ends before it starts":  asked(func(q *UsageTrendQuery) { q.Start, q.End = reportEnd.Unix(), reportStart.Unix() }),
			"a negative start":       asked(func(q *UsageTrendQuery) { q.Start = -1 }),
		} {
			trend, err := TrendUsage(db, db, c.owner, query, afterTheWeek)
			require.ErrorIs(t, err, ErrInvalidUsageQuery, name)
			require.Nil(t, trend, name)
		}
	})
}
