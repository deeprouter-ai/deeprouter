package service

import (
	"testing"
	"time"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P9 (meta-repo docs/enterprise-org-prd.md §6), both acceptance
// items of the card: "owner/admin 可查看全公司用量，按部门/成员/钥匙/模型四个维度分组汇总，
// 支持时间段筛选，以金额计" and "manager 默认只见本部门数据；readonly 可见
// 全公司（只读）；staff 只能查看自己；越权查询返回 403". The 403 itself — the status the
// refusal travels with — is pinned in router/org_usage_test.go.

// The period the reports of these tests are asked for: the first week of
// October 2026, to the second.
var (
	reportStart = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	reportEnd   = time.Date(2026, 10, 7, 23, 59, 59, 0, time.UTC)
)

// day returns noon of a day of that week.
func day(n int) time.Time {
	return time.Date(2026, 10, n, 12, 0, 0, 0, time.UTC)
}

// spending is one line of the usage log: a request that was paid for, or —
// with refund set — money given back for one that failed.
type spending struct {
	at         time.Time
	model      string
	quota      int
	prompt     int
	completion int
	refund     bool
}

// logSpending writes usage log lines the way the gateway does for an
// organization key: stamped with the organization and with the department the
// user sits in at this moment.
func logSpending(t *testing.T, db *gorm.DB, key platformmodel.Token, user platformmodel.User, lines ...spending) {
	t.Helper()
	for _, line := range lines {
		logType := platformmodel.LogTypeConsume
		if line.refund {
			logType = platformmodel.LogTypeRefund
		}
		require.NoError(t, db.Create(&platformmodel.Log{
			UserId: user.Id, Username: user.Username, TokenId: key.Id, TokenName: key.Name,
			OrgId: key.OrgId, DepartmentId: user.DepartmentId, Type: logType, CreatedAt: line.at.Unix(),
			ModelName: line.model, Quota: line.quota, PromptTokens: line.prompt, CompletionTokens: line.completion,
		}).Error)
	}
}

// usageRow is a report row with its two figures.
func usageRow(id int, name string, requests int64, quota int64) UsageRow {
	return UsageRow{Id: id, Name: name, UsedBy: []string{}, UsageFigures: UsageFigures{Requests: requests, Quota: quota}}
}

// usedBy is a key's row: a usageRow that says who used the key.
func usedBy(row UsageRow, names ...string) UsageRow {
	row.UsedBy = names
	return row
}

// usageOver asks for a report over the test period.
func usageOver(t *testing.T, db *gorm.DB, actor *Actor, groupBy string) *UsageReport {
	t.Helper()
	report, err := ReportUsage(db, db, actor, UsageQuery{GroupBy: groupBy, Start: reportStart.Unix(), End: reportEnd.Unix()})
	require.NoError(t, err)
	return report
}

// usageIn is usageOver narrowed to one department.
func usageIn(db *gorm.DB, actor *Actor, groupBy string, departmentID int) (*UsageReport, error) {
	return ReportUsage(db, db, actor, UsageQuery{
		GroupBy: groupBy, Start: reportStart.Unix(), End: reportEnd.Unix(), DepartmentId: departmentID,
	})
}

// company is an organization that has spent something: two members in two
// departments and a service account, each with a key.
type company struct {
	org     testOrg
	owner   *Actor
	general orgmodel.Department
	sales   orgmodel.Department
	product orgmodel.Department
	sam     platformmodel.User // staff in Sales
	pat     platformmodel.User // staff in Product
	bot     platformmodel.User // a service account, in the default department
	samKey  platformmodel.Token
	patKey  platformmodel.Token
	botKey  platformmodel.Token
}

// newCompany founds the organization and writes its week of usage: 600 spent
// in Sales after a refund of 40, 500 in Product and 50 by the service account.
func newCompany(t *testing.T, db *gorm.DB, name string) company {
	t.Helper()
	withUsageLog(t, db)
	org := seedOrg(t, db, name)
	c := company{
		org:     org,
		owner:   actorFor(t, db, org.owner.Id),
		general: defaultDepartment(t, db, org.id),
		sales:   departmentNamed(t, db, org.id, "Sales"),
		product: departmentNamed(t, db, org.id, "Product"),
	}
	staff := presetRoleID(t, db, orgmodel.RoleStaff)
	c.sam = seedMemberIn(t, db, org, "sam-of-"+name, staff, c.sales.Id)
	c.pat = seedMemberIn(t, db, org, "pat-of-"+name, staff, c.product.Id)
	bot, err := CreateServiceAccount(db, c.owner, "CI Pipeline", 0)
	require.NoError(t, err)
	c.bot = reloadUser(t, db, bot.Id)
	c.samKey = seedKey(t, db, org, c.sam.Id, "sam's key")
	c.patKey = seedKey(t, db, org, c.pat.Id, "pat's key")
	c.botKey = seedKey(t, db, org, c.bot.Id, "pipeline key")
	logSpending(t, db, c.samKey, c.sam,
		spending{at: day(2), model: "gpt-4o", quota: 300, prompt: 100, completion: 20},
		spending{at: day(3), model: "gpt-4o-mini", quota: 100, prompt: 50, completion: 10},
		// A video task that was paid for in full and partly given back.
		spending{at: day(4), model: "MiniMax-H3", quota: 240},
		spending{at: day(4).Add(time.Minute), model: "MiniMax-H3", quota: 40, refund: true},
	)
	logSpending(t, db, c.patKey, c.pat, spending{at: day(2), model: "gpt-4o", quota: 500, prompt: 200, completion: 40})
	logSpending(t, db, c.botKey, c.bot, spending{at: day(5), model: "claude-sonnet-5", quota: 50, prompt: 10, completion: 5})
	return c
}

// Acceptance: 按部门/成员/钥匙/模型四个维度分组汇总 … 以金额计. Every grouping is
// the same usage cut another way, so all four add up to one total. The log
// lines carry token counts; the report leaves them out (PRD D44).
func TestReportUsage_SumsByDepartmentMemberKeyAndModel(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		total := UsageFigures{Requests: 5, Quota: 1150}

		byDepartment := usageOver(t, db, c.owner, UsageByDepartment)
		require.Equal(t, UsageByDepartment, byDepartment.GroupBy)
		require.Equal(t, orgmodel.ScopeOrg, byDepartment.Scope)
		require.Equal(t, total, byDepartment.Total)
		require.Equal(t, []UsageRow{
			usageRow(c.sales.Id, "Sales", 3, 600),
			usageRow(c.product.Id, "Product", 1, 500),
			usageRow(c.general.Id, "General", 1, 50),
		}, byDepartment.Rows, "the biggest spender first")

		// Acceptance of P3: a service account's usage 出现在报表中, marked as one.
		pipeline := usageRow(c.bot.Id, "CI Pipeline", 1, 50)
		pipeline.IsService = true
		byMember := usageOver(t, db, c.owner, UsageByMember)
		require.Equal(t, total, byMember.Total)
		require.Equal(t, []UsageRow{
			usageRow(c.sam.Id, "sam-of-Acme", 3, 600),
			usageRow(c.pat.Id, "pat-of-Acme", 1, 500),
			pipeline,
		}, byMember.Rows)

		byKey := usageOver(t, db, c.owner, UsageByKey)
		require.Equal(t, total, byKey.Total)
		require.Equal(t, []UsageRow{
			usedBy(usageRow(c.samKey.Id, "sam's key", 3, 600), "sam-of-Acme"),
			usedBy(usageRow(c.patKey.Id, "pat's key", 1, 500), "pat-of-Acme"),
			usedBy(usageRow(c.botKey.Id, "pipeline key", 1, 50), "CI Pipeline"),
		}, byKey.Rows)

		byModel := usageOver(t, db, c.owner, UsageByModel)
		require.Equal(t, total, byModel.Total)
		require.Equal(t, []UsageRow{
			usageRow(0, "gpt-4o", 2, 800),
			// 240 paid, 40 given back: one request, 200 spent.
			usageRow(0, "MiniMax-H3", 1, 200),
			usageRow(0, "gpt-4o-mini", 1, 100),
			usageRow(0, "claude-sonnet-5", 1, 50),
		}, byModel.Rows)
	})
}

