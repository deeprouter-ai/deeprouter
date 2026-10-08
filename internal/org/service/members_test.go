package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P3 (meta-repo docs/enterprise-org-prd.md): members, the rules
// around admins and the owner, and service accounts.

// roleOf returns the name of the role a user holds in their organization.
func roleOf(t *testing.T, db *gorm.DB, userID int) string {
	t.Helper()
	membership, err := GetMembership(db, userID)
	require.NoError(t, err)
	require.NotNil(t, membership)
	return membership.Role
}

func TestLoadActor(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)
		staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)
		personal := seedUser(t, db, "personal", common.RoleCommonUser)

		manager := seedMemberIn(t, db, acme, "manager", presetRoleID(t, db, orgmodel.RoleManager), departmentNamed(t, db, acme.id, "Sales").Id)

		// An actor is what the permission engine judges: the role's primitives
		// and scope as they are in the database right now, and no request
		// address until a handler supplies one.
		for _, tc := range []struct {
			userID int
			want   orgmodel.Subject
		}{
			{acme.owner.Id, orgmodel.Subject{IsOwner: true, Scope: orgmodel.ScopeOrg, Permissions: orgmodel.Primitives, Departments: []int{}}},
			{admin.Id, orgmodel.Subject{IsAdmin: true, Scope: orgmodel.ScopeOrg, Permissions: orgmodel.Primitives, Departments: []int{}}},
			{staff.Id, orgmodel.Subject{Scope: orgmodel.ScopeSelf, Permissions: []string{}, Departments: []int{}}},
			{manager.Id, orgmodel.Subject{Scope: orgmodel.ScopeDept, Departments: []int{manager.DepartmentId},
				Permissions: []string{"key.read", "key.assign", "member.read", "member.invite", "usage.read", "alert.read"}}},
		} {
			actor, err := LoadActor(db, tc.userID)
			require.NoError(t, err)
			tc.want.UserId = tc.userID
			require.Equal(t, &Actor{Subject: tc.want, OrgId: acme.id}, actor)
		}

		_, err := LoadActor(db, personal.Id)
		require.ErrorIs(t, err, ErrNotMember)
	})
}

// Acceptance: 修改角色权限即时生效，无需重新登录. Nothing about a member is
// cached: the next time they are loaded, they are what the database says.
func TestLoadActor_SeesAChangeToTheRoleAtOnce(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		role, err := CreateRole(db, owner, RoleInput{Name: "Helper", Scope: orgmodel.ScopeOrg, Permissions: []string{"usage.read"}})
		require.NoError(t, err)
		helper := seedMemberIn(t, db, acme, "helper", role.Id, sales.Id)

		_, err = ListMembers(db, actorFor(t, db, helper.Id))
		require.ErrorIs(t, err, ErrForbidden, "usage.read does not show members")

		// The role gains a permission …
		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Helper", Scope: orgmodel.ScopeOrg, Permissions: []string{"usage.read", "member.read"}})
		require.NoError(t, err)
		members, err := ListMembers(db, actorFor(t, db, helper.Id))
		require.NoError(t, err)
		require.Len(t, members, 2, "and its holder sees the whole organization on their next request")

		// … is narrowed to departments …
		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Helper", Scope: orgmodel.ScopeDept, Permissions: []string{"member.read"}})
		require.NoError(t, err)
		members, err = ListMembers(db, actorFor(t, db, helper.Id))
		require.NoError(t, err)
		require.Len(t, members, 1, "then only their own department")
		require.Equal(t, helper.Id, members[0].Id)

		// … and loses it again.
		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Helper", Scope: orgmodel.ScopeOrg, Permissions: []string{"usage.read"}})
		require.NoError(t, err)
		_, err = ListMembers(db, actorFor(t, db, helper.Id))
		require.ErrorIs(t, err, ErrForbidden)

		// The same holds for the member's own role: given another, they are it.
		require.NoError(t, UpdateMember(db, owner, helper.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, orgmodel.RoleReadonly))}))
		members, err = ListMembers(db, actorFor(t, db, helper.Id))
		require.NoError(t, err)
		require.Len(t, members, 2)
	})
}

