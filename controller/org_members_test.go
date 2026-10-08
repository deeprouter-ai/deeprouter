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
// a preset role, in the default department, as joining by invite leaves it.
func (env orgTestEnv) seedMember(t *testing.T, owner model.User, username string, role string) model.User {
	t.Helper()
	return env.seedMemberIn(t, owner, username, role, 0)
}

// seedMemberIn is seedMember for a department of choice; zero is the default one.
func (env orgTestEnv) seedMemberIn(t *testing.T, owner model.User, username string, role string, departmentID int) model.User {
	t.Helper()
	user := env.seedUser(t, username)
	code := env.invite(t, owner, role, departmentID).Code
	require.NoError(t, env.db.Transaction(func(tx *gorm.DB) error {
		return orgservice.JoinByInviteTx(tx, user.Id, code, "")
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

// The founder's persona must not leak into the starter key: "team" is not a
// purpose. No persona is, and Register copies none into a key anymore
// (TestKeyPurpose_StarterKeyIsBoundToNoPersona).
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

// Acceptance: owner 不可被移除 — their own hand included. Who may and may not
// delete their own account is pinned in org_assign_test.go
// (TestOrgDeleteSelf_NoMemberDeletesTheirOwnAccount): since P6 no member does.

// PRD §2: being refused for lack of permission is a 403. Everything else an
// organization endpoint refuses is the ordinary 200 + success=false.
func TestOrgEndpoints_AnswerRefusalsWithTheRightStatus(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		general := env.department(t, owner.OrgId, "General")
		sales := env.department(t, owner.OrgId, "Sales")
		product := env.department(t, owner.OrgId, "Product")
		admin := env.seedMember(t, owner, "admin", orgmodel.RoleAdmin)
		staff := env.seedMember(t, owner, "staff", orgmodel.RoleStaff)
		readonly := env.seedMember(t, owner, "readonly", orgmodel.RoleReadonly)
		manager := env.seedMemberIn(t, owner, "manager", orgmodel.RoleManager, sales.Id)
		solo := env.seedUser(t, "solo")
		roleID := map[string]int{}
		for _, name := range []string{orgmodel.RoleOwner, orgmodel.RoleAdmin, orgmodel.RoleManager, orgmodel.RoleStaff} {
			id, err := orgmodel.PresetRoleID(env.db, name)
			require.NoError(t, err)
			roleID[name] = id
		}
		adminRole, ownerRole, staffRole := roleID[orgmodel.RoleAdmin], roleID[orgmodel.RoleOwner], roleID[orgmodel.RoleStaff]
		custom, err := orgservice.CreateRole(env.db, env.actor(t, owner.Id), orgservice.RoleInput{
			Name: "Key Desk", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"},
		})
		require.NoError(t, err)
		intoProduct := env.invite(t, owner, orgmodel.RoleStaff, product.Id)
		adminLink := env.invite(t, owner, orgmodel.RoleAdmin, 0)
		// Three organization keys in a staff member's hands: a working one, and
		// two frozen ones that cannot simply be unfrozen.
		staffKey := env.seedKey(t, staff, owner.OrgId, "staff's key")
		spentKey := env.seedKey(t, staff, owner.OrgId, "spent")
		expiredKey := env.seedKey(t, staff, owner.OrgId, "expired")
		require.NoError(t, env.db.Model(&model.Token{}).Where("id = ?", spentKey.Id).
			Updates(map[string]any{"status": common.TokenStatusDisabled, "remain_quota": 0}).Error)
		require.NoError(t, env.db.Model(&model.Token{}).Where("id = ?", expiredKey.Id).
			Updates(map[string]any{"status": common.TokenStatusDisabled, "expired_time": time.Now().Unix() - 60}).Error)
		role := func(name string, scope string, permissions ...string) map[string]any {
			return map[string]any{"name": name, "scope": scope, "permissions": permissions}
		}
		auditBefore := env.count(t, &orgmodel.OrgAuditLog{})

		type refusal struct {
			name    string
			handler gin.HandlerFunc
			method  string
			body    any
			userID  int
			params  []gin.Param
			status  int
			message string
		}
		cases := []refusal{
			{"a personal account has no organization to manage", ListOrgMembers, http.MethodGet, nil, solo.Id, nil,
				http.StatusForbidden, msgOrgNotMember},
			{"staff cannot list members", ListOrgMembers, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list departments", ListOrgDepartments, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list roles", ListOrgRoles, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot read the permission catalogue", GetOrgPermissions, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list invites", ListOrgInvites, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot read the audit log", ListOrgAuditLogs, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot create a department", CreateOrgDepartment, http.MethodPost, map[string]any{"name": "Mine"}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot promote themselves", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adminRole}, staff.Id, []gin.Param{idParam(staff.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot invite", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": adminRole}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot create a service account", CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "Bot"}, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},

			// Acceptance: 部门作用域硬检查 — a manager outside their department.
			{"a manager cannot invite into another department", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": staffRole, "department_id": product.Id}, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot invite into the default department by naming none", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": staffRole}, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot invite anything but staff", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": roleID[orgmodel.RoleManager], "department_id": sales.Id}, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot invite into a custom role", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": custom.Id, "department_id": sales.Id}, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot revoke a link into another department", RevokeOrgInvite, http.MethodDelete, nil, manager.Id, []gin.Param{idParam(intoProduct.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot move a member", UpdateOrgMember, http.MethodPut, map[string]any{"department_id": sales.Id}, manager.Id, []gin.Param{idParam(staff.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot create a role", CreateOrgRole, http.MethodPost, role("Mine", "dept", "key.read"), manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot read the audit log", ListOrgAuditLogs, http.MethodGet, nil, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},

			// Keys: holding one gives no say over it, and assigning is not creating.
			{"staff cannot list the organization's keys", ListOrgKeys, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot read the policy templates", ListOrgKeyTemplates, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list who a key can be for", ListOrgKeyHolders, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"staff cannot list the models a key can be limited to", ListOrgKeyModels, http.MethodGet, nil, staff.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager fills in no key form, and is offered no models for one", ListOrgKeyModels, http.MethodGet, nil, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"nor is a read-only member", ListOrgKeyModels, http.MethodGet, nil, readonly.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a holder cannot change their own key", UpdateOrgKey, http.MethodPut, map[string]any{"unlimited_quota": true}, staff.Id, []gin.Param{idParam(staffKey.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a holder cannot rotate their own key", RotateOrgKey, http.MethodPost, nil, staff.Id, []gin.Param{idParam(staffKey.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a holder cannot unfreeze their own key", UnfreezeOrgKey, http.MethodPost, nil, staff.Id, []gin.Param{idParam(spentKey.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a holder cannot delete their own key", DeleteOrgKey, http.MethodDelete, nil, staff.Id, []gin.Param{idParam(staffKey.Id)},
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot create a key", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "holder_id": manager.Id}, manager.Id, nil,
				http.StatusForbidden, msgOrgForbidden},
			{"a manager cannot rotate a key", RotateOrgKey, http.MethodPost, nil, manager.Id, []gin.Param{idParam(staffKey.Id)},
				http.StatusForbidden, msgOrgForbidden},
		}
		// Acceptance: readonly … 发起的任何写操作均被拒绝. Every writing endpoint
		// there is, tried by a member who may read all of it.
		for _, write := range []refusal{
			{name: "create a department", handler: CreateOrgDepartment, method: http.MethodPost, body: map[string]any{"name": "Mine"}},
			{name: "rename a department", handler: RenameOrgDepartment, method: http.MethodPut, body: map[string]any{"name": "Mine"}, params: []gin.Param{idParam(sales.Id)}},
			{name: "delete a department", handler: DeleteOrgDepartment, method: http.MethodDelete, params: []gin.Param{idParam(sales.Id)}},
			{name: "create a role", handler: CreateOrgRole, method: http.MethodPost, body: role("Mine", "org", "key.read")},
			{name: "change a role", handler: UpdateOrgRole, method: http.MethodPut, body: role("Mine", "org", "key.read"), params: []gin.Param{idParam(custom.Id)}},
			{name: "delete a role", handler: DeleteOrgRole, method: http.MethodDelete, params: []gin.Param{idParam(custom.Id)}},
			{name: "adopt a role pack", handler: AdoptOrgRolePack, method: http.MethodPost, body: map[string]any{}, params: []gin.Param{{Key: "key", Value: "finance"}}},
			{name: "give a member a role", handler: UpdateOrgMember, method: http.MethodPut, body: map[string]any{"role_id": custom.Id}, params: []gin.Param{idParam(staff.Id)}},
			{name: "move a member", handler: UpdateOrgMember, method: http.MethodPut, body: map[string]any{"department_id": sales.Id}, params: []gin.Param{idParam(staff.Id)}},
			{name: "set what a member manages", handler: UpdateOrgMember, method: http.MethodPut, body: map[string]any{"managed_department_ids": []int{product.Id}}, params: []gin.Param{idParam(manager.Id)}},
			{name: "create a service account", handler: CreateOrgServiceAccount, method: http.MethodPost, body: map[string]any{"name": "Bot"}},
			{name: "issue an invite link", handler: CreateOrgInvite, method: http.MethodPost, body: map[string]any{"role_id": staffRole}},
			{name: "revoke an invite link", handler: RevokeOrgInvite, method: http.MethodDelete, params: []gin.Param{idParam(intoProduct.Id)}},
			{name: "create a key", handler: CreateOrgKey, method: http.MethodPost, body: map[string]any{"name": "K"}},
			{name: "change a key", handler: UpdateOrgKey, method: http.MethodPut, body: map[string]any{"name": "K"}, params: []gin.Param{idParam(staffKey.Id)}},
			{name: "rotate a key", handler: RotateOrgKey, method: http.MethodPost, params: []gin.Param{idParam(staffKey.Id)}},
			{name: "freeze a key", handler: FreezeOrgKey, method: http.MethodPost, params: []gin.Param{idParam(staffKey.Id)}},
			{name: "unfreeze a key", handler: UnfreezeOrgKey, method: http.MethodPost, params: []gin.Param{idParam(spentKey.Id)}},
			{name: "delete a key", handler: DeleteOrgKey, method: http.MethodDelete, params: []gin.Param{idParam(staffKey.Id)}},
		} {
			write.name = "readonly cannot " + write.name
			write.userID, write.status, write.message = readonly.Id, http.StatusForbidden, msgOrgForbidden
			cases = append(cases, write)
		}
		cases = append(cases, []refusal{
			{"an admin cannot appoint an admin", UpdateOrgMember, http.MethodPut, map[string]any{"role_id": adminRole}, admin.Id, []gin.Param{idParam(staff.Id)},
				http.StatusForbidden, msgOrgOwnerOnly},
			{"an admin cannot issue an admin invite", CreateOrgInvite, http.MethodPost, map[string]any{"role_id": adminRole}, admin.Id, nil,
				http.StatusForbidden, msgOrgOwnerOnly},
			{"an admin cannot revoke the owner's admin invite", RevokeOrgInvite, http.MethodDelete, nil, admin.Id, []gin.Param{idParam(adminLink.Id)},
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
			{"only a department-scoped member manages departments", UpdateOrgMember, http.MethodPut, map[string]any{"managed_department_ids": []int{sales.Id}}, owner.Id, []gin.Param{idParam(staff.Id)},
				http.StatusOK, msgOrgMemberNotDepartmentScoped},
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
			{"a role needs a name", CreateOrgRole, http.MethodPost, role("  ", "org", "key.read"), owner.Id, nil,
				http.StatusOK, msgOrgRoleNameInvalid},
			{"a role name must be new", CreateOrgRole, http.MethodPost, role("Key Desk", "org", "key.read"), owner.Id, nil,
				http.StatusOK, msgOrgRoleExists},
			{"a role cannot take a preset's name", CreateOrgRole, http.MethodPost, role("Admin", "org", "key.read"), owner.Id, nil,
				http.StatusOK, msgOrgRoleExists},
			{"a role reaches the organization or departments", CreateOrgRole, http.MethodPost, role("Mine", "self", "key.read"), owner.Id, nil,
				http.StatusOK, msgOrgRoleScopeInvalid},
			{"a role needs a permission", CreateOrgRole, http.MethodPost, role("Mine", "org"), owner.Id, nil,
				http.StatusOK, msgOrgRolePermissionsInvalid},
			{"an inherent power is not a permission", CreateOrgRole, http.MethodPost, role("Mine", "org", "key.read", orgmodel.PowerRoles), owner.Id, nil,
				http.StatusOK, msgOrgRolePermissionsInvalid},
			{"the audit log is not read by department", CreateOrgRole, http.MethodPost, role("Mine", "dept", "audit.read"), owner.Id, nil,
				http.StatusOK, msgOrgRoleAuditScope},
			{"a preset cannot be changed", UpdateOrgRole, http.MethodPut, role("Mine", "org", "key.read"), owner.Id, []gin.Param{idParam(adminRole)},
				http.StatusOK, msgOrgRolePreset},
			{"a preset cannot be deleted", DeleteOrgRole, http.MethodDelete, nil, owner.Id, []gin.Param{idParam(staffRole)},
				http.StatusOK, msgOrgRolePreset},
			{"an unknown role to change", UpdateOrgRole, http.MethodPut, role("Mine", "org", "key.read"), owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgRoleNotFound},
			{"an unknown role to delete", DeleteOrgRole, http.MethodDelete, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgRoleNotFound},
			{"an unknown role pack", AdoptOrgRolePack, http.MethodPost, map[string]any{}, owner.Id, []gin.Param{{Key: "key", Value: "no_such_pack"}},
				http.StatusOK, msgOrgRolePackNotFound},
			{"a key needs a name", CreateOrgKey, http.MethodPost, map[string]any{"name": "  "}, owner.Id, nil,
				http.StatusOK, msgOrgKeyNameInvalid},
			{"a key's quota cannot be negative", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "remain_quota": -1}, owner.Id, nil,
				http.StatusOK, msgOrgKeyQuotaInvalid},
			{"a key's limits cannot be negative", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "rpm_limit": -1}, owner.Id, nil,
				http.StatusOK, msgOrgKeyLimitInvalid},
			{"a key cannot be born expired", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "expired_time": 1}, owner.Id, nil,
				http.StatusOK, msgOrgKeyExpiryInvalid},
			{"a policy template must be one the platform offers", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "policy_template": "everything"}, owner.Id, nil,
				http.StatusOK, msgOrgKeyTemplateUnknown},
			{"a key takes a template or a list of models, not both", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "policy_template": "coding", "model_limits": []string{"gpt-4o"}}, owner.Id, nil,
				http.StatusOK, msgOrgKeyModelsWithTemplate},
			{"a key is for a member of the organization", CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "holder_id": solo.Id}, owner.Id, nil,
				http.StatusOK, msgOrgMemberNotFound},
			{"the models of a key are asked for a member", ListOrgKeyModels, http.MethodGet, nil, owner.Id, nil,
				http.StatusOK, msgOrgMemberNotFound},
			{"an unknown key to change", UpdateOrgKey, http.MethodPut, map[string]any{"name": "K"}, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgKeyNotFound},
			{"an unknown key to rotate", RotateOrgKey, http.MethodPost, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgKeyNotFound},
			{"an unknown key to freeze", FreezeOrgKey, http.MethodPost, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgKeyNotFound},
			{"an unknown key to unfreeze", UnfreezeOrgKey, http.MethodPost, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgKeyNotFound},
			{"an unknown key to delete", DeleteOrgKey, http.MethodDelete, nil, owner.Id, []gin.Param{idParam(424242)},
				http.StatusOK, msgOrgKeyNotFound},
			{"a key with no quota left stays frozen", UnfreezeOrgKey, http.MethodPost, nil, owner.Id, []gin.Param{idParam(spentKey.Id)},
				http.StatusOK, msgOrgKeyExhausted},
			{"an expired key stays frozen", UnfreezeOrgKey, http.MethodPost, nil, owner.Id, []gin.Param{idParam(expiredKey.Id)},
				http.StatusOK, msgOrgKeyExpired},
			{"a path id must be a number", DeleteOrgDepartment, http.MethodDelete, nil, owner.Id, []gin.Param{{Key: "id", Value: "abc"}},
				http.StatusOK, i18n.MsgInvalidParams},
			{"a body must be JSON", CreateOrgDepartment, http.MethodPost, nil, owner.Id, nil,
				http.StatusOK, i18n.MsgInvalidParams},
		}...)

		for _, tc := range cases {
			response, status := callOrg(t, tc.handler, tc.method, tc.body, tc.userID, tc.params...)
			require.False(t, response.Success, tc.name)
			require.Equal(t, tc.status, status, tc.name)
			require.Equal(t, tc.message, response.Message, tc.name)
			require.Empty(t, string(response.Data), "%s: a refusal carries no data", tc.name)
		}

		// None of it left a mark — in the audit log either.
		require.Equal(t, common.RoleCommonUser, env.userByName(t, "staff").Role)
		require.EqualValues(t, 1, env.count(t, &model.User{}, "org_id = ? AND role_id = ?", owner.OrgId, adminRole))
		require.EqualValues(t, 6, env.count(t, &orgmodel.Department{}, "org_id = ?", owner.OrgId))
		require.EqualValues(t, 1, env.count(t, &orgmodel.OrgRole{}, "org_id = ?", owner.OrgId))
		require.Zero(t, env.count(t, &model.User{}, "is_service = ?", true))
		require.Zero(t, env.count(t, &orgmodel.DepartmentManager{}))
		require.EqualValues(t, 2, env.count(t, &orgmodel.OrgInvite{}, "id IN ?", []int{intoProduct.Id, adminLink.Id}))
		require.Equal(t, auditBefore, env.count(t, &orgmodel.OrgAuditLog{}))
		for _, username := range []string{"founder", "admin"} {
			require.Equal(t, general.Id, env.userByName(t, username).DepartmentId, username)
		}
		require.Equal(t, staff.DepartmentId, env.userByName(t, "staff").DepartmentId)
		require.EqualValues(t, 1, env.count(t, &orgmodel.OrgRole{}, "id = ? AND name = ? AND permissions = ?", custom.Id, "Key Desk", "key.read"))
		require.EqualValues(t, 3, env.count(t, &model.Token{}, "org_id = ?", owner.OrgId), "no key was made, and none is gone")
		require.Equal(t, staffKey, env.key(t, staffKey.Id))
		require.Equal(t, common.TokenStatusDisabled, env.key(t, spentKey.Id).Status)
		require.Equal(t, common.TokenStatusDisabled, env.key(t, expiredKey.Id).Status)
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