// Acceptance: 支持时间段筛选. Both ends of the period are part of it, either can be
// left open, and nothing is counted that is not this organization's usage.
func TestReportUsage_CountsThePeriodAndTheOrganizationAndNothingElse(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		logSpending(t, db, c.samKey, c.sam,
			spending{at: reportStart.Add(-time.Second), model: "early", quota: 1000},
			spending{at: reportStart, model: "first", quota: 10},
			spending{at: reportEnd, model: "last", quota: 20},
			spending{at: reportEnd.Add(time.Second), model: "late", quota: 2000},
		)
		// Another company's usage, on the same models in the same week.
		other := newCompany(t, db, "Other")
		// The owner's own requests from before the organization's keys were
		// stamped: no organization on the line, so not the company's bill.
		require.NoError(t, db.Create(&platformmodel.Log{
			UserId: c.org.owner.Id, Type: platformmodel.LogTypeConsume, CreatedAt: day(3).Unix(), ModelName: "gpt-4o", Quota: 7000,
		}).Error)
		// Lines that are not usage: a failed request and a top-up.
		for _, logType := range []int{platformmodel.LogTypeError, platformmodel.LogTypeTopup, platformmodel.LogTypeManage, platformmodel.LogTypeSystem} {
			require.NoError(t, db.Create(&platformmodel.Log{
				UserId: c.sam.Id, TokenId: c.samKey.Id, OrgId: c.org.id, DepartmentId: c.sales.Id,
				Type: logType, CreatedAt: day(3).Unix(), ModelName: "gpt-4o", Quota: 9000,
			}).Error)
		}

		quotaOf := func(query UsageQuery) int64 {
			query.GroupBy = UsageByModel
			report, err := ReportUsage(db, db, c.owner, query)
			require.NoError(t, err)
			return report.Total.Quota
		}
		require.EqualValues(t, 1150+10+20, quotaOf(UsageQuery{Start: reportStart.Unix(), End: reportEnd.Unix()}))
		require.EqualValues(t, 1150+10+20+1000, quotaOf(UsageQuery{End: reportEnd.Unix()}), "no start: from the beginning")
		require.EqualValues(t, 1150+10+20+2000, quotaOf(UsageQuery{Start: reportStart.Unix()}), "no end: until now")
		require.EqualValues(t, 1150+10+20+1000+2000, quotaOf(UsageQuery{}))
		require.EqualValues(t, 300+500, quotaOf(UsageQuery{Start: day(2).Unix(), End: day(2).Unix()}), "a period of one second")
		require.Zero(t, quotaOf(UsageQuery{Start: day(6).Unix(), End: reportEnd.Unix() - 1}), "a stretch nobody used anything in")

		// The other company reads its own week and none of this one's.
		theirs := usageOver(t, db, other.owner, UsageByMember)
		require.EqualValues(t, 1150, theirs.Total.Quota)
		require.Len(t, theirs.Rows, 3)
		for _, row := range theirs.Rows {
			require.Contains(t, []int{other.sam.Id, other.pat.Id, other.bot.Id}, row.Id)
		}
		// Nor is a department of one company a filter in the other's report.
		var own []int
		require.NoError(t, db.Model(&orgmodel.Department{}).Where("org_id = ?", c.org.id).Pluck("id", &own).Error)
		offered := usageOver(t, db, c.owner, UsageByMember).Departments
		require.Len(t, offered, len(own))
		for _, department := range offered {
			require.Contains(t, own, department.Id)
		}
		require.Len(t, theirs.Departments, 6)
		for _, department := range theirs.Departments {
			require.NotContains(t, own, department.Id)
		}
	})
}

