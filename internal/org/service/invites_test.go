package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P3 (meta-repo docs/enterprise-org-prd.md), acceptance item
// "owner/admin 能生成邀请链接/邀请码（指定角色与部门），被邀请人注册后自动加入组织并
// 获得对应角色".

// expireInvite backdates an invite so that it stopped working a second ago.
func expireInvite(t *testing.T, db *gorm.DB, inviteID int) {
	t.Helper()
	require.NoError(t, db.Model(&orgmodel.OrgInvite{}).Where("id = ?", inviteID).
		Update("expires_time", time.Now().Unix()-1).Error)
}

// joinByInvite runs JoinByInviteTx in its own transaction, as sign-up does.
func joinByInvite(db *gorm.DB, userID int, code string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return JoinByInviteTx(tx, userID, code, testIP)
	})
}

func TestCreateInvite(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		admin := actorFor(t, db, seedMember(t, db, acme, "admin", orgmodel.RoleAdmin).Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)

		before := time.Now().Unix()
		invite, err := CreateInvite(db, admin, managerRole, sales.Id)
		require.NoError(t, err)
		require.NotZero(t, invite.Id)
		require.Regexp(t, `^[0-9a-zA-Z]{32}$`, invite.Code)
		require.Equal(t, managerRole, invite.RoleId)
		require.Equal(t, orgmodel.RoleManager, invite.Role)
		require.Equal(t, sales.Id, invite.DepartmentId)
		validFor := invite.ExpiresTime - before
		require.GreaterOrEqual(t, validFor, int64(InviteValidity/time.Second))
		require.LessOrEqual(t, validFor, int64(InviteValidity/time.Second)+5)

		var stored orgmodel.OrgInvite
		require.NoError(t, db.First(&stored, invite.Id).Error)
		require.Equal(t, orgmodel.OrgInvite{
			Id: invite.Id, OrgId: acme.id, Code: invite.Code,
			RoleId: managerRole, DepartmentId: sales.Id, ExpiresTime: invite.ExpiresTime,
		}, stored)

		// No department chosen means the default one.
		unplaced, err := CreateInvite(db, admin, presetRoleID(t, db, orgmodel.RoleStaff), 0)
		require.NoError(t, err)
		require.Equal(t, defaultDepartment(t, db, acme.id).Id, unplaced.DepartmentId)
		require.NotEqual(t, invite.Code, unplaced.Code)
	})
}

// An invite that makes admins is an appointment, so it is the owner's alone;
// and no invite makes owners.
func TestCreateInvite_FollowsTheSameRoleRulesAsAnAppointment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		admin := actorFor(t, db, seedMember(t, db, acme, "admin", orgmodel.RoleAdmin).Id)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		ownerRole := presetRoleID(t, db, orgmodel.RoleOwner)
		theirRole := orgmodel.OrgRole{OrgId: other.id, Name: "Their role", Scope: orgmodel.ScopeOrg}
		require.NoError(t, db.Create(&theirRole).Error)
		theirSales := departmentNamed(t, db, other.id, "Sales")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)

		_, err := CreateInvite(db, admin, adminRole, 0)
		require.ErrorIs(t, err, ErrOwnerOnly)
		_, err = CreateInvite(db, owner, adminRole, 0)
		require.NoError(t, err)

		for _, actor := range []*Actor{owner, admin} {
			_, err := CreateInvite(db, actor, ownerRole, 0)
			require.ErrorIs(t, err, ErrOwnerImmutable)
			_, err = CreateInvite(db, actor, theirRole.Id, 0)
			require.ErrorIs(t, err, ErrRoleNotFound)
			_, err = CreateInvite(db, actor, 424242, 0)
			require.ErrorIs(t, err, ErrRoleNotFound)
			_, err = CreateInvite(db, actor, staffRole, theirSales.Id)
			require.ErrorIs(t, err, ErrDepartmentNotFound)
		}

		var invites int64
		require.NoError(t, db.Model(&orgmodel.OrgInvite{}).Count(&invites).Error)
		require.EqualValues(t, 1, invites, "refused requests issued nothing")
	})
}

