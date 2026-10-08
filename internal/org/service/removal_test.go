package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md §3): taking a member
// out of the organization.

// accountExists reports whether a user row is still live.
func accountExists(t *testing.T, db *gorm.DB, userID int) bool {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", userID).Count(&n).Error)
	return n == 1
}

// Acceptance: 成员被移出组织时，其名下的组织 key 全部按回收处理（冻结并挂回 owner
// 名下），历史用量保留.
func TestRemoveMember_TakesTheirKeysBackAndDeletesTheAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		dropped := watchKeyCache(t, true)
		leaver := f.managerOf(t, db, "leaver", f.sales.Id)
		require.NoError(t, UpdateMember(db, f.owner, leaver.UserId, MemberPatch{ManagedDepartmentIds: intsPtr(f.product.Id)}))
		first := seedKey(t, db, f.org, leaver.UserId, "their first")
		second := withKeyStatus(t, db, seedKey(t, db, f.org, leaver.UserId, "their second").Id, common.TokenStatusDisabled)
		// What must not be touched: somebody else's key, a key already parked,
		// and a personal key the leaver made before they joined.
		alices := seedKey(t, db, f.org, f.alice.Id, "alice's")
		parked := seedKey(t, db, f.org, f.org.owner.Id, "parked")
		personal := platformmodel.Token{UserId: leaver.UserId, Name: "personal", Key: "personal-key-of-the-leaver", Status: common.TokenStatusEnabled}
		require.NoError(t, db.Create(&personal).Error)
		personal = reloadKey(t, db, personal.Id)
		records := len(auditRecords(t, db, f.org.id))

		reclaimed, err := RemoveMember(db, f.owner, leaver.UserId)
		require.NoError(t, err)
		require.Equal(t, 2, reclaimed)

		// Every key they held: under the owner, frozen, with a new value — and
		// what it spent still on it.
		for _, key := range []platformmodel.Token{first, second} {
			after := reloadKey(t, db, key.Id)
			expected := key
			expected.UserId, expected.Status, expected.Key = f.org.owner.Id, common.TokenStatusDisabled, after.Key
			require.Equal(t, expected, after, key.Name)
			require.NotEqual(t, key.Key, after.Key, key.Name)
			require.Zero(t, keysWithValue(t, db, key.Key), "%s: the value in their tools must be dead", key.Name)
			require.Equal(t, 250, after.UsedQuota, key.Name)
			require.GreaterOrEqual(t, dropped(key.Key), 1, "%s: the gateway must forget it before this answers", key.Name)
		}
		require.Equal(t, alices, reloadKey(t, db, alices.Id))
		require.Equal(t, parked, reloadKey(t, db, parked.Id))
		require.Equal(t, personal, reloadKey(t, db, personal.Id))

		// The account is gone: no member, no actor, nothing to manage. The row
		// stays, so the log can go on saying who it was.
		require.False(t, accountExists(t, db, leaver.UserId))
		var kept platformmodel.User
		require.NoError(t, db.Unscoped().First(&kept, leaver.UserId).Error)
		require.True(t, kept.DeletedAt.Valid)
		require.Equal(t, f.org.id, kept.OrgId)
		require.Equal(t, common.RoleCommonUser, kept.Role)
		require.Empty(t, managerRows(t, db, leaver.UserId))
		_, err = LoadActor(db, leaver.UserId)
		require.ErrorIs(t, err, ErrNotMember, "a session they still hold opens nothing")
		membership, err := GetMembership(db, leaver.UserId)
		require.NoError(t, err)
		require.Nil(t, membership)
		members, err := ListMembers(db, f.owner)
		require.NoError(t, err)
		for _, member := range members {
			require.NotEqual(t, leaver.UserId, member.Id)
		}
		_, err = AssignKey(db, f.owner, alices.Id, leaver.UserId)
		require.ErrorIs(t, err, ErrMemberNotFound, "nor can a key be handed to them again")

		// One record per key taken back, then the removal itself.
		after := auditRecords(t, db, f.org.id)
		require.Len(t, after, records+3)
		for i, key := range []platformmodel.Token{first, second} {
			record := after[records+i]
			require.Equal(t, orgmodel.AuditKeyReclaim, record.Action)
			require.Equal(t, key.Id, record.TargetId)
			require.Equal(t, f.owner.UserId, record.ActorUserId)
			was, is := handover(t, record)
			require.Equal(t, leaver.UserId, was.HolderId)
			require.Equal(t, "leaver", was.Holder)
			require.Equal(t, "Sales", was.Department)
			require.Equal(t, key.Status, was.Status)
			require.Equal(t, f.org.owner.Id, is.HolderId)
			require.Equal(t, common.TokenStatusDisabled, is.Status)
			require.NotEqual(t, was.Key, is.Key)
		}
		removal := after[records+2]
		require.Equal(t, orgmodel.AuditMemberRemove, removal.Action)
		require.Equal(t, orgmodel.AuditTargetMember, removal.TargetType)
		require.Equal(t, leaver.UserId, removal.TargetId)
		require.Equal(t, f.owner.UserId, removal.ActorUserId)
		require.Equal(t, testIP, removal.Ip)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"name":"leaver","username":"leaver","role_id":%d,"role":"manager","department_id":%d,"department":"Sales","key_ids":[%d,%d]}}`,
			presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id, first.Id, second.Id), removal.Detail)
		requireNoKeyValueInAudit(t, db, f.org.id, first.Key, second.Key, reloadKey(t, db, first.Id).Key, reloadKey(t, db, second.Id).Key)

		// The audit log still names them, as actor and as holder.
		logs, _, err := ListAuditLogs(db, f.owner, AuditFilter{}, 0, 100)
		require.NoError(t, err)
		require.NotEmpty(t, logs)
		_, err = RemoveMember(db, f.owner, leaver.UserId)
		require.ErrorIs(t, err, ErrMemberNotFound, "they are removed once")
	})
}

// Acceptance (HR 场景): 持有含 key.delete/key.freeze/member.remove 的自定义角色，能清理
// 任意离职成员名下的 key，操作有审计日志.
func TestRemoveMember_IsHowHRCleansUpAfterALeaver(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		hr := f.memberWith(t, db, "hr", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "hr_ops")...)
		keys := []platformmodel.Token{
			seedKey(t, db, f.org, f.bob.Id, "bob's first"),
			seedKey(t, db, f.org, f.bob.Id, "bob's second"),
			seedKey(t, db, f.org, f.bob.Id, "bob's third"),
		}

		// HR is no key manager: they may not hand keys out or take one back by
		// itself. Removing the leaver is what cleans up — in one go, anywhere
		// in the company.
		_, err := ReclaimKey(db, hr, keys[0].Id)
		require.ErrorIs(t, err, ErrForbidden)
		reclaimed, err := RemoveMember(db, hr, f.bob.Id)
		require.NoError(t, err)
		require.Equal(t, 3, reclaimed)
		for _, key := range keys {
			after := reloadKey(t, db, key.Id)
			require.Equal(t, f.org.owner.Id, after.UserId)
			require.Equal(t, common.TokenStatusDisabled, after.Status)
			require.Zero(t, keysWithValue(t, db, key.Key))
		}
		require.False(t, accountExists(t, db, f.bob.Id))

		// The log names HR as the one who did it, key by key.
		var byHR []string
		for _, record := range auditRecords(t, db, f.org.id) {
			if record.ActorUserId == hr.UserId {
				byHR = append(byHR, record.Action)
			}
		}
		require.Equal(t, []string{orgmodel.AuditKeyReclaim, orgmodel.AuditKeyReclaim, orgmodel.AuditKeyReclaim, orgmodel.AuditMemberRemove}, byHR)

		// And what was taken back is theirs to delete for good (key.delete).
		require.NoError(t, DeleteKey(db, hr, keys[0].Id))
	})
}

// PRD D10, D19: the owner cannot be removed — and nobody removes themselves,
// so that leaving always has somebody else's name on it.
func TestRemoveMember_NeverTheOwnerAndNeverOneself(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		admin := actorFor(t, db, seedMember(t, db, f.org, "admin", orgmodel.RoleAdmin).Id)
		hr := f.memberWith(t, db, "hr", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "hr_ops")...)
		ownersKey := seedKey(t, db, f.org, f.org.owner.Id, "the owner's")
		before := orgRowCounts(t, db, f.org.id)

		for name, actor := range map[string]*Actor{"the owner": f.owner, "an admin": admin, "HR": hr} {
			_, err := RemoveMember(db, actor, f.org.owner.Id)
			require.ErrorIs(t, err, ErrOwnerNotRemovable, name)
		}
		for name, actor := range map[string]*Actor{"an admin": admin, "HR": hr} {
			_, err := RemoveMember(db, actor, actor.UserId)
			require.ErrorIs(t, err, ErrRemoveSelf, name)
		}
		require.Equal(t, before, orgRowCounts(t, db, f.org.id))
		require.Equal(t, ownersKey, reloadKey(t, db, ownersKey.Id))
		require.True(t, accountExists(t, db, f.org.owner.Id))
		require.True(t, accountExists(t, db, admin.UserId))
	})
}

// PRD D10: appointing and dismissing admins is the owner's alone — so removing
// one is too. This is also what keeps a department-scoped role that manages
// the default department, where the owner and the admins sit (D26), from
// reaching them.
func TestRemoveMember_OnlyTheOwnerRemovesAnAdmin(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		admin := seedMember(t, db, f.org, "admin", orgmodel.RoleAdmin)
		colleague := actorFor(t, db, seedMember(t, db, f.org, "colleague", orgmodel.RoleAdmin).Id)
		hr := f.memberWith(t, db, "hr", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "hr_ops")...)
		// A role that removes members of the departments it manages, sitting in
		// the default department itself.
		usher := f.memberWith(t, db, "usher", orgmodel.ScopeDept, f.general.Id, "member.read", "member.remove")
		adminsKey := seedKey(t, db, f.org, admin.Id, "the admin's")
		before := orgRowCounts(t, db, f.org.id)

		for name, actor := range map[string]*Actor{"another admin": colleague, "HR": hr, "a role managing their department": usher} {
			_, err := RemoveMember(db, actor, admin.Id)
			require.ErrorIs(t, err, ErrOwnerOnly, name)
		}
		_, err := RemoveMember(db, usher, f.org.owner.Id)
		require.ErrorIs(t, err, ErrOwnerNotRemovable)
		require.Equal(t, before, orgRowCounts(t, db, f.org.id))
		require.Equal(t, adminsKey, reloadKey(t, db, adminsKey.Id))

		// The usher does remove an ordinary member of that department.
		clerk := seedMember(t, db, f.org, "clerk", orgmodel.RoleStaff)
		_, err = RemoveMember(db, usher, clerk.Id)
		require.NoError(t, err)
		require.False(t, accountExists(t, db, clerk.Id))

		// The owner dismisses the admin by removing them, keys and all.
		reclaimed, err := RemoveMember(db, f.owner, admin.Id)
		require.NoError(t, err)
		require.Equal(t, 1, reclaimed)
		require.False(t, accountExists(t, db, admin.Id))
		require.Equal(t, f.org.owner.Id, reloadKey(t, db, adminsKey.Id).UserId)
	})
}

// Acceptance (the removal part of it): 部门作用域硬检查：manager 类角色对非所管部门的
// 成员/key/报表操作一律 403.
func TestRemoveMember_ADepartmentScopedRoleStaysInsideItsDepartments(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		usher := f.memberWith(t, db, "usher", orgmodel.ScopeDept, f.sales.Id, "member.read", "member.remove")
		bobsKey := seedKey(t, db, f.org, f.bob.Id, "bob's")
		before := orgRowCounts(t, db, f.org.id)

		_, err := RemoveMember(db, usher, f.bob.Id)
		require.ErrorIs(t, err, ErrForbidden, "Product is not theirs")
		require.Equal(t, before, orgRowCounts(t, db, f.org.id))
		require.Equal(t, bobsKey, reloadKey(t, db, bobsKey.Id))
		require.True(t, accountExists(t, db, f.bob.Id))

		_, err = RemoveMember(db, usher, f.alice.Id)
		require.NoError(t, err)
		require.False(t, accountExists(t, db, f.alice.Id))

		// Whoever may remove nobody is told so before anything is looked up:
		// the answer does not depend on who the target is, or whether they are.
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		readonly := actorFor(t, db, seedMember(t, db, f.org, "readonly", orgmodel.RoleReadonly).Id)
		for _, actor := range []*Actor{manager, readonly, actorFor(t, db, f.bob.Id), actorFor(t, db, f.bot.Id)} {
			for target, memberID := range map[string]int{"the owner": f.org.owner.Id, "a colleague": f.bob.Id, "themselves": actor.UserId, "nobody": 424242} {
				_, err := RemoveMember(db, actor, memberID)
				require.ErrorIs(t, err, ErrForbidden, target)
			}
		}
	})
}

// A service account is a member that is not a person: removing it takes its
// keys back and deletes it — there is nobody to hand the account back to.
func TestRemoveMember_DeletesAServiceAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.bot.Id, "the pipeline's")

		reclaimed, err := RemoveMember(db, f.owner, f.bot.Id)
		require.NoError(t, err)
		require.Equal(t, 1, reclaimed)
		require.False(t, accountExists(t, db, f.bot.Id))
		after := reloadKey(t, db, key.Id)
		require.Equal(t, f.org.owner.Id, after.UserId)
		require.Equal(t, common.TokenStatusDisabled, after.Status)
		require.Zero(t, keysWithValue(t, db, key.Key), "the value the pipeline runs on is dead")
		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditMemberRemove, record.Action)
		require.Contains(t, record.Detail, `"is_service":true`)
		require.Contains(t, record.Detail, `"name":"CI"`)

		// A member with no keys leaves exactly one record behind.
		records := len(auditRecords(t, db, f.org.id))
		reclaimed, err = RemoveMember(db, f.owner, f.alice.Id)
		require.NoError(t, err)
		require.Zero(t, reclaimed)
		require.Len(t, auditRecords(t, db, f.org.id), records+1)
		require.Contains(t, lastAudit(t, db, f.org.id).Detail, `"key_ids":[]`)
	})
}

// A member of another company cannot be removed from here, and removing one of
// ours leaves theirs alone.
func TestRemoveMember_StaysInsideTheActorsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		theirs := seedKey(t, db, other.org, other.alice.Id, "theirs")
		before := orgRowCounts(t, db, other.org.id)

		_, err := RemoveMember(db, acme.owner, other.alice.Id)
		require.ErrorIs(t, err, ErrMemberNotFound)
		_, err = RemoveMember(db, acme.owner, other.org.owner.Id)
		require.ErrorIs(t, err, ErrMemberNotFound)
		_, err = RemoveMember(db, acme.owner, acme.alice.Id)
		require.NoError(t, err)

		require.Equal(t, before, orgRowCounts(t, db, other.org.id))
		require.Equal(t, theirs, reloadKey(t, db, theirs.Id))
		require.True(t, accountExists(t, db, other.alice.Id))
	})
}

// 🔴 Removal writes org columns and the row's deleted_at — never users.role.
func TestRemoveMember_NeverWritesTheGlobalRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		_, err := RemoveMember(db, f.owner, f.alice.Id)
		require.NoError(t, err)
		var users []platformmodel.User
		require.NoError(t, db.Unscoped().Find(&users).Error)
		for _, user := range users {
			require.Equal(t, common.RoleCommonUser, user.Role, user.Username)
		}
	})
}

// The member list says how many keys each member holds — what removing them
// would take back.
func TestListMembers_CountsTheKeysEachHolds(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		seedKey(t, db, f.org, f.alice.Id, "alice's first")
		seedKey(t, db, f.org, f.alice.Id, "alice's second")
		seedKey(t, db, f.org, f.bot.Id, "the pipeline's")
		gone := seedKey(t, db, f.org, f.bob.Id, "bob's, deleted")
		require.NoError(t, DeleteKey(db, f.owner, gone.Id))
		// Not the organization's: a personal key of a member, another company's.
		require.NoError(t, db.Create(&platformmodel.Token{UserId: f.bob.Id, Name: "personal", Key: "personal-key-of-bob"}).Error)
		seedKey(t, db, other.org, other.alice.Id, "theirs")

		members, err := ListMembers(db, f.owner)
		require.NoError(t, err)
		counts := map[int]int{}
		for _, member := range members {
			counts[member.Id] = member.KeyCount
		}
		require.Equal(t, map[int]int{f.org.owner.Id: 0, f.alice.Id: 2, f.bob.Id: 0, f.bot.Id: 1}, counts)
	})
}
