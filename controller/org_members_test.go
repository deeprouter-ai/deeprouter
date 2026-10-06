package controller

import (
	"net/http"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P3 (meta-repo docs/enterprise-org-prd.md): joining by invite
// link, service accounts that cannot sign in, the owner who cannot leave, and
// how the organization endpoints answer when they refuse.

// testPasswordHash is the stored form of "password123", computed once.
// Hashing a password is slow on purpose and many times slower under the race
// detector, so tests whose subject is not the sign-up seed their accounts with
// this instead of signing each one up.
var testPasswordHash = sync.OnceValue(func() string {
	hash, err := common.Password2Hash("password123")
	if err != nil {
		panic(err)
	}
	return hash
})

// seedUser inserts a personal account directly, without the sign-up handler.
func (env orgTestEnv) seedUser(t *testing.T, username string) model.User {
	t.Helper()
	user := model.User{
		Username:    username,
		Password:    testPasswordHash(),
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     username,
	}
	require.NoError(t, env.db.Create(&user).Error)
	return env.userByName(t, username)
}

// seedOwner inserts an account that owns a new organization, as a company
// sign-up leaves it.
func (env orgTestEnv) seedOwner(t *testing.T, username string, orgName string) model.User {
	t.Helper()
	user := env.seedUser(t, username)
	require.NoError(t, env.db.Transaction(func(tx *gorm.DB) error {
		_, err := orgservice.CreateForOwnerTx(tx, user.Id, orgName, "en")
		return err
	}))
	return env.userByName(t, username)
}

// seedMember inserts an account and puts it in the owner's organization with
// a preset role, as joining by invite leaves it.
func (env orgTestEnv) seedMember(t *testing.T, owner model.User, username string, role string) model.User {
	t.Helper()
	user := env.seedUser(t, username)
	code := env.invite(t, owner, role, 0).Code
	require.NoError(t, env.db.Transaction(func(tx *gorm.DB) error {
		return orgservice.JoinByInviteTx(tx, user.Id, code)
	}))
	return env.userByName(t, username)
}

// founder signs up with an organization and returns its owner.
func (env orgTestEnv) founder(t *testing.T, username string, orgName string) model.User {
	t.Helper()
	response, _ := env.register(t, map[string]any{"username": username, "password": "password123", "org_name": orgName})
	require.True(t, response.Success, response.Message)
	return env.userByName(t, username)
}

// actor loads a user as the member performing a management action.
func (env orgTestEnv) actor(t *testing.T, userID int) *orgservice.Actor {
	t.Helper()
	actor, err := orgservice.LoadActor(env.db, userID)
	require.NoError(t, err)
	return actor
}

// invite issues an invite link for the owner's organization with a preset role.
func (env orgTestEnv) invite(t *testing.T, owner model.User, role string, departmentID int) *orgservice.InviteView {
	t.Helper()
	roleID, err := orgmodel.PresetRoleID(env.db, role)
	require.NoError(t, err)
	invite, err := orgservice.CreateInvite(env.db, env.actor(t, owner.Id), roleID, departmentID)
	require.NoError(t, err)
	return invite
}

// join signs a new person up through an invite link and returns their account.
func (env orgTestEnv) join(t *testing.T, username string, code string) model.User {
	t.Helper()
	response, _ := env.register(t, map[string]any{"username": username, "password": "password123", "org_invite": code})
	require.True(t, response.Success, response.Message)
	return env.userByName(t, username)
}

// department loads a department of an organization by name.
func (env orgTestEnv) department(t *testing.T, orgID int, name string) orgmodel.Department {
	t.Helper()
	var department orgmodel.Department
	require.NoError(t, env.db.Where("org_id = ? AND name = ?", orgID, name).First(&department).Error)
	return department
}

// persona returns the persona stored in a user's setting.
func (env orgTestEnv) persona(t *testing.T, username string) string {
	t.Helper()
	user := env.userByName(t, username)
	return user.GetSetting().Persona
}

// callOrg runs an organization handler as userID and returns the decoded
// envelope and the HTTP status it answered with.
func callOrg(t *testing.T, handler gin.HandlerFunc, method string, body any, userID int, params ...gin.Param) (tokenAPIResponse, int) {
	t.Helper()
	ctx, recorder := newAuthenticatedContext(t, method, "/api/org/test", body, userID)
	ctx.Params = params
	handler(ctx)
	return decodeAPIResponse(t, recorder), recorder.Code
}

// idParam is the :id path parameter of an organization route.
func idParam(id int) gin.Param {
	return gin.Param{Key: "id", Value: strconv.Itoa(id)}
}

// Decision D24 and its acceptance item: 注册时创建了企业组织的用户，注册完成后直接
// 进入专业模式（Advanced）控制台，不落到 Simple 模式. The console a user gets is
// derived from the persona in their setting: "team" is Advanced, and "unset"
// is what sends everyone else to the welcome page and from there to Simple.
func TestOrgRegister_FounderIsATeamPersonaFromTheStart(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		founder := env.founder(t, "founder", "Acme")
		require.Equal(t, "team", env.persona(t, "founder"))

		// Whatever the client claims, founding a company decides it.
		response, _ := env.register(t, map[string]any{
			"username": "claims-casual", "password": "password123", "org_name": "Casual Co", "persona": "casual",
		})
		require.True(t, response.Success, response.Message)
		require.Equal(t, "team", env.persona(t, "claims-casual"))

		// Nobody else's persona is decided for them. Someone joining by
		// invite is still asked; so is a personal sign-up, which
		// TestOrgRegister_PersonalSignUpIsUnchanged pins.
		env.join(t, "invited", env.invite(t, founder, orgmodel.RoleStaff, 0).Code)
		require.Equal(t, "unset", env.persona(t, "invited"))
	})
}