// Acceptance: manager 默认只见本部门数据；readonly 可见全公司（只读）；staff 只能查看
// 自己. Each kind of member asks for the same report and is given their part.
func TestReportUsage_ShowsEachRoleItsPart(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		c := newCast(t, db, "Acme")
		staff := reloadUser(t, db, c.actors[kindStaff].UserId)
		inProduct := seedMemberIn(t, db, c.org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		bot := reloadUser(t, db, c.actors[kindBot].UserId)
		hrOps := reloadUser(t, db, c.actors[kindHROps].UserId)
		manager := reloadUser(t, db, c.actors[kindManager].UserId)
		// Everyone who spends does so with a key of their own.
		spend := func(user platformmodel.User, quota int) {
			logSpending(t, db, seedKey(t, db, c.org, user.Id, "key of "+user.Username), user, spending{at: day(2), model: "gpt-4o", quota: quota})
		}
		spend(staff, 1)        // in Sales
		spend(manager, 2)      // in Sales
		spend(inProduct, 4)    // in Product
		spend(bot, 8)          // in the default department
		spend(hrOps, 16)       // in the default department
		spend(c.org.owner, 32) // in the default department

		// Whoever reads usage across the organization gets all of it, a
		// read-only member and the finance role — which holds nothing but
		// usage.read — included.
		for _, kind := range []string{kindOwner, kindAdmin, kindReadonly, kindITOps, kindFinance} {
			report := usageOver(t, db, c.actors[kind], UsageByMember)
			require.Equal(t, orgmodel.ScopeOrg, report.Scope, kind)
			require.EqualValues(t, 63, report.Total.Quota, kind)
			require.Len(t, report.Rows, 6, kind)
			require.Len(t, report.Departments, 6, kind)
			require.Equal(t, defaultDepartment(t, db, c.org.id).Id, report.Departments[0].Id, "the default department leads")
		}

		// The manager of Sales gets what was spent in Sales, whatever the
		// grouping.
		for groupBy, rows := range map[string]int{UsageByDepartment: 1, UsageByMember: 2, UsageByKey: 2, UsageByModel: 1} {
			report := usageOver(t, db, c.actors[kindManager], groupBy)
			require.Equal(t, orgmodel.ScopeDept, report.Scope, groupBy)
			require.EqualValues(t, 3, report.Total.Quota, groupBy)
			require.Len(t, report.Rows, rows, groupBy)
			require.Equal(t, []UsageDepartment{{Id: c.sales.Id, Name: "Sales"}}, report.Departments, groupBy)
		}
		// Given Product to manage as well (PRD D28), they get both.
		require.NoError(t, UpdateMember(db, c.actors[kindOwner], manager.Id, MemberPatch{ManagedDepartmentIds: intsPtr(c.product.Id)}))
		wider := usageOver(t, db, actorFor(t, db, manager.Id), UsageByDepartment)
		require.EqualValues(t, 7, wider.Total.Quota)
		require.Equal(t, []UsageRow{usageRow(c.product.Id, "Product", 1, 4), usageRow(c.sales.Id, "Sales", 2, 3)}, wider.Rows)
		require.ElementsMatch(t, []UsageDepartment{{Id: c.sales.Id, Name: "Sales"}, {Id: c.product.Id, Name: "Product"}}, wider.Departments)

		// Everyone else gets what they used themselves — staff, a role that
		// reads members and keys but not usage, a role that manages keys in a
		// department, and a service account were it ever to ask.
		for kind, quota := range map[string]int64{kindStaff: 1, kindHROps: 16, kindKeyDesk: 0, kindBot: 8} {
			actor := c.actors[kind]
			for _, groupBy := range []string{UsageByDepartment, UsageByMember, UsageByKey, UsageByModel} {
				report := usageOver(t, db, actor, groupBy)
				require.Equal(t, orgmodel.ScopeSelf, report.Scope, "%s by %s", kind, groupBy)
				require.Equal(t, quota, report.Total.Quota, "%s by %s", kind, groupBy)
				require.Empty(t, report.Departments, "%s by %s", kind, groupBy)
			}
			byMember := usageOver(t, db, actor, UsageByMember)
			for _, row := range byMember.Rows {
				require.Equal(t, actor.UserId, row.Id, kind)
			}
			if quota == 0 {
				require.Empty(t, byMember.Rows, kind)
			}
		}
	})
}

