package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// Enterprise Org D48 (meta-repo docs/enterprise-org-prd.md §6): a member
// changes their own display name from the profile page. PUT /api/user/self
// with nothing but display_name takes no password — the name is what the
// member list, the reports and the alerts call the person, not a credential —
// and changes nothing else.

// signedUpMember is an account with a password, as sign-up leaves one.
func signedUpMember(t *testing.T, env orgTestEnv, username string) model.User {
	t.Helper()
	hashed, err := common.Password2Hash("password123")
	require.NoError(t, err)
	member := model.User{Username: username, Password: hashed, DisplayName: strings.ToUpper(username[:1]) + username[1:],
		Role: common.RoleCommonUser, AffCode: "AF-" + username, OrgId: 3, OrgRoleId: 4, DepartmentId: 5}
	require.NoError(t, env.db.Create(&member).Error)
	return member
}

// rename asks to change the caller's display name and nothing else.
func rename(t *testing.T, userID int, body map[string]any) tokenAPIResponse {
	t.Helper()
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", body, userID)
	UpdateSelf(ctx)
	return decodeAPIResponse(t, recorder)
}

func TestUpdateSelfDisplayName_TakesNoPassword(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		member := signedUpMember(t, env, "wang")
		response := rename(t, member.Id, map[string]any{"display_name": " 王芳 "})
		require.True(t, response.Success, response.Message)

		reloaded := env.userByName(t, "wang")
		require.Equal(t, "王芳", reloaded.DisplayName, "trimmed, and nothing asked for the password")
		// Everything else is exactly what it was.
		require.Equal(t, member.Password, reloaded.Password)
		require.Equal(t, "wang", reloaded.Username)
		require.Equal(t, common.RoleCommonUser, reloaded.Role)
		require.Equal(t, 3, reloaded.OrgId)
		require.Equal(t, 4, reloaded.OrgRoleId)
		require.Equal(t, 5, reloaded.DepartmentId)
	})
}

// A name that is empty, blank or longer than twenty characters is refused
// with its own message, and the old name stays.
func TestUpdateSelfDisplayName_RefusesAnEmptyOrOverlongName(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		member := signedUpMember(t, env, "pat")
		for name, body := range map[string]map[string]any{
			"empty":            {"display_name": ""},
			"blank":            {"display_name": "   "},
			"twenty-one runes": {"display_name": strings.Repeat("名", 21)},
			"not a string":     {"display_name": 42},
			"null":             {"display_name": nil},
		} {
			response := rename(t, member.Id, body)
			require.False(t, response.Success, name)
			require.Equal(t, "user.display_name_invalid", response.Message, name)
			require.Equal(t, "Pat", env.userByName(t, "pat").DisplayName, name)
		}
		// Twenty characters is the most, and exactly twenty is fine.
		require.True(t, rename(t, member.Id, map[string]any{"display_name": strings.Repeat("名", 20)}).Success)
		require.Equal(t, strings.Repeat("名", 20), env.userByName(t, "pat").DisplayName)
	})
}

// Only a display name on its own goes without the password. The moment the
// request carries anything else, it is the usual update and asks for the
// current password, as it always did (TestOrgUserFields_AreNotWritableThroughUpdateSelf).
func TestUpdateSelfDisplayName_AnythingMoreStillTakesThePassword(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		member := signedUpMember(t, env, "lee")
		for name, body := range map[string]map[string]any{
			"with a new username":   {"display_name": "Lee Who", "username": "someone-else"},
			"with a new password":   {"display_name": "Lee Who", "password": "newpassword123"},
			"with a smuggled field": {"display_name": "Lee Who", "role": common.RoleRootUser},
		} {
			response := rename(t, member.Id, body)
			require.False(t, response.Success, name)
			reloaded := env.userByName(t, "lee")
			require.Equal(t, "Lee", reloaded.DisplayName, name)
			require.Equal(t, member.Password, reloaded.Password, name)
			require.Equal(t, common.RoleCommonUser, reloaded.Role, name)
		}
	})
}
