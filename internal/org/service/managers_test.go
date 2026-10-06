package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P4, decided 2026-10-06 (PRD D28): a member whose role has
// department scope manages the department they belong to, and the owner or an
// admin can add further ones.

// managedBy returns the departments a member manages, as the member list
// reports them.
func managedBy(t *testing.T, db *gorm.DB, viewer *Actor, memberID int) []int {
	t.Helper()
	members, err := ListMembers(db, viewer)
	require.NoError(t, err)
	for _, member := range members {
		if member.Id == memberID {
			return member.ManagedDepartmentIds
		}
	}
	t.Fatalf("member %d is not in the list", memberID)
	return nil
}

func TestManagedDepartments_AManagerManagesTheDepartmentTheyBelongTo(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		member := seedMemberIn(t, db, acme, "member", presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)

		// Staff manage nothing.
		require.Empty(t, actorFor(t, db, member.Id).Departments)
		require.Equal(t, []int{}, managedBy(t, db, owner, member.Id))

		// Made a manager: the department they sit in, with nothing to set up.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(managerRole)}))
		require.Equal(t, []int{sales.Id}, actorFor(t, db, member.Id).Departments)
		require.Equal(t, []int{sales.Id}, managedBy(t, db, owner, member.Id))
		require.Empty(t, managerRows(t, db, member.Id), "their own department needs no row")

		// Moved: what they manage moves with them.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(product.Id)}))
		require.Equal(t, []int{product.Id}, actorFor(t, db, member.Id).Departments)

		// The same goes for someone who joins through a manager invite.
		invite, err := CreateInvite(db, owner, managerRole, sales.Id)
		require.NoError(t, err)
		newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
		require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code))
		require.Equal(t, []int{sales.Id}, actorFor(t, db, newcomer.Id).Departments)

		// Membership says the same to the member themselves.
		membership, err := GetMembership(db, member.Id)
		require.NoError(t, err)
		require.Equal(t, []int{product.Id}, membership.ManagedDepartmentIds)
	})
}

// PRD D4: a manager is bound to one department or several.
func TestManagedDepartments_TheOwnerAddsFurtherOnes(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		support := departmentNamed(t, db, acme.id, "Customer Support")
		manager := seedMemberIn(t, db, acme, "manager", presetRoleID(t, db, orgmodel.RoleManager), sales.Id)

		// Their own department is always one of them, listed or not, and a
		// department named twice counts once.
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{ManagedDepartmentIds: intsPtr(support.Id, product.Id, support.Id)}))
		require.Equal(t, []int{product.Id, support.Id}, managerRows(t, db, manager.Id))
		require.Equal(t, []int{sales.Id, product.Id, support.Id}, actorFor(t, db, manager.Id).Departments, "their own first")
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{ManagedDepartmentIds: intsPtr(sales.Id, product.Id)}))
		require.Equal(t, []int{product.Id}, managerRows(t, db, manager.Id), "the list replaces what was there")

		// It is what they can then act in.
		actor := actorFor(t, db, manager.Id)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		for _, department := range []orgmodel.Department{sales, product} {
			_, err := CreateInvite(db, actor, staffRole, department.Id)
			require.NoError(t, err, department.Name)
		}
		_, err := CreateInvite(db, actor, staffRole, support.Id)
		require.ErrorIs(t, err, ErrForbidden)

		// An added department stays added when they move, and stops counting
		// twice when they move into it.
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{DepartmentId: intPtr(general.Id)}))
		require.Equal(t, []int{general.Id, product.Id}, actorFor(t, db, manager.Id).Departments)
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{DepartmentId: intPtr(product.Id)}))
		require.Equal(t, []int{product.Id}, actorFor(t, db, manager.Id).Departments)

		// An empty list leaves them with their own department only.
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{ManagedDepartmentIds: intsPtr()}))
		require.Empty(t, managerRows(t, db, manager.Id))
		require.Equal(t, []int{product.Id}, actorFor(t, db, manager.Id).Departments)

		// One request can move them and say what else they manage.
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{DepartmentId: intPtr(sales.Id), ManagedDepartmentIds: intsPtr(sales.Id, support.Id)}))
		require.Equal(t, []int{sales.Id, support.Id}, actorFor(t, db, manager.Id).Departments)
		require.Equal(t, []int{support.Id}, managerRows(t, db, manager.Id))
	})
}