// Acceptance: 越权查询返回 403 — and 部门作用域硬检查 of P4, as far as reports go.
// Naming a department is asking for that department's usage: outside the
// caller's reach it is refused, never answered with an empty report.
func TestReportUsage_RefusesADepartmentOutOfReach(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		withUsageLog(t, db)
		c := newCast(t, db, "Acme")
		staff := reloadUser(t, db, c.actors[kindStaff].UserId)
		logSpending(t, db, seedKey(t, db, c.org, staff.Id, "sales key"), staff, spending{at: day(2), model: "gpt-4o", quota: 5})
		inProduct := seedMemberIn(t, db, c.org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		logSpending(t, db, seedKey(t, db, c.org, inProduct.Id, "product key"), inProduct, spending{at: day(2), model: "gpt-4o", quota: 7})

		for _, groupBy := range []string{UsageByDepartment, UsageByMember, UsageByKey, UsageByModel} {
			// A role that reads usage everywhere narrows to any department.
			for _, kind := range []string{kindOwner, kindAdmin, kindReadonly, kindFinance} {
				report, err := usageIn(db, c.actors[kind], groupBy, c.product.Id)
				require.NoError(t, err, "%s by %s", kind, groupBy)
				require.EqualValues(t, 7, report.Total.Quota, "%s by %s", kind, groupBy)
				require.Len(t, report.Rows, 1, "%s by %s", kind, groupBy)
			}
			// The manager of Sales narrows to Sales and to nothing else.
			report, err := usageIn(db, c.actors[kindManager], groupBy, c.sales.Id)
			require.NoError(t, err, groupBy)
			require.EqualValues(t, 5, report.Total.Quota, groupBy)
			report, err = usageIn(db, c.actors[kindManager], groupBy, c.product.Id)
			require.ErrorIs(t, err, ErrForbidden, groupBy)
			require.Nil(t, report, groupBy)
			// Whoever only sees their own usage names no department at all,
			// their own included: a department's usage is everybody's in it.
			for _, kind := range []string{kindStaff, kindHROps, kindKeyDesk, kindBot} {
				for _, department := range []int{c.sales.Id, c.product.Id} {
					report, err := usageIn(db, c.actors[kind], groupBy, department)
					require.ErrorIs(t, err, ErrForbidden, "%s by %s", kind, groupBy)
					require.Nil(t, report, "%s by %s", kind, groupBy)
				}
			}
		}

		// A department that is not this organization's has no usage here; the
		// answer is an empty report of the caller's own company.
		other := seedOrg(t, db, "Other")
		theirSales := departmentNamed(t, db, other.id, "Sales")
		theirMember := seedMemberIn(t, db, other, "their-member", presetRoleID(t, db, orgmodel.RoleStaff), theirSales.Id)
		logSpending(t, db, seedKey(t, db, other, theirMember.Id, "their key"), theirMember, spending{at: day(2), model: "gpt-4o", quota: 900})
		report, err := usageIn(db, c.actors[kindOwner], UsageByMember, theirSales.Id)
		require.NoError(t, err)
		require.Empty(t, report.Rows)
		require.Zero(t, report.Total)
		_, err = usageIn(db, c.actors[kindManager], UsageByMember, theirSales.Id)
		require.ErrorIs(t, err, ErrForbidden)
	})
}

