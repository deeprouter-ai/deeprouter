package router

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestOrgHandover_ThroughTheRealRouter walks Enterprise Org P6 (meta-repo
// docs/enterprise-org-prd.md §3) end to end over the real routes, the real
// auth middleware and browser-style session cookies: the IT scenario (only a
// role that may create keys creates them), members who cannot make a key of
// their own, a key that changes hands and takes its old value from whoever had
// it, a manager who stops at the department line, and the HR scenario — a
// leaver removed, keys and account and all.
func TestOrgHandover_ThroughTheRealRouter(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		engine := newOrgEngine(t, db)
		SetConnectRouter(engine)
		id := strconv.Itoa
		// stored reads a key row straight from the database: where the value
		// lives that no endpoint below may say, except where the test expects it.
		stored := func(keyID int) model.Token {
			var key model.Token
			require.NoError(t, db.Unscoped().First(&key, keyID).Error)
			return key
		}

		owner := signUp(t, engine, map[string]any{"username": "founder", "password": "password123", "org_name": "Acme"}, "")
		var departments []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		owner.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		departmentID := map[string]int{}
		for _, d := range departments {
			departmentID[d.Name] = d.Id
		}
		var roles []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		owner.ok(t, http.MethodGet, "/api/org/roles", nil, &roles)
		roleID := map[string]int{}
		for _, r := range roles {
			roleID[r.Name] = r.Id
		}
		join := func(username string, roleName string, departmentName string) browser {
			var invite struct {
				Code string `json:"code"`
			}
			owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[roleName], "department_id": departmentID[departmentName]}, &invite)
			return signUp(t, engine, map[string]any{"username": username, "password": "password123", "org_invite": invite.Code}, "")
		}
		// adopt copies a role pack into the organization and gives it to a member.
		adopt := func(pack string, name string, member browser) {
			var role struct {
				Id int `json:"id"`
			}
			owner.ok(t, http.MethodPost, "/api/org/role-packs/"+pack+"/adopt", map[string]any{"name": name}, &role)
			owner.ok(t, http.MethodPut, "/api/org/members/"+id(member.userID), map[string]any{"role_id": role.Id}, nil)
		}
		manager := join("manager", orgmodel.RoleManager, "Sales")
		seller := join("seller", orgmodel.RoleStaff, "Sales")
		closer := join("closer", orgmodel.RoleStaff, "Sales")
		builder := join("builder", orgmodel.RoleStaff, "Product")
		it := join("it", orgmodel.RoleStaff, "General")
		adopt("it_ops", "IT Ops", it)
		hr := join("hr", orgmodel.RoleStaff, "General")
		adopt("hr_ops", "HR Ops", hr)
		personal := signUp(t, engine, map[string]any{"username": "solo", "password": "password123"}, "")

		type key struct {
			Id            int    `json:"id"`
			Name          string `json:"name"`
			Key           string `json:"key"`
			Value         string `json:"value"`
			Status        int    `json:"status"`
			HolderId      int    `json:"holder_id"`
			Holder        string `json:"holder"`
			HolderIsOwner bool   `json:"holder_is_owner"`
		}
		keyNames := func(b browser) []string {
			var listed []key
			b.ok(t, http.MethodGet, "/api/org/keys", nil, &listed)
			names := []string{}
			for _, k := range listed {
				names = append(names, k.Name)
			}
			return names
		}

		// --- IT 场景：持有含 key.create 的角色能创建 key；无该权限的角色创建 key 返回 403 ---
		var sellersKey key
		it.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Sales tools", "holder_id": seller.userID, "unlimited_quota": true}, &sellersKey)
		require.Equal(t, "seller", sellersKey.Holder)
		for name, member := range map[string]browser{"the manager": manager, "HR": hr, "a seller": seller} {
			status, success, message, _ := member.call(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Mine", "unlimited_quota": true})
			require.Equal(t, http.StatusForbidden, status, name)
			require.False(t, success, name)
			require.Equal(t, "org.forbidden", message, name)
		}

		// --- 组织成员不能通过个人 key 端点自建 key ---------------------------------
		for name, member := range map[string]browser{"the owner": owner, "IT": it, "the manager": manager, "a seller": seller} {
			for _, body := range []map[string]any{
				{"name": "mine", "expired_time": -1, "unlimited_quota": true},
				{"name": "my-chat-key", "expired_time": -1, "unlimited_quota": true, "simple_purpose": "chat"},
			} {
				answer := member.raw(t, http.MethodPost, "/api/token/", body)
				require.Equal(t, http.StatusForbidden, answer.Code, name)
				require.Contains(t, answer.Body.String(), "org.member_personal_key", name)
				require.NotContains(t, answer.Body.String(), `"key"`, name)
			}
		}
		var tokens int64
		require.NoError(t, db.Model(&model.Token{}).Count(&tokens).Error)
		require.EqualValues(t, 1, tokens, "only the key IT made exists")
		// A personal account next door creates keys as it always did.
		personal.ok(t, http.MethodPost, "/api/token/", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}, nil)

		// --- handing a key on ------------------------------------------------------
		// The seller copies a one-click command for their key, and does not run
		// it yet.
		var pending struct {
			ScriptPath string `json:"script_path"`
		}
		seller.ok(t, http.MethodPost, "/api/connect/token", map[string]any{"key_id": sellersKey.Id, "tools": []string{"codex"}}, &pending)
		oldValue := stored(sellersKey.Id).Key

		// The manager hands the key to another seller: allowed inside Sales.
		handover := manager.raw(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/assign", map[string]any{"holder_id": closer.userID})
		require.Equal(t, http.StatusOK, handover.Code, handover.Body.String())
		require.Contains(t, handover.Header().Get("Cache-Control"), "no-store", "an answer that may carry a key's value is never cached")
		newValue := stored(sellersKey.Id).Key
		require.NotEqual(t, oldValue, newValue, "再次分配时自动换值")
		require.Equal(t, closer.userID, stored(sellersKey.Id).UserId)
		require.Contains(t, handover.Body.String(), `"holder":"closer"`)
		require.NotContains(t, handover.Body.String(), newValue)
		require.NotContains(t, handover.Body.String(), `"value"`)

		// The command the first seller copied was for a key that is no longer
		// theirs: it hands out nothing — not the old value, not the new one.
		stale := browser{engine: engine}.raw(t, http.MethodGet, pending.ScriptPath, nil)
		require.Equal(t, http.StatusOK, stale.Code)
		require.NotContains(t, stale.Body.String(), oldValue)
		require.NotContains(t, stale.Body.String(), newValue)
		require.Contains(t, stale.Body.String(), "no longer exists")
		// Nor does the key show on their own keys page anymore; it is on the new
		// holder's, who installs it with one-click setup.
		require.NotContains(t, seller.raw(t, http.MethodGet, "/api/token/?p=1&size=20", nil).Body.String(), "Sales tools")
		require.Contains(t, closer.raw(t, http.MethodGet, "/api/token/?p=1&size=20", nil).Body.String(), "Sales tools")
		var issued struct {
			ScriptPath string `json:"script_path"`
		}
		closer.ok(t, http.MethodPost, "/api/connect/token", map[string]any{"key_id": sellersKey.Id, "tools": []string{"codex"}}, &issued)
		installed := browser{engine: engine}.raw(t, http.MethodGet, issued.ScriptPath, nil)
		require.Contains(t, installed.Body.String(), newValue)
		require.NotContains(t, installed.Body.String(), oldValue)

		// --- manager 只能分配给本部门 staff，跨部门 403 ----------------------------
		manager.refused(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/assign", map[string]any{"holder_id": builder.userID})
		var buildersKey key
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Build tools", "holder_id": builder.userID, "unlimited_quota": true}, &buildersKey)
		manager.refused(t, http.MethodPost, "/api/org/keys/"+id(buildersKey.Id)+"/assign", map[string]any{"holder_id": seller.userID})
		manager.refused(t, http.MethodPost, "/api/org/keys/"+id(buildersKey.Id)+"/reclaim", nil)
		require.Equal(t, closer.userID, stored(sellersKey.Id).UserId)
		require.Equal(t, newValue, stored(sellersKey.Id).Key)
		require.Equal(t, builder.userID, stored(buildersKey.Id).UserId)
		var assignees []struct {
			Name string `json:"name"`
		}
		manager.ok(t, http.MethodGet, "/api/org/key-assignees", nil, &assignees)
		offered := []string{}
		for _, a := range assignees {
			offered = append(offered, a.Name)
		}
		require.Equal(t, []string{"manager", "seller", "closer"}, offered, "the people of Sales, and nobody else")
		// Whoever may not hand keys out is not told whom one could go to.
		for _, member := range []browser{hr, seller} {
			member.refused(t, http.MethodGet, "/api/org/key-assignees", nil)
			member.refused(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/assign", map[string]any{"holder_id": seller.userID})
			member.refused(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/reclaim", nil)
		}

		// --- 回收后 key 自动冻结并挂回 owner 名下 ----------------------------------
		var parked key
		manager.ok(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/reclaim", nil, &parked)
		require.Equal(t, []any{owner.userID, true, common.TokenStatusDisabled}, []any{parked.HolderId, parked.HolderIsOwner, parked.Status})
		reclaimed := stored(sellersKey.Id)
		require.Equal(t, []any{owner.userID, common.TokenStatusDisabled}, []any{reclaimed.UserId, reclaimed.Status})
		require.NotContains(t, []string{oldValue, newValue}, reclaimed.Key)
		// It is out of the manager's sight and reach now, and on nobody's own page.
		require.Equal(t, []string{}, keyNames(manager))
		manager.refused(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/assign", map[string]any{"holder_id": seller.userID})
		require.NotContains(t, closer.raw(t, http.MethodGet, "/api/token/?p=1&size=20", nil).Body.String(), "Sales tools")
		// IT hands it out again: it works, under a fourth value.
		var again key
		it.ok(t, http.MethodPost, "/api/org/keys/"+id(sellersKey.Id)+"/assign", map[string]any{"holder_id": builder.userID}, &again)
		require.Equal(t, []any{"builder", common.TokenStatusEnabled}, []any{again.Holder, again.Status})
		require.NotContains(t, []string{oldValue, newValue, reclaimed.Key}, stored(sellersKey.Id).Key)

		// --- no member deletes their own account ------------------------------------
		for name, member := range map[string]browser{"a seller": seller, "the manager": manager} {
			status, success, message, _ := member.call(t, http.MethodDelete, "/api/user/self", nil)
			require.Equal(t, http.StatusOK, status, name)
			require.False(t, success, name)
			require.Equal(t, "org.member_cannot_delete_account", message, name)
		}

		// --- HR 场景：清理离职成员名下的 key，操作有审计日志 ------------------------
		// HR is no key manager — and cleans up after a leaver all the same, by
		// removing them: both of the builder's keys in one go.
		var removal struct {
			ReclaimedKeys int `json:"reclaimed_keys"`
		}
		hr.ok(t, http.MethodDelete, "/api/org/members/"+id(builder.userID), nil, &removal)
		require.Equal(t, 2, removal.ReclaimedKeys)
		for _, keyID := range []int{sellersKey.Id, buildersKey.Id} {
			after := stored(keyID)
			require.Equal(t, []any{owner.userID, common.TokenStatusDisabled}, []any{after.UserId, after.Status})
		}
		var gone int64
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", builder.userID).Count(&gone).Error)
		require.Zero(t, gone, "the account is deleted")
		// What HR may not: the owner, an admin's dismissal, themselves, a stranger.
		for target, want := range map[int]string{owner.userID: "org.owner_not_removable", hr.userID: "org.member_remove_self", personal.userID: "org.member_not_found"} {
			status, success, message, _ := hr.call(t, http.MethodDelete, "/api/org/members/"+id(target), nil)
			require.Equal(t, http.StatusOK, status)
			require.False(t, success)
			require.Equal(t, want, message)
		}
		for _, member := range []browser{manager, seller} {
			member.refused(t, http.MethodDelete, "/api/org/members/"+id(closer.userID), nil)
		}

		// The session the builder still holds opens nothing of the organization.
		status, success, _, data := builder.call(t, http.MethodGet, "/api/org/self", nil)
		require.Equal(t, http.StatusOK, status)
		require.True(t, success)
		require.Equal(t, "null", string(data))
		for _, path := range []string{"/api/org/members", "/api/org/keys", "/api/org/departments", "/api/org/self/keys?purpose=chat"} {
			status, success, message, _ := builder.call(t, http.MethodGet, path, nil)
			require.Equal(t, http.StatusForbidden, status, path)
			require.False(t, success, path)
			require.Equal(t, "org.not_member", message, path)
		}
		require.Equal(t, http.StatusForbidden, builder.raw(t, http.MethodPost, "/api/token/", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}).Code)

		// The member list no longer has them, and says what each holds.
		var members []struct {
			Username string `json:"username"`
			KeyCount int    `json:"key_count"`
		}
		owner.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		held := map[string]int{}
		for _, m := range members {
			held[m.Username] = m.KeyCount
		}
		require.Equal(t, map[string]int{"founder": 2, "manager": 0, "seller": 0, "closer": 0, "it": 0, "hr": 0}, held)

		// A member's own keys for a purpose: a list for a member, nothing for an
		// account that is in no organization.
		status, success, _, data = closer.call(t, http.MethodGet, "/api/org/self/keys?purpose=chat", nil)
		require.Equal(t, http.StatusOK, status)
		require.True(t, success)
		require.Equal(t, "[]", string(data))
		status, _, message, _ := personal.call(t, http.MethodGet, "/api/org/self/keys?purpose=chat", nil)
		require.Equal(t, http.StatusForbidden, status)
		require.Equal(t, "org.not_member", message)

		// The audit log saw every hand-over, both ways of taking a key back and
		// the removal — and holds no key value.
		var page struct {
			Items []struct {
				Action string `json:"action"`
			} `json:"items"`
		}
		owner.ok(t, http.MethodGet, "/api/org/audit-logs?p=1&page_size=100", nil, &page)
		count := map[string]int{}
		for _, record := range page.Items {
			count[record.Action]++
		}
		require.Equal(t, 2, count[orgmodel.AuditKeyAssign], "to the closer, then to the builder")
		require.Equal(t, 3, count[orgmodel.AuditKeyReclaim], "by the manager, then twice with the removal")
		require.Equal(t, 1, count[orgmodel.AuditMemberRemove])
		log := owner.raw(t, http.MethodGet, "/api/org/audit-logs?p=1&page_size=100", nil).Body.String()
		for _, keyID := range []int{sellersKey.Id, buildersKey.Id} {
			require.NotContains(t, log, stored(keyID).Key)
		}
		for _, value := range []string{oldValue, newValue, reclaimed.Key} {
			require.NotContains(t, log, value)
		}
	})
}
