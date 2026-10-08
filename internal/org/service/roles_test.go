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

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md): custom roles built
// from the primitives, and the role packs that are copied into them.

// roleRow reads a role straight from the table, deleted or not.
func roleRow(t *testing.T, db *gorm.DB, roleID int) orgmodel.OrgRole {
	t.Helper()
	var role orgmodel.OrgRole
	require.NoError(t, db.Unscoped().First(&role, roleID).Error)
	return role
}

// Acceptance: admin 能从权限原语清单自建自定义角色并分配给成员.
func TestCreateRole_BuildsARoleFromThePrimitivesAndItCanBeAssigned(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		admin := actorFor(t, db, seedMember(t, db, acme, "admin", orgmodel.RoleAdmin).Id)
		member := seedMember(t, db, acme, "member", orgmodel.RoleStaff)

		// Asked for out of order, twice over and with stray spaces: stored
		// once each, in the order of the primitive list.
		role, err := CreateRole(db, admin, RoleInput{
			Name:        "  Key Desk  ",
			Scope:       orgmodel.ScopeOrg,
			Permissions: []string{"usage.read", "key.freeze", " key.read ", "key.freeze"},
		})
		require.NoError(t, err)
		require.Equal(t, &RoleView{
			Id: role.Id, Name: "Key Desk", Scope: orgmodel.ScopeOrg,
			Permissions: []string{"key.read", "key.freeze", "usage.read"},
			Powers:      []string{}, // no custom role carries an inherent power
		}, role)
		require.Equal(t, orgmodel.OrgRole{
			Id: role.Id, OrgId: acme.id, Name: "Key Desk", Scope: orgmodel.ScopeOrg,
			Permissions: "key.read,key.freeze,usage.read",
		}, roleRow(t, db, role.Id))

		require.NoError(t, UpdateMember(db, admin, member.Id, MemberPatch{RoleId: intPtr(role.Id)}))
		holder := actorFor(t, db, member.Id)
		require.Equal(t, orgmodel.ScopeOrg, holder.Scope)
		require.Equal(t, []string{"key.read", "key.freeze", "usage.read"}, holder.Permissions)
		require.False(t, holder.IsAdmin, "a custom role never makes an admin")

		// It is on offer next to the presets from now on.
		roles, err := ListRoles(db, admin)
		require.NoError(t, err)
		require.Len(t, roles, 6)
		require.Equal(t, *role, roles[5])

		// PRD §2: 写权限自动包含同一资源的读权限. A role asked for with writes
		// only is stored — and shown, and reported to its holder — with the
		// reads they bring, so no page has to work the rule out for itself.
		writer, err := CreateRole(db, admin, RoleInput{Name: "Onboarding", Scope: orgmodel.ScopeDept, Permissions: []string{"member.invite", "key.assign"}})
		require.NoError(t, err)
		require.Equal(t, []string{"key.read", "key.assign", "member.read", "member.invite"}, writer.Permissions)
		require.Equal(t, "key.read,key.assign,member.read,member.invite", roleRow(t, db, writer.Id).Permissions)
		// A role written into the table without them still grants them.
		bare := seedRole(t, db, acme.id, "Bare", orgmodel.ScopeOrg, "member.remove")
		require.NoError(t, UpdateMember(db, admin, member.Id, MemberPatch{RoleId: intPtr(bare.Id)}))
		membership, err := GetMembership(db, member.Id)
		require.NoError(t, err)
		require.Equal(t, []string{"member.read", "member.remove"}, membership.Permissions)
		_, err = ListMembers(db, actorFor(t, db, member.Id))
		require.NoError(t, err)
	})
}

