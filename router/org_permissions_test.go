package router

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	referralmodel "github.com/QuantumNous/new-api/internal/referral/model"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newOrgEngine points the platform globals at db and returns an engine serving
// the real API routes behind a session store, as main.go does.
func newOrgEngine(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.TwoFA{}, &referralmodel.ReferralRecord{}))
	require.NoError(t, orgmodel.Migrate(db))
	dialect := db.Dialector.Name()
	setGlobal(t, &model.DB, db)
	setGlobal(t, &model.LOG_DB, db)
	setGlobal(t, &common.UsingSQLite, dialect == "sqlite")
	setGlobal(t, &common.UsingPostgreSQL, dialect == "postgres")
	setGlobal(t, &common.UsingMySQL, dialect == "mysql")
	setGlobal(t, &common.RedisEnabled, false)
	setGlobal(t, &common.RegisterEnabled, true)
	setGlobal(t, &common.PasswordRegisterEnabled, true)
	setGlobal(t, &common.PasswordLoginEnabled, true)
	setGlobal(t, &common.EmailVerificationEnabled, false)
	setGlobal(t, &constant.GenerateDefaultToken, false)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("org-permissions-test-secret"))))
	SetApiRouter(engine)
	return engine
}

// refused sends a request that must be turned away for lack of permission: a
// real 403 with the organization's "forbidden" message and no data.
func (b browser) refused(t *testing.T, method string, path string, body any) {
	t.Helper()
	status, success, message, data := b.call(t, method, path, body)
	require.Equal(t, http.StatusForbidden, status, "%s %s", method, path)
	require.False(t, success, "%s %s", method, path)
	require.Equal(t, "org.forbidden", message, "%s %s", method, path)
	require.Equal(t, "null", string(data), "%s %s leaked data", method, path)
}

