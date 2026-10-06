package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
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

// browser is one signed-in visitor: the session cookie the server set and the
// user id the frontend echoes back in New-Api-User.
type browser struct {
	engine *gin.Engine
	cookie string
	userID int
}

// call sends a request the way the web console does and returns the HTTP
// status, whether the envelope reported success, its message and its data.
func (b browser) call(t *testing.T, method string, path string, body any) (int, bool, string, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if b.cookie != "" {
		request.Header.Set("Cookie", b.cookie)
	}
	if b.userID != 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(b.userID))
	}
	recorder := httptest.NewRecorder()
	b.engine.ServeHTTP(recorder, request)

	var envelope struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope),
		"%s %s answered %d %q", method, path, recorder.Code, recorder.Body.String())
	var data struct {
		Data any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &data))
	raw, err := common.Marshal(data.Data)
	require.NoError(t, err)
	return recorder.Code, envelope.Success, envelope.Message, raw
}

// ok sends a request that must succeed and decodes its data into into (which
// may be nil).
func (b browser) ok(t *testing.T, method string, path string, body any, into any) {
	t.Helper()
	status, success, message, data := b.call(t, method, path, body)
	require.Equal(t, http.StatusOK, status, "%s %s: %s", method, path, message)
	require.True(t, success, "%s %s: %s", method, path, message)
	if into != nil {
		require.NoError(t, common.Unmarshal(data, into), "%s %s", method, path)
	}
}

