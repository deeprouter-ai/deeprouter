package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md §3): handing a key to
// another holder and taking it back. Who may do either is pinned in
// gate_test.go too; this file is about what the two of them do.

// managerOf adds a member holding the preset manager role in a department, and
// returns them ready to act.
func (f keysFixture) managerOf(t *testing.T, db *gorm.DB, username string, departmentID int) *Actor {
	t.Helper()
	member := seedMemberIn(t, db, f.org, username, presetRoleID(t, db, orgmodel.RoleManager), departmentID)
	return actorFor(t, db, member.Id)
}

// staffIn adds a person with the staff role to a department.
func (f keysFixture) staffIn(t *testing.T, db *gorm.DB, username string, departmentID int) platformmodel.User {
	t.Helper()
	return seedMemberIn(t, db, f.org, username, presetRoleID(t, db, orgmodel.RoleStaff), departmentID)
}

// withKeyStatus writes a key's status straight into the table and returns the
// row as it is then.
func withKeyStatus(t *testing.T, db *gorm.DB, keyID int, status int) platformmodel.Token {
	t.Helper()
	require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", keyID).Update("status", status).Error)
	return reloadKey(t, db, keyID)
}

// handover reads the two sides of a key.assign or key.reclaim record.
func handover(t *testing.T, record orgmodel.OrgAuditLog) (before keyHandover, after keyHandover) {
	t.Helper()
	require.Contains(t, []string{orgmodel.AuditKeyAssign, orgmodel.AuditKeyReclaim}, record.Action)
	var detail struct {
		Before keyHandover `json:"before"`
		After  keyHandover `json:"after"`
	}
	require.NoError(t, common.UnmarshalJsonStr(record.Detail, &detail))
	return detail.Before, detail.After
}

// Acceptance: 分配/回收：admin 可分配给组织内任意成员……再次分配时自动换值.
func TestAssignKey_HandsTheKeyOverWithANewValue(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		admin := actorFor(t, db, seedMember(t, db, f.org, "admin", orgmodel.RoleAdmin).Id)
		created, err := CreateKey(db, f.owner, KeyInput{
			Name: "Design", HolderId: f.alice.Id, PolicyTemplate: "creative", RemainQuota: 5000, RpmLimit: 60, MonthlyLimit: 3000,
		}, fixedModels(testCatalogue))
		require.NoError(t, err)
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", created.Id).
			Updates(map[string]any{"remain_quota": 4200, "used_quota": 800}).Error)
		before := reloadKey(t, db, created.Id)

		grant, err := AssignKey(db, admin, created.Id, f.bob.Id)
		require.NoError(t, err)

		after := reloadKey(t, db, created.Id)
		// The holder and the value change; the settings, the template and what
		// the key has spent do not.
		expected := before
		expected.UserId, expected.Key = f.bob.Id, after.Key
		require.Equal(t, expected, after)
		require.NotEqual(t, before.Key, after.Key)
		require.Len(t, after.Key, 48)
		require.Zero(t, keysWithValue(t, db, before.Key), "what the old holder has in their tools must be dead at once")
		require.Equal(t, 800, after.UsedQuota)

		// The new holder is a person: the value is not shown, they take it
		// through one-click setup.
		require.Empty(t, grant.Value)
		require.Equal(t, platformmodel.MaskTokenKey(after.Key), grant.Key)
		require.Equal(t, f.bob.Id, grant.HolderId)
		require.Equal(t, "bob-of-Acme", grant.Holder)
		require.Equal(t, "Product", grant.Department)
		require.False(t, grant.HolderIsOwner)
		require.Equal(t, common.TokenStatusEnabled, grant.Status)
		encoded, err := common.Marshal(grant)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), after.Key)
		require.NotContains(t, string(encoded), `"value"`)

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyAssign, record.Action)
		require.Equal(t, orgmodel.AuditTargetKey, record.TargetType)
		require.Equal(t, created.Id, record.TargetId)
		require.Equal(t, admin.UserId, record.ActorUserId)
		require.Equal(t, testIP, record.Ip)
		require.JSONEq(t, fmt.Sprintf(`{
			"before":{"name":"Design","holder_id":%d,"holder":"alice-of-Acme","department_id":%d,"department":"Sales","status":1,"key":%q},
			"after":{"name":"Design","holder_id":%d,"holder":"bob-of-Acme","department_id":%d,"department":"Product","status":1,"key":%q}}`,
			f.alice.Id, f.sales.Id, platformmodel.MaskTokenKey(before.Key),
			f.bob.Id, f.product.Id, platformmodel.MaskTokenKey(after.Key)), record.Detail,
			"who had it, who has it, and the two values — masked")

		// An admin hands a key to anyone in the organization, themselves
		// included, and every hand it passes through gets a value of its own.
		values := []string{before.Key, after.Key}
		for _, holderID := range []int{f.alice.Id, admin.UserId, f.bob.Id} {
			_, err := AssignKey(db, admin, created.Id, holderID)
			require.NoError(t, err)
			moved := reloadKey(t, db, created.Id)
			require.Equal(t, holderID, moved.UserId)
			require.NotContains(t, values, moved.Key, "a value is never handed out twice")
			values = append(values, moved.Key)
		}
		requireNoKeyValueInAudit(t, db, f.org.id, values...)
		require.EqualValues(t, 1, keyRows(t, db, f.org.id), "the row moves; no key is made or lost")
	})
}

