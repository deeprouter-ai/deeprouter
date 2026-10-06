package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	referralmodel "github.com/QuantumNous/new-api/internal/referral/model"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P2 (meta-repo docs/enterprise-org-prd.md): signing up with an
// organization, and the two baselines that must hold around it — an org owner
// is still a common user, and a personal account behaves exactly as before.

// orgTestEnv is one database engine's worth of sign-up plumbing.
type orgTestEnv struct {
	db     *gorm.DB
	engine *gin.Engine
}

// setForTest sets a package-level variable for the duration of one test.
func setForTest[T any](t *testing.T, target *T, value T) {
	t.Helper()
	previous := *target
	*target = value
	t.Cleanup(func() { *target = previous })
}

// forEachOrgDialect runs fn on every available engine, with the platform
// globals pointed at a fresh migrated database and an engine that serves the
// real Register and Login handlers behind a session store, as main.go does.
func forEachOrgDialect(t *testing.T, fn func(t *testing.T, env orgTestEnv)) {
	t.Helper()
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gin.SetMode(gin.TestMode)
		require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.TwoFA{}, &referralmodel.ReferralRecord{}))
		require.NoError(t, orgmodel.Migrate(db))

		dialect := db.Dialector.Name()
		setForTest(t, &model.DB, db)
		setForTest(t, &model.LOG_DB, db)
		setForTest(t, &common.UsingSQLite, dialect == "sqlite")
		setForTest(t, &common.UsingPostgreSQL, dialect == "postgres")
		setForTest(t, &common.UsingMySQL, dialect == "mysql")
		setForTest(t, &common.RedisEnabled, false)
		setForTest(t, &common.RegisterEnabled, true)
		setForTest(t, &common.PasswordRegisterEnabled, true)
		setForTest(t, &common.PasswordLoginEnabled, true)
		setForTest(t, &common.EmailVerificationEnabled, false)
		setForTest(t, &constant.GenerateDefaultToken, false)

		engine := gin.New()
		engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("org-test-session-secret"))))
		engine.POST("/api/user/register", Register)
		engine.POST("/api/user/login", Login)
		fn(t, orgTestEnv{db: db, engine: engine})
	})
}

// register posts a sign-up and returns the decoded envelope and the raw response.
func (env orgTestEnv) register(t *testing.T, body map[string]any) (tokenAPIResponse, *httptest.ResponseRecorder) {
	t.Helper()
	return env.post(t, "/api/user/register", body, nil)
}

// post sends a JSON body to one of the engine's routes, with optional headers.
func (env orgTestEnv) post(t *testing.T, path string, body map[string]any, headers map[string]string) (tokenAPIResponse, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	env.engine.ServeHTTP(recorder, request)
	return decodeAPIResponse(t, recorder), recorder
}

// userByName loads a user, or fails the test if it does not exist.
func (env orgTestEnv) userByName(t *testing.T, username string) model.User {
	t.Helper()
	var user model.User
	require.NoError(t, env.db.Where("username = ?", username).First(&user).Error)
	return user
}

// count returns the number of rows of a table matching an optional condition.
func (env orgTestEnv) count(t *testing.T, table any, query ...any) int64 {
	t.Helper()
	tx := env.db.Model(table)
	if len(query) > 0 {
		tx = tx.Where(query[0], query[1:]...)
	}
	var n int64
	require.NoError(t, tx.Count(&n).Error)
	return n
}

// Acceptance: 新用户注册时可选择"创建企业组织"，注册人自动成为该组织唯一的 owner.
func TestOrgRegister_CreatesTheOrganizationWithItsSoleOwner(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		response, recorder := env.register(t, map[string]any{
			"username": "founder",
			"password": "password123",
			"org_name": "  Acme Pty Ltd  ",
		})
		require.True(t, response.Success, response.Message)

		founder := env.userByName(t, "founder")
		var org orgmodel.Organization
		require.NoError(t, env.db.First(&org).Error)
		require.Equal(t, "Acme Pty Ltd", org.Name)
		require.Equal(t, founder.Id, org.OwnerUserId)
		require.EqualValues(t, 1, env.count(t, &orgmodel.Organization{}))

		ownerRoleID, err := orgmodel.PresetRoleID(env.db, orgmodel.RoleOwner)
		require.NoError(t, err)
		require.Equal(t, org.Id, founder.OrgId)
		require.Equal(t, ownerRoleID, founder.OrgRoleId)
		require.EqualValues(t, 1, env.count(t, &model.User{}, "org_id = ?", org.Id), "the owner is the only member")
		require.EqualValues(t, 1, env.count(t, &model.User{}, "org_id = ? AND role_id = ?", org.Id, ownerRoleID), "and the only owner")

		// The sign-up still signs the user in, as a personal sign-up does.
		require.Contains(t, recorder.Header().Get("Set-Cookie"), "session=")

		membership, err := orgservice.GetMembership(env.db, founder.Id)
		require.NoError(t, err)
		require.True(t, membership.IsOwner)
		require.Equal(t, orgmodel.RoleOwner, membership.Role)
	})
}