// A custom role may be called anything, "admin" included. Only the platform
// preset makes its holder an admin.
func TestLoadActor_ACustomRoleNamedAdminIsNotTheAdminPreset(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		impostor := seedMember(t, db, acme, "impostor", orgmodel.RoleStaff)
		lookalike := orgmodel.OrgRole{OrgId: acme.id, Name: orgmodel.RoleAdmin, Scope: orgmodel.ScopeOrg}
		require.NoError(t, db.Create(&lookalike).Error)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", impostor.Id).
			Update("role_id", lookalike.Id).Error)

		actor := actorFor(t, db, impostor.Id)
		require.False(t, actor.IsAdmin)
		_, err := ListMembers(db, actor)
		require.ErrorIs(t, err, ErrForbidden)
	})
}

func TestListRoles_OffersThePresetsAndTheOrganizationsOwn(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		require.NoError(t, db.Create(&orgmodel.OrgRole{OrgId: acme.id, Name: "IT Ops", Scope: orgmodel.ScopeOrg, Permissions: "key.read,key.create"}).Error)
		require.NoError(t, db.Create(&orgmodel.OrgRole{OrgId: other.id, Name: "Their secret role", Scope: orgmodel.ScopeOrg, Permissions: "key.delete"}).Error)

		roles, err := ListRoles(db, actorFor(t, db, acme.owner.Id))
		require.NoError(t, err)
		var names []string
		for _, role := range roles {
			names = append(names, role.Name)
		}
		require.Equal(t, []string{
			orgmodel.RoleOwner, orgmodel.RoleAdmin, orgmodel.RoleManager, orgmodel.RoleStaff, orgmodel.RoleReadonly,
			"IT Ops",
		}, names)
		require.True(t, roles[0].IsPreset)
		require.Equal(t, orgmodel.Primitives, roles[0].Permissions)
		require.Equal(t, []string{}, roles[3].Permissions, "staff holds no primitives, and that is an empty list, not null")
		require.Equal(t, orgmodel.ScopeSelf, roles[3].Scope)
		require.False(t, roles[5].IsPreset)
		require.Equal(t, []string{"key.read", "key.create"}, roles[5].Permissions)
	})
}

func TestListMembers(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)
		staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", staff.Id).
			Updates(map[string]any{"display_name": "Staff Person", "email": "staff@acme.test"}).Error)
		bot, err := CreateServiceAccount(db, owner, "CI Pipeline", 0)
		require.NoError(t, err)
		// Neither a personal account nor another company's people belong in the list.
		seedUser(t, db, "personal", common.RoleCommonUser)
		other := seedOrg(t, db, "Other")
		seedMember(t, db, other, "stranger", orgmodel.RoleAdmin)
		general := defaultDepartment(t, db, acme.id).Id

		members, err := ListMembers(db, owner)
		require.NoError(t, err)
		none := []int{} // nobody here has a department-scoped role
		require.Equal(t, []MemberView{
			{Id: acme.owner.Id, Username: "owner-of-Acme", RoleId: presetRoleID(t, db, orgmodel.RoleOwner), Role: orgmodel.RoleOwner, DepartmentId: general, IsOwner: true, ManagedDepartmentIds: none},
			{Id: admin.Id, Username: "admin", RoleId: presetRoleID(t, db, orgmodel.RoleAdmin), Role: orgmodel.RoleAdmin, DepartmentId: general, ManagedDepartmentIds: none},
			{Id: staff.Id, Username: "staff", DisplayName: "Staff Person", Email: "staff@acme.test", RoleId: presetRoleID(t, db, orgmodel.RoleStaff), Role: orgmodel.RoleStaff, DepartmentId: general, ManagedDepartmentIds: none},
			{Id: bot.Id, Username: bot.Username, DisplayName: "CI Pipeline", RoleId: presetRoleID(t, db, orgmodel.RoleStaff), Role: orgmodel.RoleStaff, DepartmentId: general, IsService: true, ManagedDepartmentIds: none},
		}, members)

		// An admin sees the same list.
		asAdmin, err := ListMembers(db, actorFor(t, db, admin.Id))
		require.NoError(t, err)
		require.Equal(t, members, asAdmin)
	})
}

