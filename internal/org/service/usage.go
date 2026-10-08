package service

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Enterprise Org P9 (meta-repo docs/enterprise-org-prd.md §6): the usage
// report. A usage log line of an organization key already says who used it,
// with which key, on which model and for how much, and carries the
// organization and the department of that moment (P7) — so a report is a sum
// over those lines, read straight from the log with no table of its own
// (PRD §8). What takes care is whose lines: the role decides, and its answer
// is part of the query rather than something a page filters afterwards.

// What a usage report can be grouped by.
const (
	UsageByDepartment = "department"
	UsageByMember     = "member"
	UsageByKey        = "key"
	UsageByModel      = "model"
)

// usageColumns names the usage log column each grouping sums over.
var usageColumns = map[string]string{
	UsageByDepartment: "department_id",
	UsageByMember:     "user_id",
	UsageByKey:        "token_id",
	UsageByModel:      "model_name",
}

// ErrInvalidUsageQuery means a usage report was asked for grouped by something
// it cannot be, or over a period that ends before it starts.
var ErrInvalidUsageQuery = errors.New("a usage report is grouped by department, member, key or model, over a period that starts before it ends")

// UsageQuery is what a usage report is asked for.
type UsageQuery struct {
	GroupBy string
	// Start and End bound the period in Unix seconds, both included. Zero
	// leaves that side open.
	Start int64
	End   int64
	// DepartmentId narrows the report to what was spent in one department.
	// Zero is every department the caller may see.
	DepartmentId int
}

// UsageFigures are what a report measures usage in: money, and the number of
// requests behind it. Tokens are not reported (PRD D44): a company reads its
// usage as what it cost.
type UsageFigures struct {
	Requests int64 `json:"requests"`
	// Quota is what was spent, in quota units, with refunds taken off: a task
	// that failed and was refunded cost the company nothing, and the report
	// has to add up to what left the company wallet.
	Quota int64 `json:"quota"`
}

// add counts more usage into the figures.
func (f *UsageFigures) add(more UsageFigures) {
	f.Requests += more.Requests
	f.Quota += more.Quota
}

// UsageRow is what one department, member, key or model accounts for.
type UsageRow struct {
	// Id is the department, the member or the key. A model has only a name.
	Id   int    `json:"id"`
	Name string `json:"name"`
	// IsService marks a member that is a service account.
	IsService bool `json:"is_service"`
	// Gone marks a department or key deleted since, or a member removed since.
	// What they spent stays in the report.
	Gone bool `json:"gone"`
	// UsedBy names who used a key in the period, the biggest spender first. A
	// key that changed hands has more than one.
	UsedBy []string `json:"used_by"`
	UsageFigures
}

// UsageDepartment is a department a report can be narrowed to.
type UsageDepartment struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// UsageReport is an organization's usage over a period, as far as the caller
// may see it.
type UsageReport struct {
	GroupBy string `json:"group_by"`
	// Scope says how much of the organization the report covers: all of it
	// (ScopeOrg), the departments the caller manages (ScopeDept), or only what
	// the caller used themselves (ScopeSelf).
	Scope string `json:"scope"`
	// Departments are the departments the caller may narrow the report to.
	Departments []UsageDepartment `json:"departments"`
	Total       UsageFigures      `json:"total"`
	// Rows are sorted by what was spent, the most first.
	Rows []UsageRow `json:"rows"`
}