// signUp registers an account through the real sign-up route and returns the
// browser it leaves signed in.
func signUp(t *testing.T, engine *gin.Engine, body map[string]any, language string) browser {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if language != "" {
		request.Header.Set("Accept-Language", language)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	var envelope struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Id int `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope), recorder.Body.String())
	require.True(t, envelope.Success, envelope.Message)
	setCookie := recorder.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "session=")
	return browser{engine: engine, cookie: strings.SplitN(setCookie, ";", 2)[0], userID: envelope.Data.Id}
}

// setGlobal sets a package-level variable for the duration of one test.
func setGlobal[T any](t *testing.T, target *T, value T) {
	t.Helper()
	previous := *target
	*target = value
	t.Cleanup(func() { *target = previous })
}

// TestOrgManagement_ThroughTheRealRouter walks Enterprise Org P3 (meta-repo
// docs/enterprise-org-prd.md) end to end over the real routes, the real auth
// middleware and browser-style session cookies: a company signs up, shapes its
// departments, invites someone, makes them an admin, adds a service account —
// and is refused wherever the rules say so.
func TestOrgManagement_ThroughTheRealRouter(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
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
		engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("org-management-test-secret"))))
		SetApiRouter(engine)
		visitor := browser{engine: engine}

		// --- A company signs up, in Chinese ---------------------------------
		owner := signUp(t, engine, map[string]any{"username": "founder", "password": "password123", "org_name": "Acme"}, "zh-CN")
		var self struct {
			OrgName string `json:"org_name"`
			IsOwner bool   `json:"is_owner"`
			IsAdmin bool   `json:"is_admin"`
			Role    string `json:"role"`
		}
		owner.ok(t, http.MethodGet, "/api/org/self", nil, &self)
		require.Equal(t, "Acme", self.OrgName)
		require.True(t, self.IsOwner)
		require.Equal(t, orgmodel.RoleOwner, self.Role)

		type department struct {
			Id          int    `json:"id"`
			Name        string `json:"name"`
			IsDefault   bool   `json:"is_default"`
			MemberCount int    `json:"member_count"`
		}
		var departments []department
		owner.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		require.Len(t, departments, 6)
		require.Equal(t, department{Id: departments[0].Id, Name: "综合", IsDefault: true, MemberCount: 1}, departments[0])
		sales := departments[4]
		require.Equal(t, "销售", sales.Name)

		// --- Departments: create, rename, delete -----------------------------
		var created department
		owner.ok(t, http.MethodPost, "/api/org/departments", map[string]any{"name": "研发二部"}, &created)
		require.NotZero(t, created.Id)
		owner.ok(t, http.MethodPut, "/api/org/departments/"+strconv.Itoa(created.Id), map[string]any{"name": "平台研发"}, nil)
		owner.ok(t, http.MethodDelete, "/api/org/departments/"+strconv.Itoa(created.Id), nil, nil)
		status, success, message, _ := owner.call(t, http.MethodDelete, "/api/org/departments/"+strconv.Itoa(departments[0].Id), nil)
		require.Equal(t, http.StatusOK, status)
		require.False(t, success, "the default department cannot be deleted")
		require.Equal(t, "org.department_default", message)
		owner.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		require.Len(t, departments, 6)

		// --- An invite link, and what a stranger can do with it ---------------
		type role struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		var roles []role
		owner.ok(t, http.MethodGet, "/api/org/roles", nil, &roles)
		roleID := map[string]int{}
		for _, r := range roles {
			roleID[r.Name] = r.Id
		}
		require.Len(t, roleID, 5)

		var invite struct {
			Id   int    `json:"id"`
			Code string `json:"code"`
		}
		owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff], "department_id": sales.Id}, &invite)
		require.Len(t, invite.Code, 32)

		// Without signing in, the link's holder can read where it leads …
		var preview struct {
			OrgName    string `json:"org_name"`
			Role       string `json:"role"`
			Department string `json:"department"`
		}
		visitor.ok(t, http.MethodGet, "/api/org/invite/"+invite.Code, nil, &preview)
		require.Equal(t, "Acme", preview.OrgName)
		require.Equal(t, orgmodel.RoleStaff, preview.Role)
		require.Equal(t, "销售", preview.Department)
		// … and nothing else under /api/org.
		for _, path := range []string{"/api/org/self", "/api/org/members", "/api/org/departments", "/api/org/roles", "/api/org/invites"} {
			status, success, _, data := visitor.call(t, http.MethodGet, path, nil)
			require.Equal(t, http.StatusUnauthorized, status, path)
			require.False(t, success, path)
			require.Equal(t, "null", string(data), path)
		}

		// --- The invited person signs up and is in ----------------------------
		member := signUp(t, engine, map[string]any{"username": "newcomer", "password": "password123", "org_invite": invite.Code}, "")
		member.ok(t, http.MethodGet, "/api/org/self", nil, &self)
		require.Equal(t, "Acme", self.OrgName)
		require.False(t, self.IsOwner)
		require.False(t, self.IsAdmin)
		require.Equal(t, orgmodel.RoleStaff, self.Role)

		// Staff do not manage the organization: a real 403.
		for _, path := range []string{"/api/org/members", "/api/org/departments", "/api/org/roles", "/api/org/invites"} {
			status, success, message, _ := member.call(t, http.MethodGet, path, nil)
			require.Equal(t, http.StatusForbidden, status, path)
			require.False(t, success, path)
			require.Equal(t, "org.forbidden", message, path)
		}
		status, _, message, _ = member.call(t, http.MethodPut, "/api/org/members/"+strconv.Itoa(member.userID),
			map[string]any{"role_id": roleID[orgmodel.RoleAdmin]})
		require.Equal(t, http.StatusForbidden, status, "staff cannot promote themselves")
		require.Equal(t, "org.forbidden", message)

		// --- The owner appoints them admin; it works on their next request ----
		owner.ok(t, http.MethodPut, "/api/org/members/"+strconv.Itoa(member.userID),
			map[string]any{"role_id": roleID[orgmodel.RoleAdmin]}, nil)
		type memberRow struct {
			Id        int    `json:"id"`
			Role      string `json:"role"`
			IsOwner   bool   `json:"is_owner"`
			IsService bool   `json:"is_service"`
		}
		var members []memberRow
		member.ok(t, http.MethodGet, "/api/org/members", nil, &members) // same cookie, no new sign-in
		require.Equal(t, []memberRow{
			{Id: owner.userID, Role: orgmodel.RoleOwner, IsOwner: true},
			{Id: member.userID, Role: orgmodel.RoleAdmin},
		}, members)

		// --- The new admin adds a service account -----------------------------
		var bot struct {
			Id       int    `json:"id"`
			Username string `json:"username"`
		}
		member.ok(t, http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "CI 流水线"}, &bot)
		member.ok(t, http.MethodGet, "/api/org/members", nil, &members)
		require.Len(t, members, 3)
		require.Equal(t, memberRow{Id: bot.Id, Role: orgmodel.RoleStaff, IsService: true}, members[2])
		// It cannot sign in.
		status, success, _, _ = visitor.call(t, http.MethodPost, "/api/user/login", map[string]any{"username": bot.Username, "password": "password123"})
		require.Equal(t, http.StatusOK, status)
		require.False(t, success)

		// --- What an admin still cannot do -----------------------------------
		status, _, message, _ = member.call(t, http.MethodPut, "/api/org/members/"+strconv.Itoa(owner.userID),
			map[string]any{"role_id": roleID[orgmodel.RoleStaff]})
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, "org.owner_immutable", message, "an admin cannot dismiss the owner")
		status, _, message, _ = member.call(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleAdmin]})
		require.Equal(t, http.StatusForbidden, status)
		require.Equal(t, "org.owner_only", message, "an admin cannot mint more admins")
		// And an organization admin is nobody on the platform.
		for _, probe := range platformAdminProbes {
			status, success, message, data := member.call(t, probe.method, probe.path, nil)
			require.Equal(t, http.StatusOK, status, "%s %s", probe.method, probe.path)
			require.False(t, success, "%s %s must be rejected", probe.method, probe.path)
			require.Equal(t, i18n.MsgAuthInsufficientPrivilege, message, "%s %s", probe.method, probe.path)
			require.Equal(t, "null", string(data), "%s %s leaked data", probe.method, probe.path)
		}

		// --- The owner cannot be removed, by their own hand either ------------
		status, success, message, _ = owner.call(t, http.MethodDelete, "/api/user/self", nil)
		require.Equal(t, http.StatusOK, status)
		require.False(t, success)
		require.Equal(t, "org.owner_cannot_delete_account", message)
		owner.ok(t, http.MethodGet, "/api/org/self", nil, &self)
		require.True(t, self.IsOwner)

		// --- Revoking the link closes the door --------------------------------
		owner.ok(t, http.MethodDelete, "/api/org/invites/"+strconv.Itoa(invite.Id), nil, nil)
		status, success, message, _ = visitor.call(t, http.MethodGet, "/api/org/invite/"+invite.Code, nil)
		require.Equal(t, http.StatusOK, status)
		require.False(t, success)
		require.Equal(t, "org.invite_invalid", message)

		// Through all of it, every account stayed a common user on the platform.
		var elevated int64
		require.NoError(t, db.Model(&model.User{}).Where("role <> ?", common.RoleCommonUser).Count(&elevated).Error)
		require.Zero(t, elevated)
	})
}