// Acceptance: owner 能任免 admin（admin 可多个）.
func TestUpdateMember_TheOwnerAppointsAndDismissesAdmins(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		first := seedMember(t, db, acme, "first", orgmodel.RoleStaff)
		second := seedMember(t, db, acme, "second", orgmodel.RoleStaff)

		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(adminRole)}))
		require.NoError(t, UpdateMember(db, owner, second.Id, MemberPatch{RoleId: intPtr(adminRole)}))
		require.Equal(t, orgmodel.RoleAdmin, roleOf(t, db, first.Id))
		require.Equal(t, orgmodel.RoleAdmin, roleOf(t, db, second.Id))
		require.True(t, actorFor(t, db, first.Id).IsAdmin, "the appointment works from the next request on")
		require.True(t, actorFor(t, db, second.Id).IsAdmin, "an organization can have several admins")

		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(staffRole)}))
		require.Equal(t, orgmodel.RoleStaff, roleOf(t, db, first.Id))
		require.False(t, actorFor(t, db, first.Id).IsAdmin, "and so does the dismissal")
		require.True(t, actorFor(t, db, second.Id).IsAdmin, "dismissing one admin leaves the others")
	})
}

// Acceptance: admin 不能任免 owner — nor, since that power is the owner's
// alone, other admins.
func TestUpdateMember_AnAdminCannotAppointOrDismissAdmins(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		acting := actorFor(t, db, seedMember(t, db, acme, "acting-admin", orgmodel.RoleAdmin).Id)
		colleague := seedMember(t, db, acme, "colleague-admin", orgmodel.RoleAdmin)
		staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)

		require.ErrorIs(t, UpdateMember(db, acting, staff.Id, MemberPatch{RoleId: intPtr(adminRole)}), ErrOwnerOnly)
		require.ErrorIs(t, UpdateMember(db, acting, colleague.Id, MemberPatch{RoleId: intPtr(staffRole)}), ErrOwnerOnly)
		require.ErrorIs(t, UpdateMember(db, acting, acting.UserId, MemberPatch{RoleId: intPtr(staffRole)}), ErrOwnerOnly,
			"an admin stepping down is still a dismissal, and that is the owner's call")
		require.Equal(t, orgmodel.RoleStaff, roleOf(t, db, staff.Id))
		require.Equal(t, orgmodel.RoleAdmin, roleOf(t, db, colleague.Id))
		require.Equal(t, orgmodel.RoleAdmin, roleOf(t, db, acting.UserId))

		// Every other role is an admin's to hand out.
		for _, role := range []string{orgmodel.RoleManager, orgmodel.RoleReadonly, orgmodel.RoleStaff} {
			require.NoError(t, UpdateMember(db, acting, staff.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, role))}), role)
			require.Equal(t, role, roleOf(t, db, staff.Id))
		}
	})
}

// Decided 2026-10-06 (PRD D26): the owner and the admins run the whole
// organization, so they sit in its default department and not in a business
// unit. Becoming an admin takes a member there.
func TestUpdateMember_AppointingAnAdminMovesThemToTheDefaultDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		first := seedMember(t, db, acme, "first", orgmodel.RoleStaff)
		second := seedMember(t, db, acme, "second", orgmodel.RoleStaff)
		for _, member := range []platformmodel.User{first, second} {
			require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		}
		before := reloadUser(t, db, first.Id)

		// An admin in Sales is not something that exists: nothing is applied.
		err := UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(adminRole), DepartmentId: intPtr(sales.Id)})
		require.ErrorIs(t, err, ErrAdminDepartment)
		require.Equal(t, before, reloadUser(t, db, first.Id))

		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(adminRole)}))
		want := before
		want.OrgRoleId = adminRole
		want.DepartmentId = general.Id
		require.Equal(t, want, reloadUser(t, db, first.Id), "the role and the department moved, and nothing else")

		// Naming the default department in the same request is just saying it out loud.
		require.NoError(t, UpdateMember(db, owner, second.Id, MemberPatch{RoleId: intPtr(adminRole), DepartmentId: intPtr(general.Id)}))
		require.Equal(t, general.Id, reloadUser(t, db, second.Id).DepartmentId)
	})
}