// Acceptance: 客户组织的任何成员（含 owner）在全局层面仍是普通用户（role=1）.
// The session role is what every platform admin gate reads, so both the stored
// role and the one returned for the session must be the common-user role.
func TestOrgRegister_OwnerRemainsACommonUserOnThePlatform(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		response, _ := env.register(t, map[string]any{
			"username": "founder",
			"password": "password123",
			"org_name": "Acme",
		})
		require.True(t, response.Success, response.Message)

		require.Equal(t, common.RoleCommonUser, env.userByName(t, "founder").Role)
		var data struct {
			Role int `json:"role"`
		}
		require.NoError(t, common.Unmarshal(response.Data, &data))
		require.Equal(t, common.RoleCommonUser, data.Role)
		require.False(t, model.IsAdmin(env.userByName(t, "founder").Id))
	})
}

// Acceptance: 无组织（org_id=0）的现有个人用户，注册…的行为与改动前完全一致.
// A sign-up without org_name must store a plain personal account, create no
// organization, and answer with exactly the fields it answered with before.
func TestOrgRegister_PersonalSignUpIsUnchanged(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &common.QuotaForNewUser, 500000)
		response, recorder := env.register(t, map[string]any{
			"username": "solo",
			"password": "password123",
		})
		require.True(t, response.Success, response.Message)

		solo := env.userByName(t, "solo")
		require.Zero(t, solo.OrgId)
		require.Zero(t, solo.OrgRoleId)
		require.Zero(t, solo.DepartmentId)
		require.False(t, solo.IsService)
		require.Equal(t, common.RoleCommonUser, solo.Role)
		require.Equal(t, common.UserStatusEnabled, solo.Status)
		require.Equal(t, 500000, solo.Quota)
		require.Equal(t, "solo", solo.DisplayName)
		require.Equal(t, "default", solo.Group)
		require.Len(t, solo.AffCode, 4)
		require.Equal(t, "unset", solo.GetSetting().Persona)
		require.Zero(t, env.count(t, &orgmodel.Organization{}))
		require.EqualValues(t, 1, env.count(t, &model.Log{}, "user_id = ? AND type = ?", solo.Id, model.LogTypeSystem))
		require.Contains(t, recorder.Header().Get("Set-Cookie"), "session=")

		var data map[string]any
		require.NoError(t, common.Unmarshal(response.Data, &data))
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		require.Equal(t, []string{"default_token", "display_name", "id", "next", "role", "trial_quota", "username"}, keys)

		none, err := orgservice.GetMembership(env.db, solo.Id)
		require.NoError(t, err)
		require.Nil(t, none)
	})
}

// A request body is not a way into an organization or out of the common-user
// role: the sign-up reads org_name and nothing else about organizations.
func TestOrgRegister_BodyCannotSmuggleOrgOrRoleFields(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		response, _ := env.register(t, map[string]any{
			"username":      "sneaky",
			"password":      "password123",
			"role":          common.RoleRootUser,
			"org_id":        1,
			"org_role_id":   1,
			"role_id":       1,
			"department_id": 1,
			"is_service":    true,
		})
		require.True(t, response.Success, response.Message)

		sneaky := env.userByName(t, "sneaky")
		require.Equal(t, common.RoleCommonUser, sneaky.Role)
		require.Zero(t, sneaky.OrgId)
		require.Zero(t, sneaky.OrgRoleId)
		require.Zero(t, sneaky.DepartmentId)
		require.False(t, sneaky.IsService)
	})
}

