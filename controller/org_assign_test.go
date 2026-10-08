package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/alias_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md §3) over HTTP: handing
// a key to another member and taking it back, removing a member, and the two
// personal endpoints that are closed to members — creating a key and deleting
// one's own account.

// orgKeyAnswer is a key as the organization endpoints answer it.
type orgKeyAnswer struct {
	Id            int    `json:"id"`
	Key           string `json:"key"`
	Value         string `json:"value"`
	Status        int    `json:"status"`
	HolderId      int    `json:"holder_id"`
	Holder        string `json:"holder"`
	HolderIsOwner bool   `json:"holder_is_owner"`
}

// Acceptance: 分配/回收：admin 可分配给组织内任意成员；manager 只能分配给本部门 staff，
// 跨部门 403；回收后 key 自动冻结并挂回 owner 名下，再次分配时自动换值.
func TestOrgKeys_AreHandedOverAndTakenBackOverHTTP(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		sales := env.department(t, owner.OrgId, "Sales")
		product := env.department(t, owner.OrgId, "Product")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		manager := env.seedMemberIn(t, owner, "manager", orgmodel.RoleManager, sales.Id)
		alice := env.seedMemberIn(t, owner, "alice", orgmodel.RoleStaff, sales.Id)
		carol := env.seedMemberIn(t, owner, "carol", orgmodel.RoleStaff, sales.Id)
		bob := env.seedMemberIn(t, owner, "bob", orgmodel.RoleStaff, product.Id)
		created, _ := callOrg(t, CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "CI", "department_id": sales.Id}, owner.Id)
		require.True(t, created.Success, created.Message)
		var bot orgservice.MemberView
		require.NoError(t, common.Unmarshal(created.Data, &bot))
		key := env.seedKey(t, alice, owner.OrgId, "team key")

		// assign hands the key over as userID and returns the answer next to
		// what the database holds afterwards.
		assign := func(userID int, holderID int) (orgKeyAnswer, model.Token, string, int) {
			response, status := callOrg(t, AssignOrgKey, http.MethodPost, map[string]any{"holder_id": holderID}, userID, idParam(key.Id))
			var answer orgKeyAnswer
			if response.Success {
				require.NoError(t, common.Unmarshal(response.Data, &answer))
			}
			return answer, env.key(t, key.Id), response.Message + string(response.Data), status
		}
		values := []string{key.Key}
		// fresh checks that the key has a value it never had before.
		fresh := func(stored model.Token) {
			require.NotContains(t, values, stored.Key, "a value is never handed out twice")
			values = append(values, stored.Key)
		}

		// An admin hands a key to anyone in the organization.
		answer, stored, raw, status := assign(admin.Id, bob.Id)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, []any{bob.Id, "bob", false, common.TokenStatusEnabled}, []any{answer.HolderId, answer.Holder, answer.HolderIsOwner, answer.Status})
		require.Equal(t, bob.Id, stored.UserId)
		fresh(stored)
		// A person's key: masked, and no value anywhere in the answer.
		require.Equal(t, model.MaskTokenKey(stored.Key), answer.Key)
		require.Empty(t, answer.Value)
		require.NotContains(t, raw, `"value"`)
		require.NotContains(t, raw, stored.Key)

		// A manager moves what the people of their department hold, to people of
		// their department — and nothing across that line, in either direction.
		_, stored, raw, status = assign(manager.Id, alice.Id)
		require.Equal(t, http.StatusForbidden, status, "the key is held in Product now")
		require.Contains(t, raw, msgOrgForbidden)
		require.Equal(t, bob.Id, stored.UserId)
		_, stored, _, status = assign(admin.Id, alice.Id)
		require.Equal(t, http.StatusOK, status)
		fresh(stored)
		answer, stored, _, status = assign(manager.Id, carol.Id)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, "carol", answer.Holder)
		fresh(stored)
		_, stored, raw, status = assign(manager.Id, bob.Id)
		require.Equal(t, http.StatusForbidden, status, "跨部门 403")
		require.Contains(t, raw, msgOrgForbidden)
		require.Equal(t, carol.Id, stored.UserId)
		require.Equal(t, values[len(values)-1], stored.Key, "a refusal leaves the value alone")

		// To a service account: the new value, this once.
		answer, stored, _, status = assign(manager.Id, bot.Id)
		require.Equal(t, http.StatusOK, status)
		fresh(stored)
		require.Equal(t, stored.Key, answer.Value)
		require.Len(t, answer.Value, 48)
		require.Equal(t, model.MaskTokenKey(stored.Key), answer.Key)

		// Taking it back: frozen, under the owner, with a new value.
		response, status := callOrg(t, ReclaimOrgKey, http.MethodPost, nil, manager.Id, idParam(key.Id))
		require.True(t, response.Success, response.Message)
		require.Equal(t, http.StatusOK, status)
		var parked orgKeyAnswer
		require.NoError(t, common.Unmarshal(response.Data, &parked))
		stored = env.key(t, key.Id)
		fresh(stored)
		require.Equal(t, []any{owner.Id, true, common.TokenStatusDisabled}, []any{parked.HolderId, parked.HolderIsOwner, parked.Status})
		require.Equal(t, []any{owner.Id, common.TokenStatusDisabled}, []any{stored.UserId, stored.Status})
		require.NotContains(t, string(response.Data), stored.Key)
		require.NotContains(t, string(response.Data), `"value"`)
		// What the manager took back is out of their hands now.
		_, _, _, status = assign(manager.Id, alice.Id)
		require.Equal(t, http.StatusForbidden, status)

		// Handed out again by someone who may unfreeze: yet another value, and
		// it works.
		answer, stored, _, status = assign(owner.Id, alice.Id)
		require.Equal(t, http.StatusOK, status)
		fresh(stored)
		require.Equal(t, common.TokenStatusEnabled, answer.Status)
		require.Equal(t, 250, stored.UsedQuota, "历史用量保留")

		// Refusals that are not about permission travel as 200 + success=false.
		_, _, raw, status = assign(owner.Id, 424242)
		require.Equal(t, http.StatusOK, status)
		require.Contains(t, raw, msgOrgMemberNotFound)
		response, status = callOrg(t, AssignOrgKey, http.MethodPost, map[string]any{"holder_id": alice.Id}, owner.Id, idParam(424242))
		require.False(t, response.Success)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, msgOrgKeyNotFound, response.Message)
		response, _ = callOrg(t, ReclaimOrgKey, http.MethodPost, nil, owner.Id, idParam(424242))
		require.Equal(t, msgOrgKeyNotFound, response.Message)
		// Whoever holds no key.assign at all is told so, whatever they name.
		for _, userID := range []int{alice.Id, bot.Id} {
			_, _, _, status = assign(userID, userID)
			require.Equal(t, http.StatusForbidden, status)
			_, status = callOrg(t, ReclaimOrgKey, http.MethodPost, nil, userID, idParam(key.Id))
			require.Equal(t, http.StatusForbidden, status)
		}

		// The list a key is handed out from: not the owner, roles for whoever
		// may read the members.
		listed, status := callOrg(t, ListOrgKeyAssignees, http.MethodGet, nil, manager.Id)
		require.True(t, listed.Success, listed.Message)
		require.Equal(t, http.StatusOK, status)
		inSales := `"department_id":` + strconv.Itoa(sales.Id) + `,"department":"Sales"`
		staff := `"role_id":` + strconv.Itoa(alice.OrgRoleId) + `,"role":"staff"`
		require.JSONEq(t, `[
			{"id":`+strconv.Itoa(manager.Id)+`,"name":"manager",`+inSales+`,"role_id":`+strconv.Itoa(manager.OrgRoleId)+`,"role":"manager","is_service":false,"is_owner":false},
			{"id":`+strconv.Itoa(alice.Id)+`,"name":"alice",`+inSales+`,`+staff+`,"is_service":false,"is_owner":false},
			{"id":`+strconv.Itoa(carol.Id)+`,"name":"carol",`+inSales+`,`+staff+`,"is_service":false,"is_owner":false},
			{"id":`+strconv.Itoa(bot.Id)+`,"name":"CI",`+inSales+`,`+staff+`,"is_service":true,"is_owner":false}
		]`, string(listed.Data))
		_, status = callOrg(t, ListOrgKeyAssignees, http.MethodGet, nil, alice.Id)
		require.Equal(t, http.StatusForbidden, status)

		// Every hand-over and the taking back are in the log; no value is.
		var actions []string
		var records []orgmodel.OrgAuditLog
		require.NoError(t, env.db.Where("org_id = ? AND target_type = ? AND target_id = ?", owner.OrgId, orgmodel.AuditTargetKey, key.Id).Order("id").Find(&records).Error)
		for _, record := range records {
			actions = append(actions, record.Action)
			for _, value := range values {
				require.NotContains(t, record.Detail, value)
			}
		}
		require.Equal(t, []string{
			orgmodel.AuditKeyAssign, orgmodel.AuditKeyAssign, orgmodel.AuditKeyAssign, orgmodel.AuditKeyAssign,
			orgmodel.AuditKeyReclaim, orgmodel.AuditKeyAssign,
		}, actions)
	})
}