// PRD D26: admins sit in the default department, so that is the only place an
// invite can bring one in.
func TestCreateInvite_AnAdminInviteIsForTheDefaultDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")

		_, err := CreateInvite(db, owner, adminRole, sales.Id)
		require.ErrorIs(t, err, ErrAdminDepartment)
		var invites int64
		require.NoError(t, db.Model(&orgmodel.OrgInvite{}).Count(&invites).Error)
		require.Zero(t, invites, "the refused request issued nothing")

		for _, departmentID := range []int{0, general.Id} {
			invite, err := CreateInvite(db, owner, adminRole, departmentID)
			require.NoError(t, err, "department %d", departmentID)
			require.Equal(t, general.Id, invite.DepartmentId)
		}

		// Whoever joins through such a link is an admin in the default department.
		invite, err := CreateInvite(db, owner, adminRole, 0)
		require.NoError(t, err)
		newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
		require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code))
		joined := reloadUser(t, db, newcomer.Id)
		require.Equal(t, adminRole, joined.OrgRoleId)
		require.Equal(t, general.Id, joined.DepartmentId)
	})
}

func TestListInvites_ShowsOnlyTheOrganizationsUsableLinks(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		readonlyRole := presetRoleID(t, db, orgmodel.RoleReadonly)

		first, err := CreateInvite(db, owner, staffRole, 0)
		require.NoError(t, err)
		second, err := CreateInvite(db, owner, readonlyRole, departmentNamed(t, db, acme.id, "Sales").Id)
		require.NoError(t, err)
		expired, err := CreateInvite(db, owner, staffRole, 0)
		require.NoError(t, err)
		expireInvite(t, db, expired.Id)
		_, err = CreateInvite(db, actorFor(t, db, other.owner.Id), staffRole, 0)
		require.NoError(t, err)

		invites, err := ListInvites(db, owner)
		require.NoError(t, err)
		require.Equal(t, []InviteView{*second, *first}, invites, "newest first, without the expired one or anyone else's")
	})
}

func TestRevokeInvite(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		invite, err := CreateInvite(db, owner, staffRole, 0)
		require.NoError(t, err)
		theirs, err := CreateInvite(db, actorFor(t, db, other.owner.Id), staffRole, 0)
		require.NoError(t, err)

		// Another company's link is not there to revoke.
		require.ErrorIs(t, RevokeInvite(db, owner, theirs.Id), ErrInviteNotFound)
		_, err = LookupInvite(db, theirs.Code)
		require.NoError(t, err, "and it keeps working")

		require.NoError(t, RevokeInvite(db, owner, invite.Id))
		_, err = LookupInvite(db, invite.Code)
		require.ErrorIs(t, err, ErrInviteNotFound)
		newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
		require.ErrorIs(t, joinByInvite(db, newcomer.Id, invite.Code), ErrInviteNotFound, "a revoked link admits nobody")
		require.Zero(t, reloadUser(t, db, newcomer.Id).OrgId)

		require.ErrorIs(t, RevokeInvite(db, owner, invite.Id), ErrInviteNotFound, "it cannot be revoked twice")
	})
}

func TestLookupInvite(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		invite, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleManager), sales.Id)
		require.NoError(t, err)

		preview, err := LookupInvite(db, invite.Code)
		require.NoError(t, err)
		require.Equal(t, &InvitePreview{OrgName: "Acme", Role: orgmodel.RoleManager, Department: "Sales"}, preview)

		// It reflects the department as it is now, not as it was when the link was made.
		require.NoError(t, RenameDepartment(db, owner, sales.Id, "Revenue"))
		preview, err = LookupInvite(db, invite.Code)
		require.NoError(t, err)
		require.Equal(t, "Revenue", preview.Department)
		require.NoError(t, DeleteDepartment(db, owner, sales.Id))
		preview, err = LookupInvite(db, invite.Code)
		require.NoError(t, err)
		require.Equal(t, "General", preview.Department)

		for _, code := range []string{"", "no-such-code", invite.Code + "x", invite.Code[:31]} {
			_, err := LookupInvite(db, code)
			require.ErrorIs(t, err, ErrInviteNotFound, "%q", code)
		}
		expireInvite(t, db, invite.Id)
		_, err = LookupInvite(db, invite.Code)
		require.ErrorIs(t, err, ErrInviteNotFound, "an expired link reads like one that never existed")
	})
}