func TestOrgRegister_RejectsABadOrganizationNameAndCreatesNothing(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		for name, orgName := range map[string]string{
			"blank":    "   ",
			"too-long": strings.Repeat("x", orgservice.NameMaxLength+1),
		} {
			response, _ := env.register(t, map[string]any{
				"username": name,
				"password": "password123",
				"org_name": orgName,
			})
			require.False(t, response.Success, name)
			require.Equal(t, msgOrgNameInvalid, response.Message, name)
		}
		// The account must not exist either: the username stays free to retry.
		require.Zero(t, env.count(t, &model.User{}))
		require.Zero(t, env.count(t, &orgmodel.Organization{}))
	})
}

// A sign-up that asked for a company must not leave a personal account behind
// when the company cannot be created.
func TestOrgRegister_RollsBackTheAccountWhenTheOrganizationFails(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		// Without the owner preset the organization step fails inside the
		// transaction, after the user row was inserted.
		require.NoError(t, env.db.Unscoped().
			Where("org_id = ? AND name = ?", 0, orgmodel.RoleOwner).
			Delete(&orgmodel.OrgRole{}).Error)

		response, recorder := env.register(t, map[string]any{
			"username": "founder",
			"password": "password123",
			"org_name": "Acme",
		})
		require.False(t, response.Success)
		require.Zero(t, env.count(t, &model.User{}))
		require.Zero(t, env.count(t, &orgmodel.Organization{}))
		require.Zero(t, env.count(t, &model.Log{}), "no sign-up bonus is logged for an account that was rolled back")
		require.NotContains(t, recorder.Header().Get("Set-Cookie"), "session=")
	})
}

// The inviter rewards run after the commit on the org path; they must land the
// same way they do for a personal sign-up.
func TestOrgRegister_InviterIsRewardedLikeForAPersonalSignUp(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &common.QuotaForNewUser, 0)
		setForTest(t, &common.QuotaForInviter, 300)
		setForTest(t, &common.QuotaForInvitee, 200)
		inviter := model.User{Username: "inviter", Password: "hash", Role: common.RoleCommonUser, AffCode: "INV1"}
		require.NoError(t, env.db.Create(&inviter).Error)

		for _, signUp := range []map[string]any{
			{"username": "personal", "password": "password123", "aff_code": "INV1"},
			{"username": "founder", "password": "password123", "aff_code": "INV1", "org_name": "Acme"},
		} {
			response, _ := env.register(t, signUp)
			require.True(t, response.Success, response.Message)
		}

		personal, founder := env.userByName(t, "personal"), env.userByName(t, "founder")
		require.Equal(t, 200, personal.Quota)
		require.Equal(t, personal.Quota, founder.Quota, "same invitee bonus on both paths")
		require.Equal(t, inviter.Id, founder.InviterId)
		reloaded := env.userByName(t, "inviter")
		require.Equal(t, 2, reloaded.AffCount)
		require.Equal(t, 600, reloaded.AffQuota)
		require.EqualValues(t, 2, env.count(t, &referralmodel.ReferralRecord{}, "inviter_id = ?", inviter.Id))
	})
}

// PRD D16: an organization account holds organization keys only, so when the
// deployment hands out a starter key it is an org key from the first second —
// while a personal starter key stays personal.
func TestOrgRegister_StarterKeyBelongsToTheOrganization(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &constant.GenerateDefaultToken, true)
		for _, signUp := range []map[string]any{
			{"username": "solo", "password": "password123"},
			{"username": "founder", "password": "password123", "org_name": "Acme"},
		} {
			response, _ := env.register(t, signUp)
			require.True(t, response.Success, response.Message)
		}
		solo, founder := env.userByName(t, "solo"), env.userByName(t, "founder")

		var personalKey, orgKey model.Token
		require.NoError(t, env.db.Where("user_id = ?", solo.Id).First(&personalKey).Error)
		require.Zero(t, personalKey.OrgId)
		require.Zero(t, personalKey.CreatedBy)
		require.NoError(t, env.db.Where("user_id = ?", founder.Id).First(&orgKey).Error)
		require.Equal(t, founder.OrgId, orgKey.OrgId)
		require.Equal(t, founder.Id, orgKey.CreatedBy)
	})
}