// Acceptance: 成员被移出组织时，其名下的组织 key 全部按回收处理（冻结并挂回 owner
// 名下），历史用量保留 — and the HR scenario: 持有含 key.delete/key.freeze/
// member.remove 的自定义角色，能清理任意离职成员名下的 key，操作有审计日志.
func TestOrgMembers_AreRemovedOverHTTP(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		product := env.department(t, owner.OrgId, "Product")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		colleague := env.seedMember(t, owner, "colleague", orgmodel.RoleAdmin)
		leaver := env.seedMemberIn(t, owner, "leaver", orgmodel.RoleStaff, product.Id)
		staff := env.seedMemberIn(t, owner, "staff", orgmodel.RoleStaff, product.Id)
		// HR: the role pack, adopted and given to a member.
		var hrRole orgservice.RoleView
		orgOK(t, AdoptOrgRolePack, http.MethodPost, map[string]any{"name": "HR Ops"}, owner.Id, &hrRole, gin.Param{Key: "key", Value: "hr_ops"})
		hr := env.seedMember(t, owner, "hr", orgmodel.RoleStaff)
		orgOK(t, UpdateOrgMember, http.MethodPut, map[string]any{"role_id": hrRole.Id}, owner.Id, nil, idParam(hr.Id))
		first := env.seedKey(t, leaver, owner.OrgId, "their first")
		second := env.seedKey(t, leaver, owner.OrgId, "their second")
		staffsKey := env.seedKey(t, staff, owner.OrgId, "a colleague's")

		remove := func(userID int, memberID int) (tokenAPIResponse, int) {
			return callOrg(t, RemoveOrgMember, http.MethodDelete, nil, userID, idParam(memberID))
		}

		// Who may not: no permission is a 403, a rule on top of it a refusal
		// with its own words.
		for _, userID := range []int{staff.Id, leaver.Id} {
			response, status := remove(userID, leaver.Id)
			require.Equal(t, http.StatusForbidden, status)
			require.Equal(t, msgOrgForbidden, response.Message)
		}
		for _, userID := range []int{owner.Id, admin.Id, hr.Id} {
			response, status := remove(userID, owner.Id)
			require.Equal(t, http.StatusOK, status)
			require.False(t, response.Success)
			require.Equal(t, msgOrgOwnerNotRemovable, response.Message)
		}
		for _, userID := range []int{admin.Id, hr.Id} {
			response, status := remove(userID, userID)
			require.Equal(t, http.StatusOK, status)
			require.Equal(t, msgOrgMemberRemoveSelf, response.Message)
			response, status = remove(userID, colleague.Id)
			require.Equal(t, http.StatusForbidden, status, "only the owner removes an admin")
			require.Equal(t, msgOrgOwnerOnly, response.Message)
		}
		response, status := remove(owner.Id, 424242)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, msgOrgMemberNotFound, response.Message)
		require.EqualValues(t, 6, env.count(t, &model.User{}, "org_id = ?", owner.OrgId), "nobody is gone yet")
		require.Equal(t, first, env.key(t, first.Id))

		// HR removes the leaver: every key of theirs is taken back in the same go.
		response, status = remove(hr.Id, leaver.Id)
		require.True(t, response.Success, response.Message)
		require.Equal(t, http.StatusOK, status)
		require.JSONEq(t, `{"reclaimed_keys":2}`, string(response.Data))
		for _, key := range []model.Token{first, second} {
			after := env.key(t, key.Id)
			require.Equal(t, []any{owner.Id, common.TokenStatusDisabled, 250}, []any{after.UserId, after.Status, after.UsedQuota}, key.Name)
			require.NotEqual(t, key.Key, after.Key, key.Name)
			require.Zero(t, env.count(t, &model.Token{}, "id = ?", 0), key.Name)
			var answering int64
			require.NoError(t, env.db.Unscoped().Model(&model.Token{}).Where(&model.Token{Key: key.Key}).Count(&answering).Error)
			require.Zero(t, answering, "%s: the value in their tools answers to nothing", key.Name)
		}
		require.Equal(t, staffsKey, env.key(t, staffsKey.Id), "nobody else's key is touched")
		require.Zero(t, env.count(t, &model.User{}, "id = ?", leaver.Id), "the account is deleted")

		// The log: after HR's own joining, two keys taken back and one member
		// removed — all in HR's name.
		var byHR []string
		var records []orgmodel.OrgAuditLog
		require.NoError(t, env.db.Where("org_id = ? AND actor_user_id = ?", owner.OrgId, hr.Id).Order("id").Find(&records).Error)
		for _, record := range records {
			byHR = append(byHR, record.Action)
			require.NotContains(t, record.Detail, first.Key)
			require.NotContains(t, record.Detail, env.key(t, first.Id).Key)
		}
		require.Equal(t, []string{orgmodel.AuditMemberJoin, orgmodel.AuditKeyReclaim, orgmodel.AuditKeyReclaim, orgmodel.AuditMemberRemove}, byHR)

		// The session the leaver may still hold opens nothing of the
		// organization: they are nobody's member, and say so to themselves.
		for name, handler := range map[string]gin.HandlerFunc{
			"the members": ListOrgMembers, "the keys": ListOrgKeys, "the roles": ListOrgRoles, "their own keys": ListOrgSelfKeys,
		} {
			response, status := callOrg(t, handler, http.MethodGet, nil, leaver.Id)
			require.Equal(t, http.StatusForbidden, status, name)
			require.Equal(t, msgOrgNotMember, response.Message, name)
			require.Empty(t, string(response.Data), name)
		}
		self, status := callOrg(t, GetOrgSelf, http.MethodGet, nil, leaver.Id)
		require.Equal(t, http.StatusOK, status)
		require.True(t, self.Success)
		require.Equal(t, "null", string(self.Data))

		// The member list counts what removing someone would take back.
		listed, _ := callOrg(t, ListOrgMembers, http.MethodGet, nil, owner.Id)
		var members []orgservice.MemberView
		require.NoError(t, common.Unmarshal(listed.Data, &members))
		counts := map[string]int{}
		for _, member := range members {
			counts[member.Username] = member.KeyCount
		}
		require.Equal(t, map[string]int{"founder": 2, "admin": 0, "colleague": 0, "staff": 1, "hr": 0}, counts)

		// The owner dismisses an admin by removing them.
		response, status = remove(owner.Id, colleague.Id)
		require.True(t, response.Success, response.Message)
		require.Equal(t, http.StatusOK, status)
		require.JSONEq(t, `{"reclaimed_keys":0}`, string(response.Data))
	})
}