func TestCreateRole_RefusesWhatARoleCannotBe(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		longest := strings.Repeat("角", RoleNameMaxLength)
		good := RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}
		_, err := CreateRole(db, owner, good)
		require.NoError(t, err)

		for name, tc := range map[string]struct {
			input RoleInput
			want  error
		}{
			"no name":               {RoleInput{Name: "   ", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrInvalidRoleName},
			"a name too long":       {RoleInput{Name: longest + "角", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrInvalidRoleName},
			"a taken name":          {good, ErrRoleExists},
			"a preset's name":       {RoleInput{Name: "admin", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrRoleExists},
			"a preset's name, big":  {RoleInput{Name: " Owner ", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrRoleExists},
			"no scope":              {RoleInput{Name: "A", Permissions: []string{"key.read"}}, ErrInvalidRoleScope},
			"the staff scope":       {RoleInput{Name: "B", Scope: orgmodel.ScopeSelf, Permissions: []string{"key.read"}}, ErrInvalidRoleScope},
			"an unknown scope":      {RoleInput{Name: "C", Scope: "world", Permissions: []string{"key.read"}}, ErrInvalidRoleScope},
			"no permissions":        {RoleInput{Name: "D", Scope: orgmodel.ScopeOrg}, ErrInvalidRolePermissions},
			"a wildcard":            {RoleInput{Name: "E", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.*"}}, ErrInvalidRolePermissions},
			"an unknown permission": {RoleInput{Name: "F", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read", "key.steal"}}, ErrInvalidRolePermissions},
			// PRD §2: managing the organization itself is not a primitive.
			"an inherent power":        {RoleInput{Name: "G", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read", orgmodel.PowerRoles}}, ErrInvalidRolePermissions},
			"the admin power":          {RoleInput{Name: "H", Scope: orgmodel.ScopeOrg, Permissions: []string{orgmodel.PowerAdmins}}, ErrInvalidRolePermissions},
			"audit.read by department": {RoleInput{Name: "I", Scope: orgmodel.ScopeDept, Permissions: []string{"member.read", "audit.read"}}, ErrAuditNeedsOrgScope},
		} {
			_, err := CreateRole(db, owner, tc.input)
			require.ErrorIs(t, err, tc.want, name)
		}
		var roles int64
		require.NoError(t, db.Model(&orgmodel.OrgRole{}).Where("org_id = ?", acme.id).Count(&roles).Error)
		require.EqualValues(t, 1, roles, "refused requests created nothing")
		require.Len(t, auditRecords(t, db, acme.id), 1, "and recorded nothing")

		// The limit counts characters, and a name is only taken inside one company.
		_, err = CreateRole(db, owner, RoleInput{Name: longest, Scope: orgmodel.ScopeDept, Permissions: []string{"member.read"}})
		require.NoError(t, err)
		other := seedOrg(t, db, "Other")
		_, err = CreateRole(db, actorFor(t, db, other.owner.Id), good)
		require.NoError(t, err)
	})
}

// Acceptance: 修改角色权限即时生效 — the role side of it. UpdateRole replaces
// name, scope and permissions together.
func TestUpdateRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		role, err := CreateRole(db, owner, RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)
		sibling, err := CreateRole(db, owner, RoleInput{Name: "Sibling", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)

		updated, err := UpdateRole(db, owner, role.Id, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeDept, Permissions: []string{"member.invite", "key.assign"}})
		require.NoError(t, err)
		// PRD §2: a write brings the read of the same resource, and the role
		// is stored saying so.
		require.Equal(t, &RoleView{Id: role.Id, Name: "Team Lead", Scope: orgmodel.ScopeDept,
			Permissions: []string{"key.read", "key.assign", "member.read", "member.invite"}, Powers: []string{}}, updated)
		require.Equal(t, "key.read,key.assign,member.read,member.invite", roleRow(t, db, role.Id).Permissions)

		// Saving a role under the name it already has is not a clash with itself.
		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeDept, Permissions: []string{"key.assign"}})
		require.NoError(t, err)

		for name, tc := range map[string]struct {
			roleID int
			input  RoleInput
			want   error
		}{
			"a sibling's name":         {role.Id, RoleInput{Name: "Sibling", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrRoleExists},
			"a preset's name":          {role.Id, RoleInput{Name: "Manager", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrRoleExists},
			"nothing left in it":       {role.Id, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeOrg}, ErrInvalidRolePermissions},
			"an unknown role":          {424242, RoleInput{Name: "X", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}}, ErrRoleNotFound},
			"audit.read by department": {sibling.Id, RoleInput{Name: "Sibling", Scope: orgmodel.ScopeDept, Permissions: []string{"audit.read"}}, ErrAuditNeedsOrgScope},
		} {
			_, err := UpdateRole(db, owner, tc.roleID, tc.input)
			require.ErrorIs(t, err, tc.want, name)
		}
		require.Equal(t, "key.read,key.assign", roleRow(t, db, role.Id).Permissions, "refused changes left the role alone")
		require.Equal(t, "Sibling", roleRow(t, db, sibling.Id).Name)
	})
}

// The five presets are the platform's: the same for every organization, and
// nobody's to edit — not the owner's either.
func TestPresetRoles_CannotBeChangedOrDeleted(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		var before []orgmodel.OrgRole
		require.NoError(t, db.Where("org_id = ?", 0).Order("id").Find(&before).Error)
		require.Len(t, before, 5)

		for _, preset := range before {
			_, err := UpdateRole(db, owner, preset.Id, RoleInput{Name: "Mine", Scope: orgmodel.ScopeOrg, Permissions: orgmodel.Primitives})
			require.ErrorIs(t, err, ErrPresetRole, preset.Name)
			require.ErrorIs(t, DeleteRole(db, owner, preset.Id), ErrPresetRole, preset.Name)
		}

		var after []orgmodel.OrgRole
		require.NoError(t, db.Where("org_id = ?", 0).Order("id").Find(&after).Error)
		require.Equal(t, before, after)
		require.Empty(t, auditRecords(t, db, acme.id))
	})
}

