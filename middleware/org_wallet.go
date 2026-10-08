package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// attachOrgSpend puts on the request whose balance an organization key spends
// and which organization and department its usage belongs to (Enterprise Org,
// meta-repo docs/enterprise-org-prd.md §7.4). TokenAuth calls it for a key
// whose org_id is set and for no other, so a personal key's request carries
// none of this and is billed as it always was.
//
// It reports false after answering the request itself: a key whose
// organization cannot be charged is refused here, before anything is spent.
func attachOrgSpend(c *gin.Context, token *model.Token) bool {
	spend, err := orgservice.SpendOf(model.DB, token.OrgId, token.UserId)
	if errors.Is(err, orgservice.ErrNoWallet) {
		abortWithOpenAiMessage(c, http.StatusForbidden,
			common.TranslateMessage(c, i18n.MsgOrgWalletUnavailable), types.ErrorCodeAccessDenied)
		return false
	}
	if err != nil {
		common.SysLog(fmt.Sprintf("TokenAuth SpendOf error for key %d of organization %d: %v", token.Id, token.OrgId, err))
		abortWithOpenAiMessage(c, http.StatusInternalServerError,
			common.TranslateMessage(c, i18n.MsgDatabaseError))
		return false
	}
	common.SetContextKey(c, constant.ContextKeyOrgId, spend.OrgId)
	common.SetContextKey(c, constant.ContextKeyOrgDepartmentId, spend.DepartmentId)
	common.SetContextKey(c, constant.ContextKeyOrgWalletUserId, spend.WalletUserId)
	return true
}