// Acceptance: manager 只能分配给本部门 staff，跨部门 403. A manager moves the keys
// held in the departments they manage, to people of those departments — they
// neither reach a key held elsewhere nor one parked under the owner (decided
// by @sam on 2026-10-07).
func TestAssignKey_AManagerStaysInsideTheirDepartments(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		carol := f.staffIn(t, db, "carol", f.sales.Id)
		inSales := seedKey(t, db, f.org, f.alice.Id, "held in Sales")
		inProduct := seedKey(t, db, f.org, f.bob.Id, "held in Product")
		parked := seedKey(t, db, f.org, f.org.owner.Id, "parked")

		// Inside Sales: to a colleague, to the department's service account, to
		// the manager themselves.
		for _, holderID := range []int{carol.Id, f.bot.Id, manager.UserId, f.alice.Id} {
			_, err := AssignKey(db, manager, inSales.Id, holderID)
			require.NoError(t, err)
			require.Equal(t, holderID, reloadKey(t, db, inSales.Id).UserId)
		}

		// Across the department line, either way, and from the pile nobody has
		// been handed: refused, and nothing moves.
		inSales = reloadKey(t, db, inSales.Id)
		records := len(auditRecords(t, db, f.org.id))
		for name, try := range map[string]struct{ keyID, holderID int }{
			"a Sales key to someone in Product":  {inSales.Id, f.bob.Id},
			"a Product key to someone in Sales":  {inProduct.Id, f.alice.Id},
			"a Product key to someone else":      {inProduct.Id, f.bob.Id},
			"a parked key to someone in Sales":   {parked.Id, f.alice.Id},
			"a parked key to themselves":         {parked.Id, manager.UserId},
			"a Sales key to the owner's deputy":  {inSales.Id, seedMember(t, db, f.org, "admin", orgmodel.RoleAdmin).Id},
			"a Sales key to a member of nowhere": {inSales.Id, 424242},
		} {
			_, err := AssignKey(db, manager, try.keyID, try.holderID)
			if try.holderID == 424242 {
				require.ErrorIs(t, err, ErrMemberNotFound, name)
			} else {
				require.ErrorIs(t, err, ErrForbidden, name)
			}
		}
		require.Equal(t, inSales, reloadKey(t, db, inSales.Id))
		require.Equal(t, inProduct, reloadKey(t, db, inProduct.Id))
		require.Equal(t, parked, reloadKey(t, db, parked.Id))
		require.Len(t, auditRecords(t, db, f.org.id), records, "a refusal records nothing")

		// Reach follows the departments: given Product as well, the manager
		// moves keys between the two.
		require.NoError(t, UpdateMember(db, f.owner, manager.UserId, MemberPatch{ManagedDepartmentIds: intsPtr(f.product.Id)}))
		manager = actorFor(t, db, manager.UserId)
		_, err := AssignKey(db, manager, inSales.Id, f.bob.Id)
		require.NoError(t, err)
		_, err = AssignKey(db, manager, inProduct.Id, f.alice.Id)
		require.NoError(t, err)
		_, err = AssignKey(db, manager, parked.Id, f.alice.Id)
		require.ErrorIs(t, err, ErrForbidden, "the owner sits in neither")
	})
}