// PRD §7.2: a line is stamped with the department of the moment, so moving
// someone never moves what they already spent — neither into the report of
// their new manager nor out of that of the old one.
func TestReportUsage_KeepsUsageWhereItWasSpent(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		ofSales := actorFor(t, db, seedMemberIn(t, db, c.org, "manager-of-sales", managerRole, c.sales.Id).Id)
		ofProduct := actorFor(t, db, seedMemberIn(t, db, c.org, "manager-of-product", managerRole, c.product.Id).Id)

		require.NoError(t, UpdateMember(db, c.owner, c.sam.Id, MemberPatch{DepartmentId: intPtr(c.product.Id)}))
		moved := reloadUser(t, db, c.sam.Id)
		logSpending(t, db, c.samKey, moved, spending{at: day(6), model: "gpt-4o", quota: 9})

		// Sales keeps the 600 sam spent while there; Product has the 9 since.
		require.EqualValues(t, 600, usageOver(t, db, ofSales, UsageByMember).Total.Quota)
		inProduct := usageOver(t, db, ofProduct, UsageByMember)
		require.EqualValues(t, 500+9, inProduct.Total.Quota)
		require.Equal(t, []UsageRow{
			usageRow(c.pat.Id, "pat-of-Acme", 1, 500),
			usageRow(c.sam.Id, "sam-of-Acme", 1, 9),
		}, inProduct.Rows)
		// Whoever sees the whole organization sees one sam, with both.
		everything := usageOver(t, db, c.owner, UsageByMember)
		require.Equal(t, usageRow(c.sam.Id, "sam-of-Acme", 4, 609), everything.Rows[0])
		byDepartment := usageOver(t, db, c.owner, UsageByDepartment)
		require.Equal(t, []UsageRow{
			usageRow(c.sales.Id, "Sales", 3, 600),
			usageRow(c.product.Id, "Product", 2, 509),
			usageRow(c.general.Id, "General", 1, 50),
		}, byDepartment.Rows)
	})
}