// Acceptance: 组织成员不能通过个人 key 端点自建 key.
func TestOrgMembers_CannotCreateAPersonalKey(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		staff := env.seedMember(t, owner, "staff", orgmodel.RoleStaff)
		gone := env.seedMember(t, owner, "gone", orgmodel.RoleStaff)
		_, err := orgservice.RemoveMember(env.db, env.actor(t, owner.Id), gone.Id)
		require.NoError(t, err)
		solo := env.seedUser(t, "solo")

		create := func(userID int, body map[string]any) (tokenAPIResponse, *httptest.ResponseRecorder) {
			return callAt(t, AddToken, http.MethodPost, "/api/token/", body, userID)
		}
		plain := map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}
		// What the Simple console used to send on a member's behalf.
		purposed := map[string]any{"name": "my-chat-key", "expired_time": -1, "unlimited_quota": true, "simple_purpose": "chat"}

		// The owner and the admins as much as anyone: an organization account
		// holds organization keys only. So does a member who was removed and
		// still holds a session.
		for _, member := range []model.User{owner, admin, staff, gone} {
			for _, body := range []map[string]any{plain, purposed} {
				response, recorder := create(member.Id, body)
				require.Equal(t, http.StatusForbidden, recorder.Code, member.Username)
				require.False(t, response.Success, member.Username)
				require.Equal(t, msgOrgMemberPersonalKey, response.Message, member.Username)
				require.NotContains(t, recorder.Body.String(), `"key"`, member.Username)
			}
		}
		require.Zero(t, env.count(t, &model.Token{}), "nothing was created")

		// A personal account creates keys exactly as before.
		response, recorder := create(solo.Id, plain)
		require.True(t, response.Success, response.Message)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.EqualValues(t, 1, env.count(t, &model.Token{}, "user_id = ? AND org_id = ?", solo.Id, 0))
	})
}

