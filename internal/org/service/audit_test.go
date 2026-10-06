package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md), acceptance item
// "组织内的管理操作全部写入审计日志 … 记录操作者、对象、时间与 IP；持有 audit.read 的
// 角色可以查看，readonly 默认拥有". That every write is recorded is pinned in
// gate_test.go; this file is about what a record says and who reads it.

// auditOf returns the newest audit record of an organization with the given
// action.
func auditOf(t *testing.T, db *gorm.DB, orgID int, action string) orgmodel.OrgAuditLog {
	t.Helper()
	var record orgmodel.OrgAuditLog
	require.NoError(t, db.Where("org_id = ? AND action = ?", orgID, action).Order("id DESC").First(&record).Error, action)
	return record
}

// A record says what the thing was before and what it became, with the names
// it had at the time: it must still read right after a rename or a deletion.
func TestAudit_RecordsWhatChangedWithTheNamesOfTheTime(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		member := seedMemberIn(t, db, acme, "member", staffRole, sales.Id)
		colleague := seedMemberIn(t, db, acme, "colleague", staffRole, sales.Id)

		// --- departments ---------------------------------------------------
		created, err := CreateDepartment(db, owner, "Research")
		require.NoError(t, err)
		record := auditOf(t, db, acme.id, orgmodel.AuditDepartmentCreate)
		require.Equal(t, orgmodel.AuditTargetDepartment, record.TargetType)
		require.Equal(t, created.Id, record.TargetId)
		require.JSONEq(t, `{"after":{"name":"Research"}}`, record.Detail)

		require.NoError(t, RenameDepartment(db, owner, created.Id, "Labs"))
		record = auditOf(t, db, acme.id, orgmodel.AuditDepartmentRename)
		require.Equal(t, created.Id, record.TargetId)
		require.JSONEq(t, `{"before":{"name":"Research"},"after":{"name":"Labs"}}`, record.Detail)

		require.NoError(t, DeleteDepartment(db, owner, sales.Id))
		record = auditOf(t, db, acme.id, orgmodel.AuditDepartmentDelete)
		require.Equal(t, sales.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"name":"Sales","member_ids":[%d,%d]}}`, member.Id, colleague.Id), record.Detail,
			"a deletion names who it moved to the default department")
		require.NoError(t, DeleteDepartment(db, owner, created.Id))
		require.JSONEq(t, `{"before":{"name":"Labs"}}`, auditOf(t, db, acme.id, orgmodel.AuditDepartmentDelete).Detail)

		// --- a member ------------------------------------------------------
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(product.Id)}))
		record = auditOf(t, db, acme.id, orgmodel.AuditMemberMove)
		require.Equal(t, orgmodel.AuditTargetMember, record.TargetType)
		require.Equal(t, member.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"department_id":%d,"department":"General"},"after":{"department_id":%d,"department":"Product"}}`,
			general.Id, product.Id), record.Detail)

		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(managerRole), ManagedDepartmentIds: intsPtr(general.Id)}))
		record = auditOf(t, db, acme.id, orgmodel.AuditRoleAssign)
		require.Equal(t, member.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"role_id":%d,"role":"staff"},"after":{"role_id":%d,"role":"manager"}}`, staffRole, managerRole), record.Detail)
		record = auditOf(t, db, acme.id, orgmodel.AuditMemberManages)
		require.Equal(t, member.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"department_ids":[]},"after":{"department_ids":[%d,%d]}}`, product.Id, general.Id), record.Detail)

		// Appointing an admin moves them to the default department (PRD D26):
		// one request, two things done, two records.
		before := len(auditRecords(t, db, acme.id))
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(adminRole)}))
		records := auditRecords(t, db, acme.id)
		require.Len(t, records, before+2)
		require.Equal(t, orgmodel.AuditRoleAssign, records[before].Action)
		require.Equal(t, orgmodel.AuditMemberMove, records[before+1].Action)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"department_id":%d,"department":"Product"},"after":{"department_id":%d,"department":"General"}}`,
			product.Id, general.Id), records[before+1].Detail)

		// A request that changes nothing records nothing.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(adminRole), DepartmentId: intPtr(general.Id), ManagedDepartmentIds: intsPtr()}))
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{}))
		require.Len(t, auditRecords(t, db, acme.id), before+2)

		// --- a role and a service account -----------------------------------
		role, err := CreateRole(db, owner, RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)
		record = auditOf(t, db, acme.id, orgmodel.AuditRoleCreate)
		require.Equal(t, orgmodel.AuditTargetRole, record.TargetType)
		require.Equal(t, role.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"after":{"id":%d,"name":"Key Desk","scope":"org","permissions":["key.read"],"powers":[],"is_preset":false}}`, role.Id), record.Detail)

		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read", "key.freeze"}})
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{
			"before":{"id":%[1]d,"name":"Key Desk","scope":"org","permissions":["key.read"],"powers":[],"is_preset":false},
			"after":{"id":%[1]d,"name":"Key Desk","scope":"org","permissions":["key.read","key.freeze"],"powers":[],"is_preset":false}
		}`, role.Id), auditOf(t, db, acme.id, orgmodel.AuditRoleUpdate).Detail)

		bot, err := CreateServiceAccount(db, owner, "CI Pipeline", 0)
		require.NoError(t, err)
		record = auditOf(t, db, acme.id, orgmodel.AuditServiceAccountCreate)
		require.Equal(t, orgmodel.AuditTargetMember, record.TargetType)
		require.Equal(t, bot.Id, record.TargetId)
		var detail struct {
			After MemberView `json:"after"`
		}
		require.NoError(t, common.Unmarshal([]byte(record.Detail), &detail))
		require.Equal(t, *bot, detail.After)

		// Every record carries who, from where and when.
		for _, record := range auditRecords(t, db, acme.id) {
			require.Equal(t, acme.owner.Id, record.ActorUserId, record.Action)
			require.Equal(t, testIP, record.Ip, record.Action)
			require.NotZero(t, record.CreatedTime, record.Action)
		}
	})
}

// A change and its record are one transaction: if the change does not stand,
// the log does not claim it happened.
func TestAudit_ARecordRollsBackWithItsChange(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)
		sales := departmentNamed(t, db, acme.id, "Sales")

		// The role half of this request is fine and is recorded first; the
		// department half then fails. Neither may survive.
		err := UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, orgmodel.RoleManager)), DepartmentId: intPtr(424242)})
		require.ErrorIs(t, err, ErrDepartmentNotFound)
		require.Equal(t, member, reloadUser(t, db, member.Id))
		require.Empty(t, auditRecords(t, db, acme.id))

		// The same when a caller's own transaction fails after the record.
		later := errors.New("a later step failed")
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := UpdateMember(tx, owner, member.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}); err != nil {
				return err
			}
			return later
		})
		require.ErrorIs(t, err, later)
		require.Empty(t, auditRecords(t, db, acme.id))
	})
}

func TestListAuditLogs(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", admin.Id).Update("display_name", "Ada Admin").Error)

		first, err := CreateDepartment(db, owner, "One")
		require.NoError(t, err)
		_, err = CreateDepartment(db, actorFor(t, db, admin.Id), "Two")
		require.NoError(t, err)
		require.NoError(t, RenameDepartment(db, owner, first.Id, "Uno"))

		// Newest first, with the page asked for and the total of all pages.
		page, total, err := ListAuditLogs(db, owner, 0, 2)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		require.Len(t, page, 2)
		require.Equal(t, orgmodel.AuditDepartmentRename, page[0].Action)
		require.Equal(t, orgmodel.AuditDepartmentCreate, page[1].Action)
		require.Greater(t, page[0].Id, page[1].Id)

		// A record names its actor: the display name, or the username of
		// someone who never set one.
		require.Equal(t, acme.owner.Id, page[0].ActorUserId)
		require.Equal(t, "owner-of-Acme", page[0].Actor)
		require.Equal(t, admin.Id, page[1].ActorUserId)
		require.Equal(t, "Ada Admin", page[1].Actor)
		require.Equal(t, testIP, page[1].Ip)
		require.Equal(t, orgmodel.AuditTargetDepartment, page[1].TargetType)
		require.NotZero(t, page[1].TargetId)
		require.NotZero(t, page[1].CreatedTime)

		// The detail travels as JSON, not as a string of JSON.
		encoded, err := common.Marshal(page[0])
		require.NoError(t, err)
		require.Contains(t, string(encoded), `"detail":{"before":{"name":"One"},"after":{"name":"Uno"}}`)

		rest, total, err := ListAuditLogs(db, owner, 2, 2)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		require.Len(t, rest, 1)
		require.Equal(t, first.Id, rest[0].TargetId)
		beyond, _, err := ListAuditLogs(db, owner, 10, 2)
		require.NoError(t, err)
		require.Empty(t, beyond)

		// An account deleted since still has a name in the log.
		require.NoError(t, db.Delete(&platformmodel.User{}, admin.Id).Error)
		page, _, err = ListAuditLogs(db, owner, 0, 10)
		require.NoError(t, err)
		require.Equal(t, "Ada Admin", page[1].Actor)
	})
}

// Acceptance: 持有 audit.read 的角色可以查看，readonly 默认拥有 — and it is the
// reading that needs the permission: nothing here can change a record.
func TestListAuditLogs_TakesAuditReadAcrossTheWholeOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		_, err := CreateDepartment(db, owner, "Research")
		require.NoError(t, err)

		auditor, err := CreateRole(db, owner, RoleInput{Name: "Auditor", Scope: orgmodel.ScopeOrg, Permissions: []string{"audit.read"}})
		require.NoError(t, err)
		for kind, roleID := range map[string]int{
			"readonly": presetRoleID(t, db, orgmodel.RoleReadonly),
			"admin":    presetRoleID(t, db, orgmodel.RoleAdmin),
			"auditor":  auditor.Id,
		} {
			reader := actorFor(t, db, seedMemberIn(t, db, acme, kind, roleID, defaultDepartment(t, db, acme.id).Id).Id)
			records, total, err := ListAuditLogs(db, reader, 0, 10)
			require.NoError(t, err, kind)
			require.EqualValues(t, 2, total, kind)
			require.Len(t, records, 2, kind)
		}

		for kind, roleID := range map[string]int{
			"staff":   presetRoleID(t, db, orgmodel.RoleStaff),
			"manager": presetRoleID(t, db, orgmodel.RoleManager),
			"it-ops":  seedRole(t, db, acme.id, "IT Ops", orgmodel.ScopeOrg, packPermissions(t, "it_ops")...).Id,
			// A role written into the table by hand with audit.read and
			// department scope — CreateRole refuses the combination — still
			// reads nothing: the log is the whole organization's.
			"forged": seedRole(t, db, acme.id, "Forged", orgmodel.ScopeDept, "audit.read").Id,
		} {
			reader := actorFor(t, db, seedMemberIn(t, db, acme, kind, roleID, sales.Id).Id)
			records, total, err := ListAuditLogs(db, reader, 0, 10)
			require.ErrorIs(t, err, ErrForbidden, kind)
			require.Empty(t, records, kind)
			require.Zero(t, total, kind)
		}
	})
}
