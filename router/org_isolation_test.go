package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// platformAdminProbes is one route from every group that api-router.go guards
// with AdminAuth or RootAuth, plus each route guarded inline (as of
// 2026-10-06). It is a sample, not the guarantee: the guarantee is that org
// creation never writes users.role (internal/org/service), and every one of
// these gates reads nothing else. The probes prove the wiring end to end.
var platformAdminProbes = []struct {
	method string
	path   string
}{
	{http.MethodGet, "/api/status/test"},
	{http.MethodGet, "/api/user/"},
	{http.MethodGet, "/api/user/search"},
	{http.MethodPost, "/api/user/manage"},
	{http.MethodGet, "/api/subscription/admin/plans"},
	{http.MethodGet, "/api/option/"},
	{http.MethodPut, "/api/option/"},
	{http.MethodGet, "/api/custom-oauth-provider/"},
	{http.MethodGet, "/api/performance/stats"},
	{http.MethodGet, "/api/ratio_sync/channels"},
	{http.MethodGet, "/api/channel/"},
	{http.MethodPost, "/api/channel/"},
	{http.MethodPost, "/api/channel/1/key"},
	{http.MethodPost, "/api/channel/fetch_models"},
	{http.MethodGet, "/api/redemption/"},
	{http.MethodGet, "/api/log/"},
	{http.MethodDelete, "/api/log/"},
	{http.MethodGet, "/api/log/stat"},
	{http.MethodGet, "/api/log/search"},
	{http.MethodGet, "/api/log/channel_affinity_usage_cache"},
	{http.MethodGet, "/api/data/"},
	{http.MethodGet, "/api/data/users"},
	{http.MethodGet, "/api/group/"},
	{http.MethodGet, "/api/prefill_group/"},
	{http.MethodGet, "/api/mj/"},
	{http.MethodGet, "/api/task/"},
	{http.MethodGet, "/api/vendors/"},
	{http.MethodGet, "/api/models/"},
	{http.MethodGet, "/api/deployments/"},
	{http.MethodGet, "/api/admin/skills/"},
}

// apiEnvelope is the {success, message, data} wrapper every /api route answers with.
type apiEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// TestOrgOwner_IsRejectedByPlatformAdminRoutes covers the acceptance item
// "客户组织的任何成员（含 owner）在全局层面仍是普通用户（role=1），访问任何平台
// 管理端点被拒绝" (Enterprise Org P2) through the real router and the real
// auth middleware: the owner of a customer organization holds a valid login,
// reaches the organization's own endpoint, and is turned away by every
// platform admin gate for lack of privilege.
func TestOrgOwner_IsRejectedByPlatformAdminRoutes(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gin.SetMode(gin.TestMode)
		require.NoError(t, db.AutoMigrate(&model.User{}))
		require.NoError(t, orgmodel.Migrate(db))
		previousDB, previousRedis := model.DB, common.RedisEnabled
		model.DB, common.RedisEnabled = db, false
		t.Cleanup(func() { model.DB, common.RedisEnabled = previousDB, previousRedis })

		accessToken := "0123456789abcdef0123456789abcdef" // users.access_token is char(32)
		owner := model.User{
			Username:    "founder",
			Password:    "hashed-password",
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			AffCode:     "FND1",
			AccessToken: &accessToken,
		}
		require.NoError(t, db.Create(&owner).Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			_, err := orgservice.CreateForOwnerTx(tx, owner.Id, "Acme", "en")
			return err
		}))

		engine := gin.New()
		engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("org-isolation-test-secret"))))
		SetApiRouter(engine)

		call := func(method string, path string) apiEnvelope {
			request := httptest.NewRequest(method, path, nil)
			request.Header.Set("Authorization", accessToken)
			request.Header.Set("New-Api-User", strconv.Itoa(owner.Id))
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			var envelope apiEnvelope
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope),
				"%s %s answered %d %q", method, path, recorder.Code, recorder.Body.String())
			return envelope
		}

		// Control: the credentials are good and the org endpoint is open to
		// them, so the rejections below are about privilege and nothing else.
		self := call(http.MethodGet, "/api/org/self")
		require.True(t, self.Success, self.Message)
		require.NotNil(t, self.Data)

		for _, probe := range platformAdminProbes {
			got := call(probe.method, probe.path)
			require.False(t, got.Success, "%s %s must be rejected", probe.method, probe.path)
			require.Equal(t, i18n.MsgAuthInsufficientPrivilege, got.Message, "%s %s", probe.method, probe.path)
			require.Nil(t, got.Data, "%s %s leaked data", probe.method, probe.path)
		}
	})
}