// Enterprise Org P6: leaving an organization is done by whoever may remove
// members — which takes the member's keys back and leaves a record of who did
// it — so no member deletes their own account. The owner is told to turn to
// the platform (PRD D19), everyone else to an administrator.
func TestOrgDeleteSelf_NoMemberDeletesTheirOwnAccount(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		staff := env.seedMember(t, owner, "staff", orgmodel.RoleStaff)
		key := env.seedKey(t, staff, owner.OrgId, "the staff member's")
		solo := env.seedUser(t, "solo")

		deleteSelf := func(userID int) tokenAPIResponse {
			ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/self", nil, userID)
			DeleteSelf(ctx)
			return decodeAPIResponse(t, recorder)
		}

		refused := deleteSelf(owner.Id)
		require.False(t, refused.Success)
		require.Equal(t, msgOrgOwnerCannotDeleteAccount, refused.Message)
		var org orgmodel.Organization
		require.NoError(t, env.db.First(&org, owner.OrgId).Error)
		require.Equal(t, owner.Id, org.OwnerUserId)

		for _, member := range []model.User{admin, staff} {
			refused := deleteSelf(member.Id)
			require.False(t, refused.Success, member.Username)
			require.Equal(t, msgOrgMemberCannotDeleteAccount, refused.Message, member.Username)
		}
		for _, member := range []model.User{owner, admin, staff} {
			require.EqualValues(t, 1, env.count(t, &model.User{}, "id = ?", member.Id), "%s is still there", member.Username)
		}
		require.Equal(t, key, env.key(t, key.Id), "and their key is where it was")

		// A personal account deletes itself as before.
		deleted := deleteSelf(solo.Id)
		require.True(t, deleted.Success, deleted.Message)
		require.Zero(t, env.count(t, &model.User{}, "id = ?", solo.Id))
	})
}