func TestJoinByInviteTx_GivesTheNewcomerTheInvitesRoleAndDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		invite, err := CreateInvite(db, owner, managerRole, sales.Id)
		require.NoError(t, err)
		newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
		bystander := seedUser(t, db, "bystander", common.RoleCommonUser)

		require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code))

		// Exactly the three org columns moved. In particular users.role did
		// not: an invited manager, like everyone, is a common user on the
		// platform (PRD §7.3 red line 1).
		want := newcomer
		want.OrgId = acme.id
		want.OrgRoleId = managerRole
		want.DepartmentId = sales.Id
		require.Equal(t, want, reloadUser(t, db, newcomer.Id))
		require.Equal(t, common.RoleCommonUser, reloadUser(t, db, newcomer.Id).Role)
		require.Equal(t, bystander, reloadUser(t, db, bystander.Id))

		membership, err := GetMembership(db, newcomer.Id)
		require.NoError(t, err)
		require.Equal(t, "Acme", membership.OrgName)
		require.Equal(t, orgmodel.RoleManager, membership.Role)
		require.False(t, membership.IsOwner)
		require.False(t, membership.IsAdmin)
	})
}

// Decided 2026-10-06: an invite link serves a whole team — it keeps working
// until it expires or is revoked.
func TestJoinByInviteTx_OneLinkAdmitsManyPeople(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		invite, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleStaff), 0)
		require.NoError(t, err)

		for _, username := range []string{"one", "two", "three"} {
			newcomer := seedUser(t, db, username, common.RoleCommonUser)
			require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code), username)
			require.Equal(t, acme.id, reloadUser(t, db, newcomer.Id).OrgId)
		}
		members, err := ListMembers(db, owner)
		require.NoError(t, err)
		require.Len(t, members, 4, "the owner and the three who used the link")
		invites, err := ListInvites(db, owner)
		require.NoError(t, err)
		require.Len(t, invites, 1, "and the link is still there")

		expireInvite(t, db, invite.Id)
		late := seedUser(t, db, "late", common.RoleCommonUser)
		require.ErrorIs(t, joinByInvite(db, late.Id, invite.Code), ErrInviteNotFound)
		require.Equal(t, late, reloadUser(t, db, late.Id))
	})
}

// PRD D20: one person, one organization. An invite does not move somebody who
// already belongs to one — their own or another's.
func TestJoinByInviteTx_DoesNotMoveSomeoneWhoAlreadyBelongs(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		invite, err := CreateInvite(db, actorFor(t, db, acme.owner.Id), presetRoleID(t, db, orgmodel.RoleAdmin), 0)
		require.NoError(t, err)
		theirStaff := seedMember(t, db, other, "their-staff", orgmodel.RoleStaff)
		ownStaff := seedMember(t, db, acme, "own-staff", orgmodel.RoleStaff)

		for _, user := range []platformmodel.User{theirStaff, other.owner, ownStaff, acme.owner} {
			before := reloadUser(t, db, user.Id)
			require.ErrorIs(t, joinByInvite(db, user.Id, invite.Code), ErrNotPersonalAccount, user.Username)
			require.Equal(t, before, reloadUser(t, db, user.Id), user.Username)
		}
		require.ErrorIs(t, joinByInvite(db, 424242, invite.Code), ErrNotPersonalAccount)
	})
}