// ReportUsage sums the organization's usage over a period by department,
// member, key or model. How much of it the caller is shown follows their role:
// everything for a role that reads usage across the organization, what was
// spent in the departments they manage for a department-scoped one, and what
// they used themselves for everyone else. Naming a department outside that is
// refused with ErrForbidden — it is never answered with an empty report.
//
// Usage is counted where it was spent: a line belongs to the department its
// user sat in at that moment, so moving someone never moves what they spent.
func ReportUsage(db *gorm.DB, logDB *gorm.DB, actor *Actor, query UsageQuery) (*UsageReport, error) {
	column, known := usageColumns[query.GroupBy]
	if !known || query.Start < 0 || (query.End != 0 && query.End < query.Start) {
		return nil, ErrInvalidUsageQuery
	}
	usage := logDB.Model(&platformmodel.Log{}).Where("org_id = ? AND type IN ?", actor.OrgId,
		[]int{platformmodel.LogTypeConsume, platformmodel.LogTypeRefund})
	if query.Start > 0 {
		usage = usage.Where("created_at >= ?", query.Start)
	}
	if query.End > 0 {
		usage = usage.Where("created_at <= ?", query.End)
	}
	if query.DepartmentId != 0 {
		if err := actor.allow("usage.read", orgmodel.Target{DepartmentId: query.DepartmentId}); err != nil {
			return nil, err
		}
		usage = usage.Where("department_id = ?", query.DepartmentId)
	}

	report := &UsageReport{GroupBy: query.GroupBy, Scope: orgmodel.ScopeOrg, Departments: []UsageDepartment{}}
	everywhere, managed := orgmodel.Reach(actor.Subject, "usage.read")
	switch {
	case everywhere:
	case len(managed) > 0:
		report.Scope = orgmodel.ScopeDept
		if query.DepartmentId == 0 {
			usage = usage.Where("department_id IN ?", managed)
		}
	default:
		// No report is theirs to read. What they used themselves every member
		// may see, whatever their role (PRD §2).
		if err := actor.allow("usage.read", orgmodel.Target{UserId: actor.UserId}); err != nil {
			return nil, err
		}
		report.Scope = orgmodel.ScopeSelf
		usage = usage.Where("user_id = ?", actor.UserId)
	}
	if report.Scope != orgmodel.ScopeSelf {
		departments, err := usageDepartments(db, actor.OrgId, everywhere, managed)
		if err != nil {
			return nil, err
		}
		report.Departments = departments
	}

	// A key is summed per user as well, to say who used it.
	selected, grouped := column+" AS group_id", column
	switch query.GroupBy {
	case UsageByModel:
		selected = column + " AS group_name"
	case UsageByKey:
		selected, grouped = column+" AS group_id, user_id AS used_by", column+", user_id"
	}
	selected += fmt.Sprintf(", SUM(CASE WHEN type = %d THEN 1 ELSE 0 END) AS requests"+
		", COALESCE(SUM(CASE WHEN type = %d THEN -quota ELSE quota END), 0) AS quota",
		platformmodel.LogTypeConsume, platformmodel.LogTypeRefund)
	var sums []struct {
		GroupId   int
		GroupName string
		UsedBy    int
		UsageFigures
	}
	if err := usage.Select(selected).Group(grouped).Scan(&sums).Error; err != nil {
		return nil, err
	}

	// group is what a row stands for: an id, or for a model its name.
	type group struct {
		id   int
		name string
	}
	rows := map[group]*UsageRow{}
	// users lists who used each key, with what each of them spent on it.
	users := map[int][]keyUsage{}
	for _, sum := range sums {
		of := group{sum.GroupId, sum.GroupName}
		row := rows[of]
		if row == nil {
			row = &UsageRow{Id: sum.GroupId, Name: sum.GroupName, UsedBy: []string{}}
			rows[of] = row
		}
		row.add(sum.UsageFigures)
		if query.GroupBy == UsageByKey {
			users[sum.GroupId] = append(users[sum.GroupId], keyUsage{userID: sum.UsedBy, quota: sum.Quota})
		}
		report.Total.add(sum.UsageFigures)
	}
	report.Rows = make([]UsageRow, 0, len(rows))
	for _, row := range rows {
		report.Rows = append(report.Rows, *row)
	}
	slices.SortFunc(report.Rows, func(a, b UsageRow) int {
		return cmp.Or(cmp.Compare(b.Quota, a.Quota), cmp.Compare(b.Requests, a.Requests),
			cmp.Compare(a.Id, b.Id), cmp.Compare(a.Name, b.Name))
	})
	if err := nameUsageRows(db, actor.OrgId, query.GroupBy, report.Rows, users); err != nil {
		return nil, err
	}
	return report, nil
}