// PRD D15: 服务账号的 key 在归到服务账号名下的那一刻向操作者展示一次——后来分配给它的，
// 自动换值.
func TestAssignKey_ShowsTheNewValueOnlyForAServiceAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")

		grant, err := AssignKey(db, f.owner, key.Id, f.bot.Id)
		require.NoError(t, err)
		after := reloadKey(t, db, key.Id)
		require.Equal(t, after.Key, grant.Value, "a service account cannot sign in to fetch it")
		require.NotEqual(t, key.Key, grant.Value, "and it is not the value its last holder has")
		require.Equal(t, platformmodel.MaskTokenKey(after.Key), grant.Key)
		require.True(t, grant.HolderIsService)
		_, recorded := handover(t, lastAudit(t, db, f.org.id))
		require.True(t, recorded.ValueShown, "the log says the value was seen")

		// From the service account to a person: nothing is shown, and the log
		// does not claim otherwise.
		grant, err = AssignKey(db, f.owner, key.Id, f.bob.Id)
		require.NoError(t, err)
		require.Empty(t, grant.Value)
		require.NotContains(t, lastAudit(t, db, f.org.id).Detail, "value_shown")
		requireNoKeyValueInAudit(t, db, f.org.id, key.Key, after.Key, reloadKey(t, db, key.Id).Key)
	})
}

// A key that is frozen when it is handed over works for its new holder only if
// whoever hands it over could have unfrozen it there themselves.
func TestAssignKey_UnfreezesOnlyForWhoMayUnfreeze(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		carol := f.staffIn(t, db, "carol", f.sales.Id)
		frozen := func(name string) platformmodel.Token {
			return withKeyStatus(t, db, seedKey(t, db, f.org, f.alice.Id, name).Id, common.TokenStatusDisabled)
		}
		statusAfter := func(actor *Actor, keyID int, holderID int) int {
			grant, err := AssignKey(db, actor, keyID, holderID)
			require.NoError(t, err)
			require.Equal(t, grant.Status, reloadKey(t, db, keyID).Status, "the answer says what the key is now")
			return grant.Status
		}

		// The owner holds key.freeze: the key comes out working, and the
		// record shows that it did.
		key := frozen("for the owner to hand over")
		require.Equal(t, common.TokenStatusEnabled, statusAfter(f.owner, key.Id, carol.Id))
		was, is := handover(t, lastAudit(t, db, f.org.id))
		require.Equal(t, common.TokenStatusDisabled, was.Status)
		require.Equal(t, common.TokenStatusEnabled, is.Status)

		// The manager holds key.assign and not key.freeze: the key moves and
		// stays frozen. Handing a key over is not a way around a freeze.
		key = frozen("for the manager to hand over")
		require.Equal(t, common.TokenStatusDisabled, statusAfter(manager, key.Id, carol.Id))
		require.Equal(t, carol.Id, reloadKey(t, db, key.Id).UserId)

		// It is the permission that counts, not being the owner: a custom role
		// that holds both, over both departments, hands the key over working.
		mover := f.memberWith(t, db, "mover", orgmodel.ScopeDept, f.product.Id, "key.assign", "key.freeze")
		require.NoError(t, UpdateMember(db, f.owner, mover.UserId, MemberPatch{ManagedDepartmentIds: intsPtr(f.sales.Id)}))
		mover = actorFor(t, db, mover.UserId)
		key = frozen("for a role that holds both")
		require.Equal(t, common.TokenStatusEnabled, statusAfter(mover, key.Id, f.bob.Id))

		// What would stop an unfreeze stops this one too: the key moves, and
		// waits for its expiry or its quota to be put right.
		expired := frozen("expired")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", expired.Id).Update("expired_time", time.Now().Add(-time.Hour).Unix()).Error)
		require.Equal(t, common.TokenStatusDisabled, statusAfter(f.owner, expired.Id, carol.Id))
		exhausted := frozen("exhausted")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", exhausted.Id).Update("remain_quota", 0).Error)
		require.Equal(t, common.TokenStatusDisabled, statusAfter(f.owner, exhausted.Id, carol.Id))

		// A key that works goes on working, whoever hands it over; one the
		// gateway marked expired or exhausted keeps that mark.
		working := seedKey(t, db, f.org, f.alice.Id, "working")
		require.Equal(t, common.TokenStatusEnabled, statusAfter(manager, working.Id, carol.Id))
		for _, status := range []int{common.TokenStatusExpired, common.TokenStatusExhausted} {
			marked := withKeyStatus(t, db, seedKey(t, db, f.org, f.alice.Id, "marked").Id, status)
			require.Equal(t, status, statusAfter(f.owner, marked.Id, carol.Id))
		}
	})
}

