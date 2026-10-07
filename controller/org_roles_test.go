package controller

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md): the role, role
// pack, permission catalogue and audit log endpoints as the page calls them.
// Who is refused, and with which status, is in org_members_test.go.

// orgOK runs an organization handler that must succeed and decodes its data
// into into (which may be nil).
func orgOK(t *testing.T, handler gin.HandlerFunc, method string, body any, userID int, into any, params ...gin.Param) {
	t.Helper()
	response, status := callOrg(t, handler, method, body, userID, params...)
	require.Equal(t, http.StatusOK, status, response.Message)
	require.True(t, response.Success, response.Message)
	if into != nil {
		require.NoError(t, common.Unmarshal(response.Data, into))
	}
}

// Acceptance: admin 能从权限原语清单自建自定义角色并分配给成员 … 平台预置的岗位角色包
// 可一键采用为组织的自定义角色，采用后可自由修改.
func TestOrgRoles_AnAdminBuildsAdoptsChangesAndDeletesRoles(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		member := env.seedMember(t, owner, "member", orgmodel.RoleStaff)

		// What a role can be made of comes from the backend, not from the page.
		var catalog struct {
			Primitives []string `json:"primitives"`
			Powers     []struct {
				Name      string `json:"name"`
				OwnerOnly bool   `json:"owner_only"`
			} `json:"powers"`
			RolePacks []orgservice.RolePackView `json:"role_packs"`
		}
		orgOK(t, GetOrgPermissions, http.MethodGet, nil, admin.Id, &catalog)
		require.Equal(t, orgmodel.Primitives, catalog.Primitives)
		require.Len(t, catalog.Powers, len(orgmodel.InherentPowers))
		require.Equal(t, orgmodel.PowerAdmins, catalog.Powers[5].Name)
		require.True(t, catalog.Powers[5].OwnerOnly)
		require.Len(t, catalog.RolePacks, 3)

		// Built from the primitives …
		var built orgservice.RoleView
		orgOK(t, CreateOrgRole, http.MethodPost, map[string]any{
			"name": " Key Desk ", "scope": "org", "permissions": []string{"key.freeze", "key.read"},
		}, admin.Id, &built)
		require.Equal(t, orgservice.RoleView{Id: built.Id, Name: "Key Desk", Scope: "org", Permissions: []string{"key.read", "key.freeze"}, Powers: []string{}}, built)

		// … adopted from a pack under the name the page shows it by …
		var adopted orgservice.RoleView
		orgOK(t, AdoptOrgRolePack, http.MethodPost, map[string]any{"name": "财务对账"}, admin.Id, &adopted, gin.Param{Key: "key", Value: "finance"})
		require.Equal(t, orgservice.RoleView{Id: adopted.Id, Name: "财务对账", Scope: "org", Permissions: []string{"usage.read"}, Powers: []string{}}, adopted)
		// … or under the pack's own.
		var plain orgservice.RoleView
		orgOK(t, AdoptOrgRolePack, http.MethodPost, map[string]any{}, admin.Id, &plain, gin.Param{Key: "key", Value: "hr_ops"})
		require.Equal(t, "HR Ops", plain.Name)

		// … assigned, changed and deleted.
		orgOK(t, UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adopted.Id}, admin.Id, nil, idParam(member.Id))
		require.Equal(t, adopted.Id, env.userByName(t, "member").OrgRoleId)

		var changed orgservice.RoleView
		orgOK(t, UpdateOrgRole, http.MethodPut, map[string]any{
			"name": "Bookkeeping", "scope": "dept", "permissions": []string{"usage.read", "member.read"},
		}, admin.Id, &changed, idParam(adopted.Id))
		require.Equal(t, orgservice.RoleView{Id: adopted.Id, Name: "Bookkeeping", Scope: "dept", Permissions: []string{"member.read", "usage.read"}, Powers: []string{}}, changed)

		var roles []orgservice.RoleView
		orgOK(t, ListOrgRoles, http.MethodGet, nil, admin.Id, &roles)
		require.Len(t, roles, 8, "five presets and three of the organization's own")
		require.Equal(t, changed, roles[6])

		orgOK(t, DeleteOrgRole, http.MethodDelete, nil, admin.Id, nil, idParam(adopted.Id))
		staffRole, err := orgmodel.PresetRoleID(env.db, orgmodel.RoleStaff)
		require.NoError(t, err)
		require.Equal(t, staffRole, env.userByName(t, "member").OrgRoleId, "its holder is staff again")
		orgOK(t, ListOrgRoles, http.MethodGet, nil, admin.Id, &roles)
		require.Len(t, roles, 7)

		// 🔴 None of it touched a platform role.
		require.Zero(t, env.count(t, &model.User{}, "role <> ?", common.RoleCommonUser))
	})
}