// Enterprise Org P4, acceptance item "持有 member.invite 的部门级角色（如 manager）
// 只能邀请本部门的 staff；越权邀请被拒绝".
func TestCreateInvite_AManagerInvitesStaffIntoTheirDepartmentsOnly(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		custom, err := CreateRole(db, owner, RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)
		manager := actorFor(t, db, seedMemberIn(t, db, acme, "manager", managerRole, sales.Id).Id)

		invite, err := CreateInvite(db, manager, staffRole, sales.Id)
		require.NoError(t, err)
		require.Equal(t, orgmodel.RoleStaff, invite.Role)
		require.Equal(t, sales.Id, invite.DepartmentId)

		// Another department — and "no department", which is the default one,
		// is another department to them.
		for _, departmentID := range []int{product.Id, 0} {
			_, err := CreateInvite(db, manager, staffRole, departmentID)
			require.ErrorIs(t, err, ErrForbidden, "department %d", departmentID)
		}
		// Their own department, but a role that is not theirs to give: nobody
		// gives what they do not have, and a manager holds no role but staff
		// to hand out.
		for name, roleID := range map[string]int{
			"manager":  managerRole,
			"readonly": presetRoleID(t, db, orgmodel.RoleReadonly),
			"custom":   custom.Id,
		} {
			_, err := CreateInvite(db, manager, roleID, sales.Id)
			require.ErrorIs(t, err, ErrForbidden, name)
		}
		_, err = CreateInvite(db, manager, presetRoleID(t, db, orgmodel.RoleAdmin), sales.Id)
		require.ErrorIs(t, err, ErrOwnerOnly)
		_, err = CreateInvite(db, manager, presetRoleID(t, db, orgmodel.RoleOwner), sales.Id)
		require.ErrorIs(t, err, ErrOwnerImmutable)
		_, err = CreateInvite(db, manager, staffRole, departmentNamed(t, db, other.id, "Sales").Id)
		require.ErrorIs(t, err, ErrDepartmentNotFound)

		var invites int64
		require.NoError(t, db.Model(&orgmodel.OrgInvite{}).Where("org_id = ?", acme.id).Count(&invites).Error)
		require.EqualValues(t, 1, invites, "refused requests issued nothing")

		// The scope is the role's, not the primitive's: the same member.invite
		// in a role that reaches the whole organization invites staff anywhere
		// — and still only staff.
		hrRole := seedRole(t, db, acme.id, "HR Ops", orgmodel.ScopeOrg, packPermissions(t, "hr_ops")...)
		hr := actorFor(t, db, seedMember(t, db, acme, "hr", orgmodel.RoleStaff).Id)
		require.NoError(t, UpdateMember(db, owner, hr.UserId, MemberPatch{RoleId: intPtr(hrRole.Id)}))
		hr = actorFor(t, db, hr.UserId)
		for _, departmentID := range []int{sales.Id, product.Id, 0} {
			_, err := CreateInvite(db, hr, staffRole, departmentID)
			require.NoError(t, err, "department %d", departmentID)
		}
		_, err = CreateInvite(db, hr, managerRole, sales.Id)
		require.ErrorIs(t, err, ErrForbidden)
	})
}