// The founder's persona must not leak into the starter key: Register copies a
// client-sent persona into the key's purpose, and "team" is not a purpose.
func TestOrgRegister_FounderPersonaDoesNotBecomeAKeyPurpose(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &constant.GenerateDefaultToken, true)
		founder := env.founder(t, "founder", "Acme")

		var key model.Token
		require.NoError(t, env.db.Where("user_id = ?", founder.Id).First(&key).Error)
		require.Empty(t, key.SimplePurpose)
	})
}

func TestOrgRegister_NamesTheStarterDepartmentsInTheRequestLanguage(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		for username, tc := range map[string]struct {
			language string
			general  string
			support  string
		}{
			"zh":   {"zh-CN,zh;q=0.9,en;q=0.8", "综合", "客服"},
			"hant": {"zh-TW", "綜合", "客服"},
			"none": {"", "General", "Customer Support"},
		} {
			headers := map[string]string{}
			if tc.language != "" {
				headers["Accept-Language"] = tc.language
			}
			response, _ := env.post(t, "/api/user/register", map[string]any{
				"username": username, "password": "password123", "org_name": "Org " + username,
			}, headers)
			require.True(t, response.Success, response.Message)

			owner := env.userByName(t, username)
			var names []string
			require.NoError(t, env.db.Model(&orgmodel.Department{}).Where("org_id = ?", owner.OrgId).
				Order("id").Pluck("name", &names).Error)
			require.Len(t, names, 6, username)
			require.Equal(t, tc.general, names[0], username)
			require.Equal(t, tc.support, names[5], username)
			require.Equal(t, env.department(t, owner.OrgId, tc.general).Id, owner.DepartmentId,
				"%s: the owner sits in the default department", username)
		}
	})
}

// Acceptance: 被邀请人注册后自动加入组织并获得对应角色.
func TestOrgRegister_JoinsTheOrganizationOfAnInviteLink(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &common.QuotaForNewUser, 500000)
		owner := env.seedOwner(t, "founder", "Acme")
		sales := env.department(t, owner.OrgId, "Sales")
		invite := env.invite(t, owner, orgmodel.RoleManager, sales.Id)
		managerRole, err := orgmodel.PresetRoleID(env.db, orgmodel.RoleManager)
		require.NoError(t, err)

		response, recorder := env.register(t, map[string]any{
			"username": "newcomer", "password": "password123", "org_invite": invite.Code,
		})
		require.True(t, response.Success, response.Message)

		newcomer := env.userByName(t, "newcomer")
		require.Equal(t, owner.OrgId, newcomer.OrgId)
		require.Equal(t, managerRole, newcomer.OrgRoleId)
		require.Equal(t, sales.Id, newcomer.DepartmentId)
		require.False(t, newcomer.IsService)
		// Still an ordinary account on the platform, signed in like any other.
		require.Equal(t, common.RoleCommonUser, newcomer.Role)
		require.False(t, model.IsAdmin(newcomer.Id))
		require.Equal(t, 500000, newcomer.Quota, "the sign-up gift is the one every new account gets")
		require.Contains(t, recorder.Header().Get("Set-Cookie"), "session=")

		// The answer has exactly the fields a personal sign-up gets.
		var data map[string]any
		require.NoError(t, common.Unmarshal(response.Data, &data))
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		require.Equal(t, []string{"default_token", "display_name", "id", "next", "role", "trial_quota", "username"}, keys)
		require.EqualValues(t, common.RoleCommonUser, data["role"])

		membership, err := orgservice.GetMembership(env.db, newcomer.Id)
		require.NoError(t, err)
		require.Equal(t, "Acme", membership.OrgName)
		require.Equal(t, orgmodel.RoleManager, membership.Role)
		require.False(t, membership.IsOwner)

		// One link serves the whole team, and founds nothing.
		second := env.join(t, "second", invite.Code)
		require.Equal(t, owner.OrgId, second.OrgId)
		require.EqualValues(t, 1, env.count(t, &orgmodel.Organization{}))
		require.EqualValues(t, 3, env.count(t, &model.User{}, "org_id = ?", owner.OrgId))
	})
}