// PRD D26: nobody moves the owner or an admin out of the default department —
// not the owner, not another admin, not they themselves.
func TestUpdateMember_TheOwnerAndAdminsStayInTheDefaultDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		first := seedMember(t, db, acme, "first-admin", orgmodel.RoleAdmin)
		second := seedMember(t, db, acme, "second-admin", orgmodel.RoleAdmin)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")

		for _, actor := range []*Actor{owner, actorFor(t, db, first.Id)} {
			for _, target := range []platformmodel.User{acme.owner, first, second} {
				err := UpdateMember(db, actor, target.Id, MemberPatch{DepartmentId: intPtr(sales.Id)})
				require.ErrorIs(t, err, ErrAdminDepartment, "user %d moving %s", actor.UserId, target.Username)
				require.Equal(t, general.Id, reloadUser(t, db, target.Id).DepartmentId, target.Username)
			}
		}

		// Saving the department they are already in changes nothing and is not
		// an error, so the page can submit the whole form.
		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{DepartmentId: intPtr(general.Id)}))
		require.Equal(t, first, reloadUser(t, db, first.Id))
	})
}

// PRD D26 ties the department to the role, not to the person: once dismissed,
// a former admin is an ordinary member who can be placed anywhere.
func TestUpdateMember_ADismissedAdminCanBeMovedAgain(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		first := seedMember(t, db, acme, "first-admin", orgmodel.RoleAdmin)
		second := seedMember(t, db, acme, "second-admin", orgmodel.RoleAdmin)

		// Dismissed and placed in one request.
		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(staffRole), DepartmentId: intPtr(sales.Id)}))
		require.Equal(t, orgmodel.RoleStaff, roleOf(t, db, first.Id))
		require.Equal(t, sales.Id, reloadUser(t, db, first.Id).DepartmentId)

		// Dismissed first: they stay where they were until someone moves them.
		require.NoError(t, UpdateMember(db, owner, second.Id, MemberPatch{RoleId: intPtr(staffRole)}))
		require.Equal(t, general.Id, reloadUser(t, db, second.Id).DepartmentId)
		require.NoError(t, UpdateMember(db, owner, second.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		require.Equal(t, sales.Id, reloadUser(t, db, second.Id).DepartmentId)
	})
}

// Acceptance: admin 不能任免 owner；owner 不可被移除. Nothing reachable from the
// API takes the owner role off the owner or puts it on anyone else.
func TestUpdateMember_TheOwnerRoleNeverMoves(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		ownerRole := presetRoleID(t, db, orgmodel.RoleOwner)
		owner := actorFor(t, db, acme.owner.Id)
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)
		staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)

		for _, actor := range []*Actor{owner, actorFor(t, db, admin.Id)} {
			for _, role := range []string{orgmodel.RoleAdmin, orgmodel.RoleManager, orgmodel.RoleStaff, orgmodel.RoleReadonly} {
				err := UpdateMember(db, actor, acme.owner.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, role))})
				require.ErrorIs(t, err, ErrOwnerImmutable, "demoting the owner to %s", role)
			}
			for _, target := range []int{admin.Id, staff.Id} {
				err := UpdateMember(db, actor, target, MemberPatch{RoleId: intPtr(ownerRole)})
				require.ErrorIs(t, err, ErrOwnerImmutable, "making user %d an owner", target)
			}
		}

		require.Equal(t, orgmodel.RoleOwner, roleOf(t, db, acme.owner.Id))
		var owners int64
		require.NoError(t, db.Model(&platformmodel.User{}).
			Where("org_id = ? AND role_id = ?", acme.id, ownerRole).Count(&owners).Error)
		require.EqualValues(t, 1, owners, "the organization still has exactly one owner")
		var org orgmodel.Organization
		require.NoError(t, db.First(&org, acme.id).Error)
		require.Equal(t, acme.owner.Id, org.OwnerUserId)

		// Re-saving the role and the department the owner already has is not a
		// change, so the page can submit the whole form.
		general := defaultDepartment(t, db, acme.id)
		require.NoError(t, UpdateMember(db, owner, acme.owner.Id, MemberPatch{RoleId: intPtr(ownerRole), DepartmentId: intPtr(general.Id)}))
		require.Equal(t, acme.owner, reloadUser(t, db, acme.owner.Id))
	})
}

// PRD §7.3 red line 1: an org role is never a platform role. Appointing an
// admin changes users.role_id and nothing else about the row.
func TestUpdateMember_NeverWritesTheGlobalRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		adminRole := presetRoleID(t, db, orgmodel.RoleAdmin)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)
		before := reloadUser(t, db, member.Id)

		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(adminRole)}))

		want := before
		want.OrgRoleId = adminRole
		got := reloadUser(t, db, member.Id)
		require.Equal(t, want, got)
		require.Equal(t, common.RoleCommonUser, got.Role)
	})
}