// A report covers the past, so it goes on naming a member who was removed, a
// key that was deleted and a department that was dissolved — by the name they
// have now, and marked as gone.
func TestReportUsage_GoesOnNamingWhatIsGone(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", c.sam.Id).Update("display_name", "Sam Seller").Error)
		_, err := RemoveMember(db, c.owner, c.sam.Id)
		require.NoError(t, err)
		require.NoError(t, DeleteKey(db, c.owner, c.patKey.Id))
		_, err = UpdateKey(db, c.owner, c.samKey.Id, KeyPatch{Name: strPtr("spare key")}, fixedModels(testCatalogue))
		require.NoError(t, err)
		require.NoError(t, DeleteDepartment(db, c.owner, c.product.Id))
		require.NoError(t, RenameDepartment(db, c.owner, c.sales.Id, "Field Sales"))

		gone := func(row UsageRow) UsageRow {
			row.Gone = true
			return row
		}
		require.Equal(t, []UsageRow{
			usageRow(c.sales.Id, "Field Sales", 3, 600),
			gone(usageRow(c.product.Id, "Product", 1, 500)),
			usageRow(c.general.Id, "General", 1, 50),
		}, usageOver(t, db, c.owner, UsageByDepartment).Rows)

		byMember := usageOver(t, db, c.owner, UsageByMember).Rows
		require.Equal(t, gone(usageRow(c.sam.Id, "Sam Seller", 3, 600)), byMember[0])
		require.False(t, byMember[1].Gone)

		byKey := usageOver(t, db, c.owner, UsageByKey).Rows
		require.Equal(t, usedBy(usageRow(c.samKey.Id, "spare key", 3, 600), "Sam Seller"), byKey[0],
			"taken back from a removed member, the key itself is still there")
		require.Equal(t, gone(usedBy(usageRow(c.patKey.Id, "pat's key", 1, 500), "pat-of-Acme")), byKey[1])

		// A dissolved department is not one to narrow a report to.
		for _, department := range usageOver(t, db, c.owner, UsageByMember).Departments {
			require.NotEqual(t, c.product.Id, department.Id)
		}
	})
}

// A key that changed hands was used by more than one member. Its row says who,
// the biggest spender first — out of the usage the caller may see, so a
// manager is not told about someone outside their departments.
func TestReportUsage_SaysWhoUsedAKey(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		// sam's key goes on to pat, who spends more with it than sam did.
		logSpending(t, db, c.samKey, c.pat, spending{at: day(6), model: "gpt-4o", quota: 700, prompt: 1, completion: 1})

		byKey := usageOver(t, db, c.owner, UsageByKey).Rows
		require.Equal(t, usedBy(usageRow(c.samKey.Id, "sam's key", 4, 1300), "pat-of-Acme", "sam-of-Acme"), byKey[0])

		ofSales := actorFor(t, db, seedMemberIn(t, db, c.org, "manager-of-sales", presetRoleID(t, db, orgmodel.RoleManager), c.sales.Id).Id)
		require.Equal(t, []UsageRow{usedBy(usageRow(c.samKey.Id, "sam's key", 3, 600), "sam-of-Acme")},
			usageOver(t, db, ofSales, UsageByKey).Rows)
	})
}

func TestReportUsage_RefusesAGroupingOrAPeriodThatMakesNoSense(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCompany(t, db, "Acme")
		for name, query := range map[string]UsageQuery{
			"no grouping":           {},
			"an unknown grouping":   {GroupBy: "team"},
			"a column of the log":   {GroupBy: "token_id"},
			"ends before it starts": {GroupBy: UsageByModel, Start: reportEnd.Unix(), End: reportStart.Unix()},
			"a negative start":      {GroupBy: UsageByModel, Start: -1},
			"a negative end":        {GroupBy: UsageByModel, End: -1},
		} {
			report, err := ReportUsage(db, db, c.owner, query)
			require.ErrorIs(t, err, ErrInvalidUsageQuery, name)
			require.Nil(t, report, name)
		}
	})
}