// PRD D16: a member holds only the keys an admin hands them, so a deployment
// that gives new accounts a starter key gives none to someone who joined by
// invite — while founders and personal accounts still get theirs.
func TestOrgRegister_AnInvitedMemberGetsNoStarterKey(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &constant.GenerateDefaultToken, true)
		owner := env.founder(t, "founder", "Acme")
		invite := env.invite(t, owner, orgmodel.RoleStaff, 0)

		response, _ := env.register(t, map[string]any{
			"username": "newcomer", "password": "password123", "org_invite": invite.Code,
		})
		require.True(t, response.Success, response.Message)
		var data struct {
			DefaultToken string `json:"default_token"`
		}
		require.NoError(t, common.Unmarshal(response.Data, &data))
		require.Empty(t, data.DefaultToken)

		newcomer := env.userByName(t, "newcomer")
		require.Zero(t, env.count(t, &model.Token{}, "user_id = ?", newcomer.Id))
		require.EqualValues(t, 1, env.count(t, &model.Token{}, "user_id = ?", owner.Id), "the founder's starter key is untouched")
	})
}

func TestOrgRegister_RejectsADeadInviteAndCreatesNothing(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		expired := env.invite(t, owner, orgmodel.RoleStaff, 0)
		require.NoError(t, env.db.Model(&orgmodel.OrgInvite{}).Where("id = ?", expired.Id).
			Update("expires_time", time.Now().Unix()-1).Error)
		revoked := env.invite(t, owner, orgmodel.RoleStaff, 0)
		require.NoError(t, orgservice.RevokeInvite(env.db, env.actor(t, owner.Id), revoked.Id))

		for name, code := range map[string]string{
			"unknown": "no-such-code",
			"expired": expired.Code,
			"revoked": revoked.Code,
		} {
			response, recorder := env.register(t, map[string]any{
				"username": name, "password": "password123", "org_invite": code,
			})
			require.False(t, response.Success, name)
			require.Equal(t, msgOrgInviteInvalid, response.Message, name)
			require.NotContains(t, recorder.Header().Get("Set-Cookie"), "session=", name)
		}
		// Not even a personal account is left behind: the username stays free.
		require.EqualValues(t, 1, env.count(t, &model.User{}), "only the founder exists")
	})
}

// PRD D20: one person, one organization — a sign-up cannot found one and join
// another in the same breath.
func TestOrgRegister_CannotFoundAndJoinAtOnce(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		invite := env.invite(t, owner, orgmodel.RoleStaff, 0)

		response, _ := env.register(t, map[string]any{
			"username": "greedy", "password": "password123", "org_name": "Mine", "org_invite": invite.Code,
		})
		require.False(t, response.Success)
		require.Equal(t, i18n.MsgInvalidParams, response.Message)
		require.EqualValues(t, 1, env.count(t, &model.User{}))
		require.EqualValues(t, 1, env.count(t, &orgmodel.Organization{}))
	})
}