// TestOrgPermissions_ThroughTheRealRouter walks Enterprise Org P4 (meta-repo
// docs/enterprise-org-prd.md) end to end over the real routes, the real auth
// middleware and browser-style session cookies: a manager who reaches one
// department, a readonly member who reads everything and writes nothing, a
// custom role whose change takes effect on a session that never signed in
// again, and an audit log that saw all of it.
func TestOrgPermissions_ThroughTheRealRouter(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		engine := newOrgEngine(t, db)
		visitor := browser{engine: engine}
		id := strconv.Itoa

		owner := signUp(t, engine, map[string]any{"username": "founder", "password": "password123", "org_name": "Acme"}, "")
		type department struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		var departments []department
		owner.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		departmentID := map[string]int{}
		for _, d := range departments {
			departmentID[d.Name] = d.Id
		}
		type role struct {
			Id          int      `json:"id"`
			Name        string   `json:"name"`
			Scope       string   `json:"scope"`
			Permissions []string `json:"permissions"`
			IsPreset    bool     `json:"is_preset"`
		}
		var roles []role
		owner.ok(t, http.MethodGet, "/api/org/roles", nil, &roles)
		roleID := map[string]int{}
		for _, r := range roles {
			roleID[r.Name] = r.Id
		}

		// join signs somebody up through a fresh invite link for a role and department.
		join := func(username string, roleName string, departmentName string) browser {
			var invite struct {
				Code string `json:"code"`
			}
			owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[roleName], "department_id": departmentID[departmentName]}, &invite)
			return signUp(t, engine, map[string]any{"username": username, "password": "password123", "org_invite": invite.Code}, "")
		}
		manager := join("manager", orgmodel.RoleManager, "Sales")
		readonly := join("readonly", orgmodel.RoleReadonly, "General")
		seller := join("seller", orgmodel.RoleStaff, "Sales")
		builder := join("builder", orgmodel.RoleStaff, "Product")

		// --- A manager reaches the department they sit in, and nothing else ----
		var self struct {
			Role                 string   `json:"role"`
			RoleScope            string   `json:"role_scope"`
			Permissions          []string `json:"permissions"`
			ManagedDepartmentIds []int    `json:"managed_department_ids"`
		}
		manager.ok(t, http.MethodGet, "/api/org/self", nil, &self)
		require.Equal(t, orgmodel.RoleManager, self.Role)
		require.Equal(t, orgmodel.ScopeDept, self.RoleScope)
		require.Equal(t, []int{departmentID["Sales"]}, self.ManagedDepartmentIds)

		type member struct {
			Id                   int   `json:"id"`
			DepartmentId         int   `json:"department_id"`
			ManagedDepartmentIds []int `json:"managed_department_ids"`
		}
		var members []member
		manager.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		require.Equal(t, []member{
			{Id: manager.userID, DepartmentId: departmentID["Sales"], ManagedDepartmentIds: []int{departmentID["Sales"]}},
			{Id: seller.userID, DepartmentId: departmentID["Sales"], ManagedDepartmentIds: []int{}},
		}, members, "the manager sees Sales, and the builder in Product is not in the answer at all")
		manager.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		require.Equal(t, []department{{Id: departmentID["Sales"], Name: "Sales"}}, departments)

		// Acceptance: 只能邀请本部门的 staff；越权邀请被拒绝.
		var invite struct {
			Id   int    `json:"id"`
			Code string `json:"code"`
		}
		manager.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff], "department_id": departmentID["Sales"]}, &invite)
		recruit := signUp(t, engine, map[string]any{"username": "recruit", "password": "password123", "org_invite": invite.Code}, "")
		manager.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		require.Len(t, members, 3, "whoever they invited joined their department")
		require.Equal(t, recruit.userID, members[2].Id)

		manager.refused(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff], "department_id": departmentID["Product"]})
		manager.refused(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff]})
		manager.refused(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleManager], "department_id": departmentID["Sales"]})
		// Acceptance: 对非所管部门的成员 … 操作一律 403 — and managing the
		// organization itself is not a manager's at all.
		manager.refused(t, http.MethodPut, "/api/org/members/"+id(builder.userID), map[string]any{"department_id": departmentID["Sales"]})
		manager.refused(t, http.MethodPost, "/api/org/departments", map[string]any{"name": "Mine"})
		manager.refused(t, http.MethodPost, "/api/org/roles", map[string]any{"name": "Mine", "scope": "dept", "permissions": []string{"key.read"}})
		manager.refused(t, http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "Bot"})
		manager.refused(t, http.MethodGet, "/api/org/audit-logs", nil)

		// The owner gives them Product as well; their open session reaches it.
		owner.ok(t, http.MethodPut, "/api/org/members/"+id(manager.userID),
			map[string]any{"managed_department_ids": []int{departmentID["Sales"], departmentID["Product"]}}, nil)
		manager.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		require.Len(t, members, 4)
		manager.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff], "department_id": departmentID["Product"]}, nil)

		// --- Readonly reads the whole organization and writes nothing ----------
		for _, path := range []string{"/api/org/members", "/api/org/departments", "/api/org/roles", "/api/org/permissions", "/api/org/audit-logs"} {
			readonly.ok(t, http.MethodGet, path, nil, nil)
		}
		readonly.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		require.Len(t, members, 6)
		for _, write := range []struct {
			method string
			path   string
			body   any
		}{
			{http.MethodPost, "/api/org/departments", map[string]any{"name": "Mine"}},
			{http.MethodPut, "/api/org/departments/" + id(departmentID["Sales"]), map[string]any{"name": "Mine"}},
			{http.MethodDelete, "/api/org/departments/" + id(departmentID["Sales"]), nil},
			{http.MethodPost, "/api/org/roles", map[string]any{"name": "Mine", "scope": "org", "permissions": []string{"key.read"}}},
			{http.MethodPost, "/api/org/role-packs/finance/adopt", map[string]any{}},
			{http.MethodPut, "/api/org/members/" + id(seller.userID), map[string]any{"role_id": roleID[orgmodel.RoleManager]}},
			{http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "Bot"}},
			{http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff]}},
			{http.MethodDelete, "/api/org/invites/" + id(invite.Id), nil},
			{http.MethodGet, "/api/org/invites", nil}, // a link is a way in, so even seeing one takes member.invite
		} {
			readonly.refused(t, write.method, write.path, write.body)
		}

		// --- A custom role, and a change to it that needs no new sign-in --------
		builder.refused(t, http.MethodGet, "/api/org/members", nil)
		var helper role
		owner.ok(t, http.MethodPost, "/api/org/roles", map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read"}}, &helper)
		owner.ok(t, http.MethodPut, "/api/org/members/"+id(builder.userID), map[string]any{"role_id": helper.Id}, nil)
		builder.refused(t, http.MethodGet, "/api/org/members", nil)

		rolePath := "/api/org/roles/" + id(helper.Id)
		owner.ok(t, http.MethodPut, rolePath, map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read", "member.read"}}, nil)
		builder.ok(t, http.MethodGet, "/api/org/members", nil, &members) // same cookie, no new sign-in
		require.Len(t, members, 6)
		builder.refused(t, http.MethodPost, "/api/org/departments", map[string]any{"name": "Mine"})

		owner.ok(t, http.MethodPut, rolePath, map[string]any{"name": "Helper", "scope": "org", "permissions": []string{"usage.read"}}, nil)
		builder.refused(t, http.MethodGet, "/api/org/members", nil)

		// A role pack, adopted in one request, is a role like any other.
		var finance role
		owner.ok(t, http.MethodPost, "/api/org/role-packs/finance/adopt", map[string]any{"name": "财务对账"}, &finance)
		require.Equal(t, role{Id: finance.Id, Name: "财务对账", Scope: orgmodel.ScopeOrg, Permissions: []string{"usage.read"}}, finance)
		owner.ok(t, http.MethodPut, "/api/org/roles/"+id(finance.Id), map[string]any{"name": "财务对账", "scope": "org", "permissions": []string{"usage.read", "audit.read"}}, nil)
		owner.ok(t, http.MethodPut, "/api/org/members/"+id(builder.userID), map[string]any{"role_id": finance.Id}, nil)
		builder.ok(t, http.MethodGet, "/api/org/audit-logs", nil, nil)

		// Deleting it sends its holder back to staff, at once.
		owner.ok(t, http.MethodDelete, "/api/org/roles/"+id(finance.Id), nil, nil)
		builder.refused(t, http.MethodGet, "/api/org/audit-logs", nil)
		builder.ok(t, http.MethodGet, "/api/org/self", nil, &self)
		require.Equal(t, orgmodel.RoleStaff, self.Role)
		require.Empty(t, self.Permissions)

		// --- The audit log saw all of it ---------------------------------------
		var audit struct {
			Total int `json:"total"`
			Items []struct {
				ActorUserId int    `json:"actor_user_id"`
				Actor       string `json:"actor"`
				Action      string `json:"action"`
				Ip          string `json:"ip"`
				CreatedTime int64  `json:"created_time"`
			} `json:"items"`
		}
		readonly.ok(t, http.MethodGet, "/api/org/audit-logs?page_size=100", nil, &audit)
		require.Equal(t, audit.Total, len(audit.Items))
		counts := map[string]int{}
		for _, item := range audit.Items {
			counts[item.Action]++
			require.NotZero(t, item.ActorUserId, item.Action)
			require.NotEmpty(t, item.Actor, item.Action)
			require.Equal(t, "192.0.2.1", item.Ip, item.Action)
			require.NotZero(t, item.CreatedTime, item.Action)
		}
		require.Equal(t, map[string]int{
			orgmodel.AuditMemberInvite:  6, // four by the owner, two by the manager
			orgmodel.AuditMemberJoin:    5,
			orgmodel.AuditMemberManages: 1,
			orgmodel.AuditRoleCreate:    2, // one built, one adopted
			orgmodel.AuditRoleUpdate:    3,
			orgmodel.AuditRoleAssign:    2,
			orgmodel.AuditRoleDelete:    1,
		}, counts, "every write that succeeded, and none of the refused ones")
		require.Equal(t, orgmodel.AuditRoleDelete, audit.Items[0].Action, "newest first")
		require.Equal(t, "founder", audit.Items[0].Actor)

		// --- No sign-in, no organization ---------------------------------------
		for _, probe := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/org/permissions"},
			{http.MethodPost, "/api/org/roles"},
			{http.MethodPut, rolePath},
			{http.MethodDelete, rolePath},
			{http.MethodPost, "/api/org/role-packs/finance/adopt"},
			{http.MethodGet, "/api/org/audit-logs"},
		} {
			status, success, _, data := visitor.call(t, probe.method, probe.path, nil)
			require.Equal(t, http.StatusUnauthorized, status, "%s %s", probe.method, probe.path)
			require.False(t, success, "%s %s", probe.method, probe.path)
			require.Equal(t, "null", string(data), "%s %s", probe.method, probe.path)
		}

		// Through all of it, every account stayed a common user on the platform.
		var elevated int64
		require.NoError(t, db.Model(&model.User{}).Where("role <> ?", common.RoleCommonUser).Count(&elevated).Error)
		require.Zero(t, elevated)
	})
}
