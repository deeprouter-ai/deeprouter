package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// Enterprise Org P7 (meta-repo docs/enterprise-org-prd.md §4): what the
// console needs so that a member is not shown a balance their keys do not
// spend. The billing itself is tested in service/org_wallet_test.go, and end
// to end through the gateway in router/org_wallet_test.go.

// The profile of the signed-in user says which organization the account
// belongs to, so the console knows from the first page — before it has asked
// anything else — that the balance in that same answer is not what a member's
// keys spend. A personal account's answer has no such field at all: it is
// what it was before organizations existed.
func TestOrgSelf_TheProfileNamesTheOrganizationOfAMember(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		for _, signUp := range []map[string]any{
			{"username": "solo", "password": "password123"},
			{"username": "founder", "password": "password123", "org_name": "Acme"},
		} {
			response, _ := env.register(t, signUp)
			require.True(t, response.Success, response.Message)
		}
		profile := func(username string) map[string]any {
			user := env.userByName(t, username)
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, user.Id)
			ctx.Set("role", user.Role)
			GetSelf(ctx)
			answer := decodeAPIResponse(t, recorder)
			require.True(t, answer.Success, answer.Message)
			var data map[string]any
			require.NoError(t, common.Unmarshal(answer.Data, &data))
			return data
		}

		founder := profile("founder")
		require.EqualValues(t, env.userByName(t, "founder").OrgId, founder["org_id"])
		require.NotZero(t, founder["org_id"])

		personal := profile("solo")
		require.NotContains(t, personal, "org_id")
		// Everything else is there for both, the balance included: hiding it
		// from a member is the console's job, and the owner's is the wallet.
		for _, field := range []string{"id", "username", "quota", "used_quota", "request_count", "setting"} {
			require.Contains(t, personal, field)
			require.Contains(t, founder, field)
		}
	})
}