// Acceptance: owner/admin 能…调整成员的部门归属.
func TestUpdateMember_MovesAMemberBetweenDepartments(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)
		sales := departmentNamed(t, db, acme.id, "Sales")
		theirSales := departmentNamed(t, db, other.id, "Sales")
		before := reloadUser(t, db, member.Id)

		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		want := before
		want.DepartmentId = sales.Id
		require.Equal(t, want, reloadUser(t, db, member.Id), "only the department moved")

		// A department is always one of the company's own, and never "none".
		for _, bad := range []int{theirSales.Id, 0, 424242} {
			err := UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(bad)})
			require.ErrorIs(t, err, ErrDepartmentNotFound, "department %d", bad)
		}
		require.NoError(t, DeleteDepartment(db, owner, sales.Id))
		require.ErrorIs(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}), ErrDepartmentNotFound,
			"a deleted department takes nobody")

		// An empty patch is a no-op, not an error.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{}))
	})
}

// A patch is all or nothing: when the role half is refused, the department
// half must not have been applied.
func TestUpdateMember_AppliesNothingWhenOneHalfIsRefused(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		acting := actorFor(t, db, seedMember(t, db, acme, "acting-admin", orgmodel.RoleAdmin).Id)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)
		sales := departmentNamed(t, db, acme.id, "Sales")
		before := reloadUser(t, db, member.Id)

		err := UpdateMember(db, acting, member.Id, MemberPatch{
			RoleId:       intPtr(presetRoleID(t, db, orgmodel.RoleAdmin)),
			DepartmentId: intPtr(sales.Id),
		})
		require.ErrorIs(t, err, ErrOwnerOnly)
		require.Equal(t, before, reloadUser(t, db, member.Id))
	})
}

func TestUpdateMember_OnlyTakesRolesTheOrganizationMayUse(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)
		own := orgmodel.OrgRole{OrgId: acme.id, Name: "IT Ops", Scope: orgmodel.ScopeOrg, Permissions: "key.create"}
		require.NoError(t, db.Create(&own).Error)
		theirs := orgmodel.OrgRole{OrgId: other.id, Name: "Their role", Scope: orgmodel.ScopeOrg, Permissions: "key.delete"}
		require.NoError(t, db.Create(&theirs).Error)

		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(own.Id)}))
		require.Equal(t, "IT Ops", roleOf(t, db, member.Id))

		for _, bad := range []int{theirs.Id, 0, 424242} {
			err := UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(bad)})
			require.ErrorIs(t, err, ErrRoleNotFound, "role %d", bad)
		}
		require.Equal(t, "IT Ops", roleOf(t, db, member.Id))
	})
}

func TestUpdateMember_OnlyReachesMembersOfTheActorsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)
		stranger := seedMember(t, db, other, "stranger", orgmodel.RoleStaff)
		personal := seedUser(t, db, "personal", common.RoleCommonUser)
		sales := departmentNamed(t, db, acme.id, "Sales")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)

		for _, target := range []platformmodel.User{stranger, personal, other.owner} {
			before := reloadUser(t, db, target.Id)
			err := UpdateMember(db, owner, target.Id, MemberPatch{RoleId: intPtr(managerRole), DepartmentId: intPtr(sales.Id)})
			require.ErrorIs(t, err, ErrMemberNotFound, target.Username)
			require.Equal(t, before, reloadUser(t, db, target.Id), target.Username)
		}
		require.ErrorIs(t, UpdateMember(db, owner, 424242, MemberPatch{RoleId: intPtr(managerRole)}), ErrMemberNotFound)
	})
}