// PRD D16: 简易控制台、视频页等自动建 key 的入口对组织成员改用已分配的组织 key；
// 没有可用 key 时提示联系管理员. The console asks here which of the member's keys
// can serve a purpose; the purposes resolve through the live catalogue, like a
// policy template's.
func TestOrgSelfKeys_AreTheMembersOwnKeysThatCanServeAPurpose(t *testing.T) {
	const ownerID = 7601
	db := orgWithCatalogue(t, ownerID)
	chatRules, limited := alias_setting.ModelWhitelistForToken("chat", "", "")
	require.True(t, limited)
	require.NotEmpty(t, chatRules)

	ask := func(purpose string) []orgservice.OwnKeyView {
		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/org/self/keys?purpose="+purpose, nil, ownerID)
		ListOrgSelfKeys(ctx)
		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var keys []orgservice.OwnKeyView
		require.NoError(t, common.Unmarshal(response.Data, &keys))
		// An empty answer is a list, so the page never has to guard against null.
		require.NotEqual(t, "null", string(response.Data))
		return keys
	}
	names := func(keys []orgservice.OwnKeyView) []string {
		out := []string{}
		for _, key := range keys {
			out = append(out, key.Name)
		}
		return out
	}
	require.Empty(t, ask("video"), "handed nothing, there is nothing to set a tool up with")

	create := func(body map[string]any) int {
		response, _ := callOrg(t, CreateOrgKey, http.MethodPost, body, ownerID)
		require.True(t, response.Success, response.Message)
		var view orgservice.KeyView
		require.NoError(t, common.Unmarshal(response.Data, &view))
		return view.Id
	}
	create(map[string]any{"name": "everything", "unlimited_quota": true})
	create(map[string]any{"name": "creative", "policy_template": "creative", "unlimited_quota": true})
	create(map[string]any{"name": "coding", "policy_template": "coding", "unlimited_quota": true})
	create(map[string]any{"name": "one video model", "model_limits": []string{"new-video-b"}, "unlimited_quota": true})
	frozen := create(map[string]any{"name": "frozen", "unlimited_quota": true})
	response, _ := callOrg(t, FreezeOrgKey, http.MethodPost, nil, ownerID, idParam(frozen))
	require.True(t, response.Success, response.Message)

	// Video: named models, each key told the ones it may call. Not the coding
	// key, not the frozen one; nothing unpriced or from another group.
	video := ask("video")
	require.Equal(t, []string{"one video model", "creative", "everything"}, names(video))
	require.Equal(t, []string{"new-video-b"}, video[0].Models)
	require.Equal(t, []string{"new-video-a", "new-video-b"}, video[1].Models)
	require.Equal(t, []string{"new-video-a", "new-video-b"}, video[2].Models)
	require.Equal(t, []string{"creative", "everything"}, names(ask("image")))
	// Chat and coding are rules, and the two rule tables share models
	// (claude-sonnet-* is covered by claude-*): a key serves either by what its
	// own rules cover. A key picked down to one video model serves neither.
	require.Equal(t, []string{"coding", "creative", "everything"}, names(ask("chat")))
	require.Equal(t, []string{"coding", "creative", "everything"}, names(ask("coding")))
	// The creative pack has no voice in it.
	require.Equal(t, []string{"everything"}, names(ask("voice")))
	require.Empty(t, ask("no-such-purpose"))

	// A personal account has no organization to ask.
	require.NoError(t, db.Create(&model.User{
		Id: 7602, Username: "solo-7602", Password: "password", Group: "default", Status: common.UserStatusEnabled, AffCode: "solo-7602",
	}).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/org/self/keys?purpose=video", nil, 7602)
	ListOrgSelfKeys(ctx)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, msgOrgNotMember, decodeAPIResponse(t, recorder).Message)
}