// Handing a key to whoever holds it already is nothing: the value in their
// tools goes on working, and the log stays as it was.
func TestAssignKey_ToItsHolderChangesNothing(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		dropped := watchKeyCache(t, true)
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")
		records := len(auditRecords(t, db, f.org.id))

		grant, err := AssignKey(db, f.owner, key.Id, f.alice.Id)
		require.NoError(t, err)
		require.Equal(t, key, reloadKey(t, db, key.Id))
		require.Equal(t, key.Id, grant.Id)
		require.Equal(t, f.alice.Id, grant.HolderId)
		require.Empty(t, grant.Value)
		require.Len(t, auditRecords(t, db, f.org.id), records)
		require.Zero(t, dropped(key.Key))

		// The same goes for a service account's key: its value is not shown
		// again by "handing" it to the account that has it.
		bots := seedKey(t, db, f.org, f.bot.Id, "the pipeline's")
		grant, err = AssignKey(db, f.owner, bots.Id, f.bot.Id)
		require.NoError(t, err)
		require.Empty(t, grant.Value)
		require.Equal(t, bots, reloadKey(t, db, bots.Id))
	})
}

// The owner holds what has been handed to nobody, so "assigning" a key to the
// owner takes it back — frozen, like any reclaim, not handed over in working
// order.
func TestAssignKey_ToTheOwnerTakesItBack(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")

		// Taking back asks for reach over the holder alone, so the manager
		// may — although the owner is nobody they could hand a key to.
		grant, err := AssignKey(db, manager, key.Id, f.org.owner.Id)
		require.NoError(t, err)
		after := reloadKey(t, db, key.Id)
		require.Equal(t, f.org.owner.Id, after.UserId)
		require.Equal(t, common.TokenStatusDisabled, after.Status)
		require.NotEqual(t, key.Key, after.Key)
		require.True(t, grant.HolderIsOwner)
		require.Empty(t, grant.Value)
		require.Equal(t, orgmodel.AuditKeyReclaim, lastAudit(t, db, f.org.id).Action)
	})
}

// One account holds only so many keys; being handed one counts like having one
// made.
func TestAssignKey_StopsAtTheMostKeysOneAccountMayHold(t *testing.T) {
	setting := operation_setting.GetTokenSetting()
	previous := setting.MaxUserTokens
	setting.MaxUserTokens = 2
	t.Cleanup(func() { setting.MaxUserTokens = previous })

	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		seedKey(t, db, f.org, f.bob.Id, "bob's first")
		seedKey(t, db, f.org, f.bob.Id, "bob's second")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")
		records := len(auditRecords(t, db, f.org.id))

		_, err := AssignKey(db, f.owner, key.Id, f.bob.Id)
		require.ErrorIs(t, err, ErrKeyLimitReached)
		require.Equal(t, key, reloadKey(t, db, key.Id), "the key stays where it was, value and all")
		require.Len(t, auditRecords(t, db, f.org.id), records)

		_, err = AssignKey(db, f.owner, key.Id, f.bot.Id)
		require.NoError(t, err, "the limit is each holder's own")
	})
}

// Neither a key nor a member of another company exists for this one.
func TestAssignKey_StaysInsideTheActorsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		ours := seedKey(t, db, acme.org, acme.alice.Id, "ours")
		theirs := seedKey(t, db, other.org, other.alice.Id, "theirs")
		before := orgRowCounts(t, db, other.org.id)

		_, err := AssignKey(db, acme.owner, ours.Id, other.bob.Id)
		require.ErrorIs(t, err, ErrMemberNotFound, "our key to their member")
		_, err = AssignKey(db, acme.owner, theirs.Id, acme.bob.Id)
		require.ErrorIs(t, err, ErrKeyNotFound, "their key to our member")
		_, err = AssignKey(db, acme.owner, theirs.Id, other.bob.Id)
		require.ErrorIs(t, err, ErrKeyNotFound, "their key to their member")
		_, err = ReclaimKey(db, acme.owner, theirs.Id)
		require.ErrorIs(t, err, ErrKeyNotFound)
		_, err = AssignKey(db, acme.owner, 424242, acme.bob.Id)
		require.ErrorIs(t, err, ErrKeyNotFound)

		require.Equal(t, ours, reloadKey(t, db, ours.Id))
		require.Equal(t, theirs, reloadKey(t, db, theirs.Id))
		require.Equal(t, before, orgRowCounts(t, db, other.org.id))
	})
}