// Acceptance: admin 能创建服务账号（不可登录的成员）. A service account has no
// password to try, and is turned away even if someone later gives it one —
// at the one place every way of signing in ends.
func TestOrgLogin_AServiceAccountCannotSignIn(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		created, _ := callOrg(t, CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "CI Pipeline"}, owner.Id)
		require.True(t, created.Success, created.Message)
		var bot orgservice.MemberView
		require.NoError(t, common.Unmarshal(created.Data, &bot))
		require.True(t, bot.IsService)

		login := func(username string, password string) (tokenAPIResponse, string) {
			response, recorder := env.post(t, "/api/user/login", map[string]any{"username": username, "password": password}, nil)
			return response, recorder.Header().Get("Set-Cookie")
		}

		// As created: nothing matches, because there is no password.
		for _, password := range []string{"password123", "svc", bot.Username} {
			response, cookie := login(bot.Username, password)
			require.False(t, response.Success, password)
			require.Equal(t, i18n.MsgUserUsernameOrPasswordError, response.Message)
			require.NotContains(t, cookie, "session=")
		}

		// Even with a password in the row (a platform admin could set one),
		// the right password does not open a session.
		require.NoError(t, env.db.Model(&model.User{}).Where("id = ?", bot.Id).Update("password", testPasswordHash()).Error)
		response, cookie := login(bot.Username, "password123")
		require.False(t, response.Success)
		require.Equal(t, msgOrgServiceAccountLogin, response.Message)
		require.NotContains(t, cookie, "session=")

		// Third-party sign-in, passkeys and 2FA all finish in setupLogin; it
		// refuses before it touches the session.
		var stored model.User
		require.NoError(t, env.db.First(&stored, bot.Id).Error)
		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/oauth/github", nil, 0)
		setupLogin(&stored, ctx)
		refused := decodeAPIResponse(t, recorder)
		require.False(t, refused.Success)
		require.Equal(t, msgOrgServiceAccountLogin, refused.Message)
		require.Empty(t, recorder.Header().Get("Set-Cookie"))
		require.Zero(t, stored.LastLoginAt)

		// Control: the same password signs a person in.
		response, cookie = login("founder", "password123")
		require.True(t, response.Success, response.Message)
		require.Contains(t, cookie, "session=")
	})
}

// Acceptance: owner 不可被移除 — their own hand included. The owner's account is
// the company wallet (PRD §7.3), so the self-service "delete my account" is
// closed to them; everyone else keeps it.
func TestOrgDeleteSelf_TheOwnerCannotDeleteTheirAccount(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		member := env.seedMember(t, owner, "member", orgmodel.RoleAdmin)
		solo := env.seedUser(t, "solo")

		ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/self", nil, owner.Id)
		DeleteSelf(ctx)
		refused := decodeAPIResponse(t, recorder)
		require.False(t, refused.Success)
		require.Equal(t, msgOrgOwnerCannotDeleteAccount, refused.Message)
		require.EqualValues(t, 1, env.count(t, &model.User{}, "id = ?", owner.Id), "the owner is still there")
		var org orgmodel.Organization
		require.NoError(t, env.db.First(&org, owner.OrgId).Error)
		require.Equal(t, owner.Id, org.OwnerUserId)

		for _, user := range []model.User{member, solo} {
			ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/self", nil, user.Id)
			DeleteSelf(ctx)
			deleted := decodeAPIResponse(t, recorder)
			require.True(t, deleted.Success, "%s: %s", user.Username, deleted.Message)
			require.Zero(t, env.count(t, &model.User{}, "id = ?", user.Id), user.Username)
		}
	})
}