// Decided 2026-10-06 (PRD D29): deleting a role that is still in use does not
// leave anybody pointing at nothing — its holders and its unused invite links
// fall back to staff, the role that grants nothing.
func TestDeleteRole_MakesItsHoldersAndInviteLinksStaff(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
		doomed, err := CreateRole(db, owner, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeDept, Permissions: []string{"member.invite"}})
		require.NoError(t, err)
		kept, err := CreateRole(db, owner, RoleInput{Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)

		first := seedMemberIn(t, db, acme, "first", doomed.Id, sales.Id)
		second := seedMemberIn(t, db, acme, "second", doomed.Id, sales.Id)
		bystander := seedMemberIn(t, db, acme, "bystander", kept.Id, sales.Id)
		require.NoError(t, UpdateMember(db, owner, first.Id, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id)}))
		doomedInvite, err := CreateInvite(db, owner, doomed.Id, sales.Id)
		require.NoError(t, err)
		keptInvite, err := CreateInvite(db, owner, kept.Id, sales.Id)
		require.NoError(t, err)

		require.NoError(t, DeleteRole(db, owner, doomed.Id))

		for _, member := range []platformmodel.User{first, second} {
			want := member
			want.OrgRoleId = staffRole
			require.Equal(t, want, reloadUser(t, db, member.Id), "%s kept everything but the role", member.Username)
			require.Empty(t, managerRows(t, db, member.Id), "staff manage no department")
			actor := actorFor(t, db, member.Id)
			require.Empty(t, actor.Permissions)
			require.Equal(t, orgmodel.ScopeSelf, actor.Scope)
		}
		require.Equal(t, bystander, reloadUser(t, db, bystander.Id), "holders of other roles are untouched")
		// 🔴 Still common users on the platform, all of them.
		require.Equal(t, common.RoleCommonUser, reloadUser(t, db, first.Id).Role)

		var repointed, untouched orgmodel.OrgInvite
		require.NoError(t, db.First(&repointed, doomedInvite.Id).Error)
		require.Equal(t, staffRole, repointed.RoleId, "the link still works, and now brings in staff")
		require.Equal(t, sales.Id, repointed.DepartmentId)
		require.NoError(t, db.First(&untouched, keptInvite.Id).Error)
		require.Equal(t, kept.Id, untouched.RoleId)

		// Gone for every purpose: not on offer, not assignable, not deletable
		// twice — and its name is free again.
		roles, err := ListRoles(db, owner)
		require.NoError(t, err)
		for _, role := range roles {
			require.NotEqual(t, doomed.Id, role.Id)
		}
		require.ErrorIs(t, UpdateMember(db, owner, first.Id, MemberPatch{RoleId: intPtr(doomed.Id)}), ErrRoleNotFound)
		require.ErrorIs(t, DeleteRole(db, owner, doomed.Id), ErrRoleNotFound)
		_, err = CreateRole(db, owner, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.NoError(t, err)

		// The audit log says which role went and who lost it.
		var deletion orgmodel.OrgAuditLog
		require.NoError(t, db.Where("org_id = ? AND action = ?", acme.id, orgmodel.AuditRoleDelete).First(&deletion).Error)
		var detail struct {
			Before struct {
				Name      string `json:"name"`
				MemberIds []int  `json:"member_ids"`
			} `json:"before"`
		}
		require.NoError(t, common.Unmarshal([]byte(deletion.Detail), &detail))
		require.Equal(t, "Team Lead", detail.Before.Name)
		require.Equal(t, []int{first.Id, second.Id}, detail.Before.MemberIds)
	})
}