// Acceptance: 回收后 key 自动冻结并挂回 owner 名下，再次分配时自动换值.
func TestReclaimKey_FreezesTheKeyAndParksItUnderTheOwner(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		created, err := CreateKey(db, f.owner, KeyInput{
			Name: "Design", HolderId: f.alice.Id, PolicyTemplate: "creative", RemainQuota: 5000, RpmLimit: 60,
		}, fixedModels(testCatalogue))
		require.NoError(t, err)
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", created.Id).
			Updates(map[string]any{"remain_quota": 4200, "used_quota": 800}).Error)
		before := reloadKey(t, db, created.Id)

		view, err := ReclaimKey(db, f.owner, created.Id)
		require.NoError(t, err)

		parked := reloadKey(t, db, created.Id)
		expected := before
		expected.UserId, expected.Status, expected.Key = f.org.owner.Id, common.TokenStatusDisabled, parked.Key
		require.Equal(t, expected, parked, "frozen, under the owner, with a new value — and nothing else touched")
		require.NotEqual(t, before.Key, parked.Key)
		require.Zero(t, keysWithValue(t, db, before.Key), "the value in the old holder's tools is gone for good")
		require.Equal(t, 800, parked.UsedQuota, "what it spent stays on it")

		require.Equal(t, f.org.owner.Id, view.HolderId)
		require.True(t, view.HolderIsOwner)
		require.Equal(t, f.org.owner.Username, view.Holder)
		require.Equal(t, "General", view.Department)
		require.Equal(t, common.TokenStatusDisabled, view.Status)
		require.Equal(t, platformmodel.MaskTokenKey(parked.Key), view.Key)

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyReclaim, record.Action)
		require.Equal(t, created.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{
			"before":{"name":"Design","holder_id":%d,"holder":"alice-of-Acme","department_id":%d,"department":"Sales","status":1,"key":%q},
			"after":{"name":"Design","holder_id":%d,"holder":%q,"department_id":%d,"department":"General","status":2,"key":%q}}`,
			f.alice.Id, f.sales.Id, platformmodel.MaskTokenKey(before.Key),
			f.org.owner.Id, f.org.owner.Username, f.general.Id, platformmodel.MaskTokenKey(parked.Key)), record.Detail)

		// Unfreezing the parked key wakes the new value, never the one its old
		// holder still has.
		require.NoError(t, UnfreezeKey(db, f.owner, created.Id))
		require.Equal(t, parked.Key, reloadKey(t, db, created.Id).Key)
		require.Zero(t, keysWithValue(t, db, before.Key))
		require.NoError(t, FreezeKey(db, f.owner, created.Id))

		// Handed out again, it gets a third value and works.
		grant, err := AssignKey(db, f.owner, created.Id, f.bob.Id)
		require.NoError(t, err)
		again := reloadKey(t, db, created.Id)
		require.NotContains(t, []string{before.Key, parked.Key}, again.Key)
		require.Equal(t, f.bob.Id, again.UserId)
		require.Equal(t, common.TokenStatusEnabled, again.Status)
		require.Equal(t, common.TokenStatusEnabled, grant.Status)
		require.Equal(t, 800, again.UsedQuota)
		requireNoKeyValueInAudit(t, db, f.org.id, before.Key, parked.Key, again.Key)
	})
}

// A key comes back whatever the state of the place it returns to: the owner
// may already hold as many keys as one account can.
func TestReclaimKey_IsNotStoppedByHowManyKeysTheOwnerHolds(t *testing.T) {
	setting := operation_setting.GetTokenSetting()
	previous := setting.MaxUserTokens
	setting.MaxUserTokens = 2
	t.Cleanup(func() { setting.MaxUserTokens = previous })

	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		seedKey(t, db, f.org, f.org.owner.Id, "parked first")
		seedKey(t, db, f.org, f.org.owner.Id, "parked second")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")

		_, err := ReclaimKey(db, f.owner, key.Id)
		require.NoError(t, err)
		require.Equal(t, f.org.owner.Id, reloadKey(t, db, key.Id).UserId)
	})
}

// A key parked under the owner has been handed to nobody: there is nothing to
// take back, and the owner's working key is not frozen by trying.
func TestReclaimKey_OfAParkedKeyChangesNothing(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		dropped := watchKeyCache(t, true)
		parked := seedKey(t, db, f.org, f.org.owner.Id, "parked")
		records := len(auditRecords(t, db, f.org.id))

		view, err := ReclaimKey(db, f.owner, parked.Id)
		require.NoError(t, err)
		require.Equal(t, parked, reloadKey(t, db, parked.Id))
		require.Equal(t, common.TokenStatusEnabled, view.Status)
		require.True(t, view.HolderIsOwner)
		require.Len(t, auditRecords(t, db, f.org.id), records)
		require.Zero(t, dropped(parked.Key))
	})
}

// Taking a key back takes reach over whoever holds it, and nothing else.
func TestReclaimKey_AManagerTakesBackWhatTheirDepartmentsHold(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		inSales := seedKey(t, db, f.org, f.alice.Id, "held in Sales")
		bots := seedKey(t, db, f.org, f.bot.Id, "the pipeline's")
		inProduct := seedKey(t, db, f.org, f.bob.Id, "held in Product")

		for _, key := range []platformmodel.Token{inSales, bots} {
			_, err := ReclaimKey(db, manager, key.Id)
			require.NoError(t, err)
			after := reloadKey(t, db, key.Id)
			require.Equal(t, f.org.owner.Id, after.UserId)
			require.Equal(t, common.TokenStatusDisabled, after.Status)
		}
		_, err := ReclaimKey(db, manager, inProduct.Id)
		require.ErrorIs(t, err, ErrForbidden)
		require.Equal(t, inProduct, reloadKey(t, db, inProduct.Id))

		// What the manager took back is out of their hands: it sits with the
		// owner now, in a department they do not manage.
		_, err = AssignKey(db, manager, inSales.Id, f.alice.Id)
		require.ErrorIs(t, err, ErrForbidden)
		listed, err := ListKeys(db, manager)
		require.NoError(t, err)
		require.Empty(t, listed)
	})
}

// The list a key is handed out from: whoever the actor may assign to, the
// owner left out, roles told only to whoever may read the members.
func TestListKeyAssignees_AreThePeopleAKeyCanBeHandedTo(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		newKeysFixture(t, db, "Other")
		manager := f.managerOf(t, db, "manager", f.sales.Id)
		mover := f.memberWith(t, db, "mover", orgmodel.ScopeDept, f.product.Id, "key.assign")
		staff := presetRoleID(t, db, orgmodel.RoleStaff)
		managerRole := presetRoleID(t, db, orgmodel.RoleManager)
		var moverRole orgmodel.OrgRole
		require.NoError(t, db.Where("name = ? AND org_id = ?", "Role of mover", f.org.id).First(&moverRole).Error)

		// The owner: everyone but themselves, each with their role.
		assignees, err := ListKeyAssignees(db, f.owner)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.alice.Id, Name: "alice-of-Acme", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff"},
			{Id: f.bob.Id, Name: "bob-of-Acme", DepartmentId: f.product.Id, Department: "Product", RoleId: staff, Role: "staff"},
			{Id: f.bot.Id, Name: "CI", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff", IsService: true},
			{Id: manager.UserId, Name: "manager", DepartmentId: f.sales.Id, Department: "Sales", RoleId: managerRole, Role: "manager"},
			{Id: mover.UserId, Name: "mover", DepartmentId: f.product.Id, Department: "Product", RoleId: moverRole.Id, Role: "Role of mover"},
		}, assignees)

		// The preset manager, who may also read the members of Sales: the
		// people of Sales, themselves among them.
		assignees, err = ListKeyAssignees(db, manager)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.alice.Id, Name: "alice-of-Acme", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff"},
			{Id: f.bot.Id, Name: "CI", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff", IsService: true},
			{Id: manager.UserId, Name: "manager", DepartmentId: f.sales.Id, Department: "Sales", RoleId: managerRole, Role: "manager"},
		}, assignees)

		// Assigning without the right to read members: names and departments.
		assignees, err = ListKeyAssignees(db, mover)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.bob.Id, Name: "bob-of-Acme", DepartmentId: f.product.Id, Department: "Product"},
			{Id: mover.UserId, Name: "mover", DepartmentId: f.product.Id, Department: "Product"},
		}, assignees)

		// Creating keys is not handing them out, and reading is neither.
		minter := f.memberWith(t, db, "minter", orgmodel.ScopeOrg, f.general.Id, "key.create")
		readonly := actorFor(t, db, seedMember(t, db, f.org, "readonly", orgmodel.RoleReadonly).Id)
		for _, actor := range []*Actor{minter, readonly, actorFor(t, db, f.alice.Id)} {
			_, err = ListKeyAssignees(db, actor)
			require.ErrorIs(t, err, ErrForbidden)
		}
	})
}