// The personal key endpoints must not be a way to mark a key as an org key, or
// to detach one: tokens.org_id decides whose wallet pays (P7) and which
// endpoints may change the key (P5).
func TestOrgTokenFields_AreNotWritableThroughThePersonalKeyEndpoints(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		smuggled := map[string]any{
			"name":            "smuggled",
			"expired_time":    -1,
			"unlimited_quota": true,
			"org_id":          7,
			"created_by":      7,
			"policy_template": "coding",
		}
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", smuggled, 1)
		AddToken(ctx)
		require.True(t, decodeAPIResponse(t, recorder).Success)
		var created model.Token
		require.NoError(t, env.db.Where("name = ?", "smuggled").First(&created).Error)
		require.Zero(t, created.OrgId)
		require.Zero(t, created.CreatedBy)
		require.Empty(t, created.PolicyTemplate)

		orgKey := model.Token{UserId: 1, Name: "org-key", Key: "org-key-value", Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UnlimitedQuota: true, OrgId: 3, CreatedBy: 9, PolicyTemplate: "creative"}
		require.NoError(t, env.db.Create(&orgKey).Error)
		detach := map[string]any{
			"id":              orgKey.Id,
			"name":            "org-key",
			"expired_time":    -1,
			"unlimited_quota": true,
			"org_id":          0,
			"created_by":      0,
			"policy_template": "",
		}
		ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/token/", detach, 1)
		UpdateToken(ctx)
		require.True(t, decodeAPIResponse(t, recorder).Success)
		var reloaded model.Token
		require.NoError(t, env.db.First(&reloaded, orgKey.Id).Error)
		require.Equal(t, 3, reloaded.OrgId)
		require.Equal(t, 9, reloaded.CreatedBy)
		require.Equal(t, "creative", reloaded.PolicyTemplate)
	})
}

// Nor may a member edit their own way into another organization or role.
func TestOrgUserFields_AreNotWritableThroughUpdateSelf(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		// UpdateSelf checks the current password before it changes anything.
		hashed, err := common.Password2Hash("password123")
		require.NoError(t, err)
		member := model.User{Username: "member", Password: hashed, DisplayName: "Member",
			Role: common.RoleCommonUser, AffCode: "MEM1", OrgId: 3, OrgRoleId: 4, DepartmentId: 5}
		require.NoError(t, env.db.Create(&member).Error)

		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", map[string]any{
			"username":          "member",
			"display_name":      "Renamed",
			"original_password": "password123",
			"role":              common.RoleRootUser,
			"org_id":            99,
			"org_role_id":       1,
			"role_id":           1,
			"department_id":     99,
			"is_service":        true,
		}, member.Id)
		UpdateSelf(ctx)
		require.True(t, decodeAPIResponse(t, recorder).Success)

		reloaded := env.userByName(t, "member")
		require.Equal(t, "Renamed", reloaded.DisplayName, "the legitimate part of the update went through")
		require.Equal(t, common.RoleCommonUser, reloaded.Role)
		require.Equal(t, 3, reloaded.OrgId)
		require.Equal(t, 4, reloaded.OrgRoleId)
		require.Equal(t, 5, reloaded.DepartmentId)
		require.False(t, reloaded.IsService)
	})
}

func TestOrgSelf_ReportsMembershipOrNullForAPersonalAccount(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		for _, signUp := range []map[string]any{
			{"username": "solo", "password": "password123"},
			{"username": "founder", "password": "password123", "org_name": "Acme"},
		} {
			response, _ := env.register(t, signUp)
			require.True(t, response.Success, response.Message)
		}

		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/org/self", nil, env.userByName(t, "solo").Id)
		GetOrgSelf(ctx)
		personal := decodeAPIResponse(t, recorder)
		require.True(t, personal.Success)
		require.Equal(t, "null", string(personal.Data))

		founder := env.userByName(t, "founder")
		ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/org/self", nil, founder.Id)
		GetOrgSelf(ctx)
		owner := decodeAPIResponse(t, recorder)
		require.True(t, owner.Success)
		var membership orgservice.Membership
		require.NoError(t, common.Unmarshal(owner.Data, &membership))
		require.NotZero(t, founder.DepartmentId, "the owner sits in the default department")
		require.Equal(t, orgservice.Membership{
			OrgId:                founder.OrgId,
			OrgName:              "Acme",
			IsOwner:              true,
			RoleId:               founder.OrgRoleId,
			Role:                 orgmodel.RoleOwner,
			RoleScope:            orgmodel.ScopeOrg,
			Permissions:          orgmodel.Primitives,
			DepartmentId:         founder.DepartmentId,
			ManagedDepartmentIds: []int{},
		}, membership)
		// A list that is empty travels as [], so the page never has to guard
		// against null.
		require.Contains(t, string(owner.Data), `"managed_department_ids":[]`)
	})
}