// A role that stops having department scope leaves its holders managing no
// department: the extra ones are dropped, not kept for a later surprise.
func TestUpdateRole_LeavingDepartmentScopeDropsWhatItsHoldersManaged(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")
		role, err := CreateRole(db, owner, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeDept, Permissions: []string{"member.read"}})
		require.NoError(t, err)
		lead := seedMemberIn(t, db, acme, "lead", role.Id, sales.Id)
		manager := seedMemberIn(t, db, acme, "manager", presetRoleID(t, db, orgmodel.RoleManager), sales.Id)
		for _, member := range []platformmodel.User{lead, manager} {
			require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{ManagedDepartmentIds: intsPtr(product.Id)}))
		}
		require.Equal(t, []int{sales.Id, product.Id}, actorFor(t, db, lead.Id).Departments)

		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeOrg, Permissions: []string{"member.read"}})
		require.NoError(t, err)
		require.Empty(t, managerRows(t, db, lead.Id))
		require.Empty(t, actorFor(t, db, lead.Id).Departments)
		require.Equal(t, []int{product.Id}, managerRows(t, db, manager.Id), "holders of other roles keep theirs")

		// Back to department scope: they manage their own department again,
		// and only that.
		_, err = UpdateRole(db, owner, role.Id, RoleInput{Name: "Team Lead", Scope: orgmodel.ScopeDept, Permissions: []string{"member.read"}})
		require.NoError(t, err)
		require.Equal(t, []int{sales.Id}, actorFor(t, db, lead.Id).Departments)
	})
}