// Whoever could have issued a link sees it and may revoke it — and nobody
// else: a link is a way into the organization.
func TestInvites_AreSeenAndRevokedByWhoCouldHaveIssuedThem(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		admin := actorFor(t, db, seedMember(t, db, acme, "admin", orgmodel.RoleAdmin).Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		manager := actorFor(t, db, seedMemberIn(t, db, acme, "manager", managerRole, sales.Id).Id)

		staffToSales, err := CreateInvite(db, owner, staffRole, sales.Id)
		require.NoError(t, err)
		staffToProduct, err := CreateInvite(db, owner, staffRole, product.Id)
		require.NoError(t, err)
		managerToSales, err := CreateInvite(db, admin, managerRole, sales.Id)
		require.NoError(t, err)
		adminLink, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleAdmin), 0)
		require.NoError(t, err)

		seen := func(actor *Actor) []int {
			invites, err := ListInvites(db, actor)
			require.NoError(t, err)
			ids := []int{}
			for _, invite := range invites {
				ids = append(ids, invite.Id)
			}
			return ids
		}
		require.Equal(t, []int{adminLink.Id, managerToSales.Id, staffToProduct.Id, staffToSales.Id}, seen(owner))
		// Appointing admins is the owner's alone, so the link that does it is
		// not an admin's to copy.
		require.Equal(t, []int{managerToSales.Id, staffToProduct.Id, staffToSales.Id}, seen(admin))
		require.Equal(t, []int{staffToSales.Id}, seen(manager))

		require.ErrorIs(t, RevokeInvite(db, manager, staffToProduct.Id), ErrForbidden)
		require.ErrorIs(t, RevokeInvite(db, manager, managerToSales.Id), ErrForbidden)
		require.ErrorIs(t, RevokeInvite(db, manager, adminLink.Id), ErrForbidden)
		require.ErrorIs(t, RevokeInvite(db, admin, adminLink.Id), ErrOwnerOnly)
		require.Len(t, seen(owner), 4, "refused revocations revoked nothing")

		// The manager revokes a link into their department, whoever made it.
		require.NoError(t, RevokeInvite(db, manager, staffToSales.Id))
		require.NoError(t, RevokeInvite(db, owner, adminLink.Id))
		require.Equal(t, []int{managerToSales.Id, staffToProduct.Id}, seen(owner))
	})
}

// PRD D17 and D25: issuing a link, revoking it and every use of it are on
// record — without the code, which is a credential the audit log's readers
// have no claim to.
func TestInvites_AreAuditedWithoutTheirCode(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)

		invite, err := CreateInvite(db, owner, managerRole, sales.Id)
		require.NoError(t, err)
		issued := lastAudit(t, db, acme.id)
		require.Equal(t, orgmodel.AuditMemberInvite, issued.Action)
		require.Equal(t, orgmodel.AuditTargetInvite, issued.TargetType)
		require.Equal(t, invite.Id, issued.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"after":{"role_id":%d,"role":"manager","department_id":%d,"department":"Sales","expires_time":%d}}`,
			managerRole, sales.Id, invite.ExpiresTime), issued.Detail)

		newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
		require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code))
		joined := lastAudit(t, db, acme.id)
		require.Equal(t, orgmodel.AuditMemberJoin, joined.Action)
		require.Equal(t, newcomer.Id, joined.ActorUserId, "the newcomer is who acted")
		require.Equal(t, orgmodel.AuditTargetMember, joined.TargetType)
		require.Equal(t, newcomer.Id, joined.TargetId)
		require.Equal(t, testIP, joined.Ip)
		require.JSONEq(t, fmt.Sprintf(`{"after":{"invite_id":%d,"role_id":%d,"role":"manager","department_id":%d,"department":"Sales"}}`,
			invite.Id, managerRole, sales.Id), joined.Detail)

		require.NoError(t, RevokeInvite(db, owner, invite.Id))
		revoked := lastAudit(t, db, acme.id)
		require.Equal(t, orgmodel.AuditInviteRevoke, revoked.Action)
		require.Equal(t, invite.Id, revoked.TargetId)

		for _, record := range auditRecords(t, db, acme.id) {
			require.NotContains(t, record.Detail, invite.Code, record.Action)
		}

		// A join that fails leaves no record: nobody joined.
		live, err := CreateInvite(db, owner, managerRole, sales.Id)
		require.NoError(t, err)
		before := len(auditRecords(t, db, acme.id))
		late := seedUser(t, db, "late", common.RoleCommonUser)
		require.ErrorIs(t, joinByInvite(db, late.Id, invite.Code), ErrInviteNotFound, "a revoked link")
		require.ErrorIs(t, joinByInvite(db, acme.owner.Id, live.Code), ErrNotPersonalAccount, "someone who already belongs")
		require.Len(t, auditRecords(t, db, acme.id), before)
	})
}
