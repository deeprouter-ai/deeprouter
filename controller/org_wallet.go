package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// Enterprise Org: the two ways into the relay that the company wallet cannot
// follow (meta-repo docs/enterprise-org-prd.md §7.4). An organization key
// spends the company wallet and its usage is stamped; these two entrances
// could do neither, so they are closed to organization accounts rather than
// left to bill the wrong balance.

// orgPlaygroundRefusal closes the playground endpoint to members of an
// organization. The playground sends requests with no key at all, on the
// signed-in user's own balance: a member's spending there would never reach
// the company's reports (PRD D16), and the owner's would leave the company
// wallet with nothing to show for it. Its page is gone from the console
// already; this is the endpoint behind it. A personal account passes
// untouched and gets nil.
func orgPlaygroundRefusal(c *gin.Context) *types.NewAPIError {
	orgID, err := callerOrgID(c)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if orgID == 0 {
		return nil
	}
	return types.NewErrorWithStatusCode(
		errors.New(common.TranslateMessage(c, i18n.MsgOrgPlaygroundClosed)),
		types.ErrorCodeAccessDenied, http.StatusForbidden, types.ErrOptionWithSkipRetry())
}

// refuseOrgKeyForMidjourney answers a Midjourney proxy request made with an
// organization key and reports true. A Midjourney task has nowhere to record
// who paid for it, so the refund of a failed one goes to the task's user: for
// an organization key that would move the company's money into a member's own
// account. Until the task can say who paid, organization keys are not served
// here. Every other key passes untouched.
func refuseOrgKeyForMidjourney(c *gin.Context, relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo.OrgId == 0 {
		return false
	}
	c.JSON(http.StatusForbidden, gin.H{
		"description": "organization_key_not_supported",
		"type":        "new_api_error",
		"code":        constant.MjRequestError,
	})
	return true
}