// keyUsage is what one member spent with one key.
type keyUsage struct {
	userID int
	quota  int64
}

// usageDepartments returns the departments a report can be narrowed to: all
// that exist for a role that reaches the whole organization, the managed ones
// for a department-scoped role. The default department comes first.
func usageDepartments(db *gorm.DB, orgID int, everywhere bool, managed []int) ([]UsageDepartment, error) {
	query := db.Model(&orgmodel.Department{}).Where("org_id = ?", orgID)
	if !everywhere {
		query = query.Where("id IN ?", managed)
	}
	departments := []UsageDepartment{}
	err := query.Select("id", "name").Order("is_default DESC").Order("id").Scan(&departments).Error
	return departments, err
}

// nameUsageRows fills in what each row of a report is called today, and
// whether it is still there. The log is not asked: it keeps the names of the
// moment, and a department or a key renamed since would show up under two.
func nameUsageRows(db *gorm.DB, orgID int, groupBy string, rows []UsageRow, users map[int][]keyUsage) error {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	switch groupBy {
	case UsageByDepartment:
		var departments []orgmodel.Department
		if err := db.Unscoped().Select("id", "name", "deleted_at").Where("org_id = ?", orgID).Find(&departments).Error; err != nil {
			return err
		}
		byID := make(map[int]orgmodel.Department, len(departments))
		for _, department := range departments {
			byID[department.Id] = department
		}
		for i := range rows {
			rows[i].Name, rows[i].Gone = byID[rows[i].Id].Name, byID[rows[i].Id].DeletedAt.Valid
		}
	case UsageByMember:
		members, err := membersEver(db, orgID, ids)
		if err != nil {
			return err
		}
		for i := range rows {
			member := members[rows[i].Id]
			rows[i].Name, rows[i].IsService, rows[i].Gone = memberName(member), member.IsService, member.DeletedAt.Valid
		}
	case UsageByKey:
		byID := make(map[int]platformmodel.Token, len(ids))
		for chunk := range slices.Chunk(ids, scanChunk) {
			var keys []platformmodel.Token
			if err := db.Unscoped().Select("id", "name", "deleted_at").
				Where("org_id = ? AND id IN ?", orgID, chunk).Find(&keys).Error; err != nil {
				return err
			}
			for _, key := range keys {
				byID[key.Id] = key
			}
		}
		var userIDs []int
		for _, spenders := range users {
			for _, spender := range spenders {
				userIDs = append(userIDs, spender.userID)
			}
		}
		slices.Sort(userIDs)
		members, err := membersEver(db, orgID, slices.Compact(userIDs))
		if err != nil {
			return err
		}
		for i := range rows {
			rows[i].Name, rows[i].Gone = byID[rows[i].Id].Name, byID[rows[i].Id].DeletedAt.Valid
			spenders := users[rows[i].Id]
			slices.SortFunc(spenders, func(a, b keyUsage) int {
				return cmp.Or(cmp.Compare(b.quota, a.quota), cmp.Compare(a.userID, b.userID))
			})
			for _, spender := range spenders {
				rows[i].UsedBy = append(rows[i].UsedBy, memberName(members[spender.userID]))
			}
		}
	}
	return nil
}

// membersEver loads members of an organization by id, the removed ones too: a
// report goes on naming whoever spent, and says when they have left.
func membersEver(db *gorm.DB, orgID int, userIDs []int) (map[int]platformmodel.User, error) {
	members := make(map[int]platformmodel.User, len(userIDs))
	for chunk := range slices.Chunk(userIDs, scanChunk) {
		var users []platformmodel.User
		if err := db.Unscoped().Select("id", "username", "display_name", "is_service", "deleted_at").
			Where("org_id = ? AND id IN ?", orgID, chunk).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			members[user.Id] = user
		}
	}
	return members, nil
}

// memberName returns what to call a member: the display name, or the username
// when they never set one.
func memberName(user platformmodel.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}