// Acceptance: 平台预置的岗位角色包可一键采用为组织的自定义角色，采用后可自由修改.
func TestAdoptRolePack_CopiesThePackIntoARoleTheOrganizationOwns(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		owner := actorFor(t, db, acme.owner.Id)

		// Every pack can be adopted as it is.
		for _, pack := range orgmodel.RolePacks {
			role, err := AdoptRolePack(db, owner, pack.Key, "")
			require.NoError(t, err, pack.Key)
			require.Equal(t, &RoleView{Id: role.Id, Name: pack.Name, Scope: pack.Scope, Permissions: pack.Permissions, Powers: []string{}}, role)
			require.Equal(t, acme.id, roleRow(t, db, role.Id).OrgId)
			require.False(t, roleRow(t, db, role.Id).IsPreset)
		}

		// The page sends the name it shows the pack under.
		named, err := AdoptRolePack(db, owner, "finance", "  财务对账  ")
		require.NoError(t, err)
		require.Equal(t, "财务对账", named.Name)
		require.Equal(t, []string{"usage.read"}, named.Permissions)

		// The copy is the organization's own: changing or deleting it touches
		// neither the pack nor anyone else's copy.
		theirs, err := AdoptRolePack(db, actorFor(t, db, other.owner.Id), "finance", "")
		require.NoError(t, err)
		changed, err := UpdateRole(db, owner, named.Id, RoleInput{Name: "Bookkeeping", Scope: orgmodel.ScopeDept, Permissions: []string{"usage.read", "member.read"}})
		require.NoError(t, err)
		require.Equal(t, []string{"member.read", "usage.read"}, changed.Permissions)
		require.NoError(t, DeleteRole(db, owner, named.Id))
		require.Equal(t, []string{"usage.read"}, packPermissions(t, "finance"))
		require.Equal(t, "usage.read", roleRow(t, db, theirs.Id).Permissions)
		again, err := AdoptRolePack(db, owner, "finance", "Bookkeeping")
		require.NoError(t, err, "and the pack can be adopted again")
		require.Equal(t, []string{"usage.read"}, again.Permissions)

		// Adopting twice under one name is a clash like any other.
		_, err = AdoptRolePack(db, owner, "it_ops", "")
		require.ErrorIs(t, err, ErrRoleExists)
		_, err = AdoptRolePack(db, owner, "it_ops", "admin")
		require.ErrorIs(t, err, ErrRoleExists)
		_, err = AdoptRolePack(db, owner, "no_such_pack", "")
		require.ErrorIs(t, err, ErrRolePackNotFound)
	})
}

// PRD §2, the last column of the role table: managing the organization itself
// comes with the owner and admin presets and with nothing else. The roles page
// ticks its matrix from this list, so it is written out by hand here.
func TestListRoles_SaysWhichInherentPowersEachRoleCarries(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		// A custom role may hold every primitive, or be named like a preset;
		// neither gives it a power.
		_, err := CreateRole(db, owner, RoleInput{Name: "Everything", Scope: orgmodel.ScopeOrg, Permissions: orgmodel.Primitives})
		require.NoError(t, err)
		seedRole(t, db, acme.id, orgmodel.RoleAdmin, orgmodel.ScopeOrg, orgmodel.Primitives...)

		roles, err := ListRoles(db, owner)
		require.NoError(t, err)
		powers := map[string][]string{}
		for _, role := range roles {
			key := role.Name
			if !role.IsPreset {
				key = "custom " + role.Name
			}
			powers[key] = role.Powers
		}
		shared := []string{"role.manage", "department.manage", "service_account.manage", "org.settings", "alert.handle"}
		require.Equal(t, map[string][]string{
			"owner":             append(append([]string{}, shared...), "admin.appoint", "wallet.manage", "org.transfer"),
			"admin":             shared,
			"manager":           {},
			"staff":             {},
			"readonly":          {},
			"custom Everything": {},
			"custom admin":      {},
		}, powers)
	})
}

// The roles page draws its matrix from this, so it carries the whole
// vocabulary: every primitive, every inherent power, every pack.
func TestGetPermissionCatalog(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		catalog, err := GetPermissionCatalog(actorFor(t, db, acme.owner.Id))
		require.NoError(t, err)
		require.Equal(t, orgmodel.Primitives, catalog.Primitives)
		require.Equal(t, orgmodel.InherentPowers, catalog.Powers)
		require.Len(t, catalog.RolePacks, len(orgmodel.RolePacks))
		for i, pack := range orgmodel.RolePacks {
			require.Equal(t, RolePackView{Key: pack.Key, Name: pack.Name, Scope: pack.Scope, Permissions: pack.Permissions}, catalog.RolePacks[i])
		}

		encoded, err := common.Marshal(catalog)
		require.NoError(t, err)
		require.Contains(t, string(encoded), `{"name":"admin.appoint","owner_only":true}`)
		require.Contains(t, string(encoded), `{"name":"role.manage","owner_only":false}`)
	})
}