// Acceptance: admin 能创建服务账号（不可登录的成员）.
func TestCreateServiceAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		previous := common.QuotaForNewUser
		common.QuotaForNewUser = 500000 // sign-ups get trial credit; a service account must not
		t.Cleanup(func() { common.QuotaForNewUser = previous })

		acme := seedOrg(t, db, "Acme")
		admin := actorFor(t, db, seedMember(t, db, acme, "admin", orgmodel.RoleAdmin).Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)

		created, err := CreateServiceAccount(db, admin, "  CI Pipeline  ", sales.Id)
		require.NoError(t, err)
		require.Equal(t, &MemberView{
			Id:           created.Id,
			Username:     created.Username,
			DisplayName:  "CI Pipeline",
			RoleId:       staffRole,
			Role:         orgmodel.RoleStaff,
			DepartmentId: sales.Id,
			IsService:    true,
			// A service account is staff: it manages nothing.
			ManagedDepartmentIds: []int{},
		}, created)
		require.Regexp(t, `^svc-[a-z0-9]{12}$`, created.Username)
		require.LessOrEqual(t, len(created.Username), platformmodel.UserNameMaxLength)

		stored := reloadUser(t, db, created.Id)
		require.True(t, stored.IsService)
		require.Equal(t, acme.id, stored.OrgId)
		require.Equal(t, staffRole, stored.OrgRoleId)
		require.Equal(t, sales.Id, stored.DepartmentId)
		// A common user on the platform, enabled so that its keys work …
		require.Equal(t, common.RoleCommonUser, stored.Role)
		require.Equal(t, common.UserStatusEnabled, stored.Status)
		require.Equal(t, "default", stored.Group)
		// … with nothing to sign in with, and no sign-up gift.
		require.Empty(t, stored.Password)
		require.Empty(t, stored.Email)
		require.Nil(t, stored.AccessToken)
		require.Empty(t, stored.GitHubId+stored.DiscordId+stored.OidcId+stored.WeChatId+stored.TelegramId+stored.LinuxDOId)
		require.Zero(t, stored.Quota)
		require.Zero(t, stored.InviterId)

		// Without a department it lands in the default one.
		unplaced, err := CreateServiceAccount(db, admin, "Bot", 0)
		require.NoError(t, err)
		require.Equal(t, defaultDepartment(t, db, acme.id).Id, unplaced.DepartmentId)

		// The name is the company's to choose, so two can share one; the
		// generated usernames keep them apart.
		twin, err := CreateServiceAccount(db, admin, "Bot", 0)
		require.NoError(t, err)
		require.NotEqual(t, unplaced.Username, twin.Username)

		longest := strings.Repeat("机", ServiceAccountNameMaxLength)
		_, err = CreateServiceAccount(db, admin, longest, 0)
		require.NoError(t, err, "the limit counts characters, not bytes")
		for _, bad := range []string{"", "   ", longest + "机"} {
			_, err := CreateServiceAccount(db, admin, bad, 0)
			require.ErrorIs(t, err, ErrInvalidServiceAccountName, "%q", bad)
		}
		_, err = CreateServiceAccount(db, admin, "Lost", 424242)
		require.ErrorIs(t, err, ErrDepartmentNotFound)

		var accounts int64
		require.NoError(t, db.Model(&platformmodel.User{}).
			Where("org_id = ? AND is_service = ?", acme.id, true).Count(&accounts).Error)
		require.EqualValues(t, 4, accounts, "refused requests created nothing")
	})
}

// A service account is a member that holds keys, not a seat with authority:
// it stays staff and can never be made an admin.
func TestUpdateMember_AServiceAccountKeepsItsRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		bot, err := CreateServiceAccount(db, owner, "CI", 0)
		require.NoError(t, err)
		sales := departmentNamed(t, db, acme.id, "Sales")

		for _, role := range []string{orgmodel.RoleAdmin, orgmodel.RoleManager, orgmodel.RoleReadonly} {
			err := UpdateMember(db, owner, bot.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, role))})
			require.ErrorIs(t, err, ErrServiceAccountRole, role)
		}
		require.Equal(t, orgmodel.RoleStaff, roleOf(t, db, bot.Id))

		// Its department is as movable as anyone's.
		require.NoError(t, UpdateMember(db, owner, bot.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		require.Equal(t, sales.Id, reloadUser(t, db, bot.Id).DepartmentId)
	})
}

func TestOwnsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)
		personal := seedUser(t, db, "personal", common.RoleCommonUser)

		for userID, want := range map[int]bool{acme.owner.Id: true, admin.Id: false, personal.Id: false, 424242: false} {
			owns, err := OwnsOrganization(db, userID)
			require.NoError(t, err)
			require.Equal(t, want, owns, "user %d", userID)
		}
	})
}