// Acceptance: 修改角色权限即时生效，无需重新登录. The same account, asked twice:
// between the two requests only the role changed.
func TestOrgRoles_AChangedRoleTakesEffectOnTheNextRequest(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		member := env.seedMember(t, owner, "member", orgmodel.RoleStaff)
		var role orgservice.RoleView
		orgOK(t, CreateOrgRole, http.MethodPost, map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read"}}, owner.Id, &role)
		orgOK(t, UpdateOrgMember, http.MethodPut, map[string]any{"role_id": role.Id}, owner.Id, nil, idParam(member.Id))

		response, status := callOrg(t, ListOrgMembers, http.MethodGet, nil, member.Id)
		require.Equal(t, http.StatusForbidden, status)
		require.Equal(t, msgOrgForbidden, response.Message)

		orgOK(t, UpdateOrgRole, http.MethodPut, map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read", "member.read"}}, owner.Id, nil, idParam(role.Id))
		var members []orgservice.MemberView
		orgOK(t, ListOrgMembers, http.MethodGet, nil, member.Id, &members)
		require.Len(t, members, 2)

		orgOK(t, UpdateOrgRole, http.MethodPut, map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read"}}, owner.Id, nil, idParam(role.Id))
		_, status = callOrg(t, ListOrgMembers, http.MethodGet, nil, member.Id)
		require.Equal(t, http.StatusForbidden, status)
	})
}

// Acceptance: 组织内的管理操作全部写入审计日志 … 记录操作者、对象、时间与 IP；持有
// audit.read 的角色可以查看，readonly 默认拥有.
func TestOrgAuditLogs_AreReadByPageWithTheAddressOfEachRequest(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		readonly := env.seedMember(t, owner, "readonly", orgmodel.RoleReadonly)
		before := env.count(t, &orgmodel.OrgAuditLog{})

		orgOK(t, CreateOrgDepartment, http.MethodPost, map[string]any{"name": "Research"}, owner.Id, nil)
		orgOK(t, CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "CI"}, owner.Id, nil)
		require.Equal(t, before+2, env.count(t, &orgmodel.OrgAuditLog{}))

		var page struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
			Items    []struct {
				Id          int    `json:"id"`
				ActorUserId int    `json:"actor_user_id"`
				Actor       string `json:"actor"`
				Action      string `json:"action"`
				TargetType  string `json:"target_type"`
				TargetId    int    `json:"target_id"`
				Detail      struct {
					After map[string]any `json:"after"`
				} `json:"detail"`
				Ip          string `json:"ip"`
				CreatedTime int64  `json:"created_time"`
			} `json:"items"`
		}
		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/org/audit-logs?p=1&page_size=2", nil, readonly.Id)
		ListOrgAuditLogs(ctx)
		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		require.NoError(t, common.Unmarshal(response.Data, &page))

		require.Equal(t, 1, page.Page)
		require.Equal(t, 2, page.PageSize)
		require.EqualValues(t, before+2, page.Total)
		require.Len(t, page.Items, 2)
		newest, older := page.Items[0], page.Items[1]
		require.Equal(t, orgmodel.AuditServiceAccountCreate, newest.Action)
		require.Equal(t, orgmodel.AuditDepartmentCreate, older.Action)
		require.Equal(t, owner.Id, older.ActorUserId)
		require.Equal(t, "founder", older.Actor)
		require.Equal(t, orgmodel.AuditTargetDepartment, older.TargetType)
		require.Equal(t, env.department(t, owner.OrgId, "Research").Id, older.TargetId)
		require.Equal(t, "Research", older.Detail.After["name"], "the detail is JSON, not a string holding JSON")
		// httptest requests come from 192.0.2.1; the handler put that on the record.
		require.Equal(t, "192.0.2.1", older.Ip)
		require.NotZero(t, older.CreatedTime)

		// The second page holds what the first did not.
		ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/org/audit-logs?p=2&page_size=2", nil, readonly.Id)
		ListOrgAuditLogs(ctx)
		require.NoError(t, common.Unmarshal(decodeAPIResponse(t, recorder).Data, &page))
		require.Equal(t, 2, page.Page)
		for _, item := range page.Items {
			require.Less(t, item.Id, older.Id)
		}

		// A record is not something the API can change: there is no handler
		// that writes one on request, and the log only ever grew.
		require.Equal(t, before+2, env.count(t, &orgmodel.OrgAuditLog{}))
	})
}

// A refusal the page cannot put into words is a bug in waiting: every message
// an organization endpoint can answer with has to exist in every language the
// gateway speaks.
func TestOrgMessages_AreTranslatedInEveryLocale(t *testing.T) {
	keys := []string{msgOrgOwnerCannotDeleteAccount, msgOrgMemberCannotDeleteAccount, msgOrgServiceAccountLogin}
	for _, refusal := range orgRefusals {
		if strings.HasPrefix(refusal.key, "org.") {
			keys = append(keys, refusal.key)
		}
	}
	require.Greater(t, len(keys), 20)
	for _, locale := range []string{"en", "zh-CN", "zh-TW"} {
		raw, err := os.ReadFile("../i18n/locales/" + locale + ".yaml")
		require.NoError(t, err)
		lines := strings.Split(string(raw), "\n")
		for _, key := range keys {
			found := false
			for _, line := range lines {
				text, isKey := strings.CutPrefix(line, key+": ")
				if isKey {
					found = len(strings.Trim(strings.TrimSpace(text), `"`)) > 0
				}
			}
			require.True(t, found, "%s has no %s message", locale, key)
		}
	}
}