func TestManagedDepartments_OnlyADepartmentScopedRoleManagesAny(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		readonlyRole := presetRoleID(t, db, orgmodel.RoleReadonly)
		member := seedMemberIn(t, db, acme, "member", staffRole, sales.Id)
		admin := seedMember(t, db, acme, "admin", orgmodel.RoleAdmin)

		// Staff, readonly, admins and the owner manage no department.
		for _, target := range []int{member.Id, admin.Id, acme.owner.Id} {
			err := UpdateMember(db, owner, target, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id)})
			require.ErrorIs(t, err, ErrNotDepartmentScoped, "user %d", target)
			require.Empty(t, managerRows(t, db, target))
		}
		// Saying "none" about them is true, so the page can submit it.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{ManagedDepartmentIds: intsPtr()}))

		// The role and the departments can arrive in one request …
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(managerRole), ManagedDepartmentIds: intsPtr(product.Id)}))
		require.Equal(t, []int{sales.Id, product.Id}, actorFor(t, db, member.Id).Departments)
		// … and a request that takes the role away cannot keep them.
		err := UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(readonlyRole), ManagedDepartmentIds: intsPtr(product.Id)})
		require.ErrorIs(t, err, ErrNotDepartmentScoped)
		require.Equal(t, managerRole, reloadUser(t, db, member.Id).OrgRoleId, "nothing of the refused request was applied")

		// Losing the role drops what was added: made a manager again later,
		// they start from their own department, not from an old list.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(staffRole)}))
		require.Empty(t, managerRows(t, db, member.Id))
		require.Empty(t, actorFor(t, db, member.Id).Departments)
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{RoleId: intPtr(managerRole)}))
		require.Equal(t, []int{sales.Id}, actorFor(t, db, member.Id).Departments)

		// Only real departments of this company can be managed.
		for _, bad := range []int{0, 424242} {
			err := UpdateMember(db, owner, member.Id, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id, bad)})
			require.ErrorIs(t, err, ErrDepartmentNotFound, "department %d", bad)
		}
		require.Empty(t, managerRows(t, db, member.Id))

		// A deleted department is nobody's to manage any more.
		require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id)}))
		require.NoError(t, DeleteDepartment(db, owner, product.Id))
		require.Empty(t, managerRows(t, db, member.Id))
		require.Equal(t, []int{sales.Id}, actorFor(t, db, member.Id).Departments)
	})
}

// Acceptance: 部门作用域硬检查 — what a manager is shown is cut to the
// departments they manage, not merely hidden on the page.
func TestManagedDepartments_AManagerSeesOnlyTheirDepartments(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		manager := seedMemberIn(t, db, acme, "manager", presetRoleID(t, db, orgmodel.RoleManager), sales.Id)
		inSales := seedMemberIn(t, db, acme, "in-sales", staffRole, sales.Id)
		inProduct := seedMemberIn(t, db, acme, "in-product", staffRole, product.Id)
		for _, departmentID := range []int{sales.Id, product.Id, 0} {
			_, err := CreateInvite(db, owner, staffRole, departmentID)
			require.NoError(t, err)
		}
		// A link that brings managers into Sales is in the manager's own
		// department, and still not theirs to see: they could not have made it.
		_, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleManager), sales.Id)
		require.NoError(t, err)

		ids := func(members []MemberView) []int {
			list := []int{}
			for _, member := range members {
				list = append(list, member.Id)
			}
			return list
		}
		actor := actorFor(t, db, manager.Id)

		members, err := ListMembers(db, actor)
		require.NoError(t, err)
		require.Equal(t, []int{manager.Id, inSales.Id}, ids(members))
		departments, err := ListDepartments(db, actor)
		require.NoError(t, err)
		require.Equal(t, []DepartmentView{{Id: sales.Id, Name: "Sales", MemberCount: 2}}, departments)
		invites, err := ListInvites(db, actor)
		require.NoError(t, err)
		require.Len(t, invites, 1)
		require.Equal(t, sales.Id, invites[0].DepartmentId)
		require.Equal(t, orgmodel.RoleStaff, invites[0].Role)

		// Given Product as well, they see both — on their next request.
		require.NoError(t, UpdateMember(db, owner, manager.Id, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id)}))
		actor = actorFor(t, db, manager.Id)
		members, err = ListMembers(db, actor)
		require.NoError(t, err)
		require.Equal(t, []int{manager.Id, inSales.Id, inProduct.Id}, ids(members))
		departments, err = ListDepartments(db, actor)
		require.NoError(t, err)
		require.Len(t, departments, 2)
		invites, err = ListInvites(db, actor)
		require.NoError(t, err)
		require.Len(t, invites, 2)

		// Roles belong to no department, so the whole list is theirs to read.
		roles, err := ListRoles(db, actor)
		require.NoError(t, err)
		require.Len(t, roles, 5)

		// A readonly member, whose role reaches the whole organization, sees
		// every member and department — and no invite link at all.
		readonly := actorFor(t, db, seedMember(t, db, acme, "readonly", orgmodel.RoleReadonly).Id)
		members, err = ListMembers(db, readonly)
		require.NoError(t, err)
		require.Len(t, members, 5)
		departments, err = ListDepartments(db, readonly)
		require.NoError(t, err)
		require.Len(t, departments, 6)
		_, err = ListInvites(db, readonly)
		require.ErrorIs(t, err, ErrForbidden)
	})
}