// PRD §2: being refused for lack of permission is a 403. Everything else an
// organization endpoint refuses is the ordinary 200 + success=false.
func TestOrgEndpoints_AnswerRefusalsWithTheRightStatus(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		staff := env.seedMember(t, owner, "staff", orgmodel.RoleStaff)
		solo := env.seedUser(t, "solo")
		adminRole, err := orgmodel.PresetRoleID(env.db, orgmodel.RoleAdmin)
		require.NoError(t, err)
		ownerRole, err := orgmodel.PresetRoleID(env.db, orgmodel.RoleOwner)
		require.NoError(t, err)
		general := env.department(t, owner.OrgId, "General")
		sales := env.department(t, owner.OrgId, "Sales")

		for _, tc := range []struct {
			name    string
			handler gin.HandlerFunc
			method  string
			body    any
			userID  int
			params  []gin.Param
			status  int
			message string
		}{
			{"a personal account has no organization to manage", ListOrgMembers, http.MethodGet, nil, solo.Id, nil,
				http.StatusForbidden, msgOrgNotMember},
			{"staff cannot list members", ListOrgMembers, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list departments", ListOrgDepartments, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list roles", ListOrgRoles, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list invites", ListOrgInvites, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot create a department", CreateOrgDepartment, http.MethodPost, map[string]any{"name": "Mine"}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot promote themselves", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adminRole}, staff.Id, []gin.Param{idParam(staff.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot invite", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": adminRole}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot create a service account", CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "Bot"}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"an admin cannot appoint an admin", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adminRole}, admin.Id, []gin.Param{idParam(staff.Id)},
				http.StatusForbidden, msgOrgOwnerOnly},
			{"an admin cannot issue an admin invite", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": adminRole}, admin.Id, nil,
				http.StatusForbidden, msgOrgOwnerOnly},
			{"nobody demotes the owner", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adminRole}, admin.Id, []gin.Param{idParam(owner.Id)},
				http.StatusOK, msgOrgOwnerImmutable},
			{"nobody is made an owner", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": ownerRole}, owner.Id, []gin.Param{idParam(staff.Id)},
				http.StatusOK, msgOrgOwnerImmutable},
			{"nobody moves the owner out of the default department", UpdateOrgMember, http.MethodPut, map[string]any{"department_id": sales.Id}, admin.Id, []gin.Param{idParam(owner.Id)},
				http.StatusOK, msgOrgAdminDepartment},
			{"nobody moves an admin out of the default department", UpdateOrgMember, http.MethodPut, map[string]any{"department_id": sales.Id}, owner.Id, []gin.Param{idParam(admin.Id)},
				http.StatusOK, msgOrgAdminDepartment},
			{"an admin invite is for the default department", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": adminRole, "department_id": sales.Id}, owner.Id, nil,
				http.StatusOK, msgOrgAdminDepartment},
			{"a department name must be new", CreateOrgDepartment, http.MethodPost, map[string]any{"name": "Sales"}, owner.Id, nil,
				http.StatusOK, msgOrgDepartmentExists},
			{"a department name must not be blank", CreateOrgDepartment, http.MethodPost, map[string]any{"name": "  "}, owner.Id, nil,
				http.StatusOK, msgOrgDepartmentNameInvalid},
			{"the default department stays", DeleteOrgDepartment, http.MethodDelete, nil, owner.Id, []gin.Param{idParam(general.Id)},
				http.StatusOK, msgOrgDepartmentDefault},
			{"an unknown department", RenameOrgDepartment, http.MethodPut, map[string]any{"name": "X"}, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgDepartmentNotFound},
			{"an unknown member", UpdateOrgMember, http.MethodPut, map[string]any{"department_id": general.Id}, owner.Id, []gin.Param{idParam(solo.Id)},
				http.StatusOK, msgOrgMemberNotFound},
			{"an unknown role", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": 424242}, owner.Id, nil,
				http.StatusOK, msgOrgRoleNotFound},
			{"an unknown invite", RevokeOrgInvite, http.MethodDelete, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgInviteInvalid},
			{"a service account needs a name", CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": ""}, owner.Id, nil,
				http.StatusOK, msgOrgServiceAccountNameInvalid},
			{"a path id must be a number", DeleteOrgDepartment, http.MethodDelete, nil, owner.Id, []gin.Param{{Key: "id", Value: "abc"}},
				http.StatusOK, i18n.MsgInvalidParams},
			{"a body must be JSON", CreateOrgDepartment, http.MethodPost, nil, owner.Id, nil,
				http.StatusOK, i18n.MsgInvalidParams},
		} {
			response, status := callOrg(t, tc.handler, tc.method, tc.body, tc.userID, tc.params...)
			require.False(t, response.Success, tc.name)
			require.Equal(t, tc.status, status, tc.name)
			require.Equal(t, tc.message, response.Message, tc.name)
			require.Empty(t, string(response.Data), "%s: a refusal carries no data", tc.name)
		}

		// None of it left a mark.
		require.Equal(t, common.RoleCommonUser, env.userByName(t, "staff").Role)
		require.EqualValues(t, 1, env.count(t, &model.User{}, "org_id = ? AND role_id = ?", owner.OrgId, adminRole))
		require.EqualValues(t, 6, env.count(t, &orgmodel.Department{}, "org_id = ?", owner.OrgId))
		require.Zero(t, env.count(t, &model.User{}, "is_service = ?", true))
		for _, username := range []string{"founder", "admin"} {
			require.Equal(t, general.Id, env.userByName(t, username).DepartmentId, username)
		}
	})
}

// The public invite preview tells the holder of a code where it leads, and
// tells everyone else nothing.
func TestOrgInvitePreview(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		invite := env.invite(t, owner, orgmodel.RoleReadonly, env.department(t, owner.OrgId, "Product").Id)

		preview, status := callOrg(t, GetOrgInvitePreview, http.MethodGet, nil, 0, gin.Param{Key: "code", Value: invite.Code})
		require.True(t, preview.Success, preview.Message)
		require.Equal(t, http.StatusOK, status)
		require.JSONEq(t, `{"org_name":"Acme","role":"readonly","department":"Product"}`, string(preview.Data))

		missing, status := callOrg(t, GetOrgInvitePreview, http.MethodGet, nil, 0, gin.Param{Key: "code", Value: "no-such-code"})
		require.False(t, missing.Success)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, msgOrgInviteInvalid, missing.Message)
		require.Empty(t, string(missing.Data))
	})
}
