package service

// Enterprise Org on the billing path (meta-repo docs/enterprise-org-prd.md
// §7.4): a request made with an organization key is paid from the company
// wallet — the balance of the organization's owner — while what was spent
// stays recorded under the member who spent it.
//
// The branch itself is one method, relaycommon.RelayInfo.WalletUserId: the
// places that take money from a balance or put it back ask it whose balance,
// where they used to say relayInfo.UserId. For a personal key the answer is
// still relayInfo.UserId, so nothing changes for it. This file holds what
// comes with paying from somebody else's balance: an error that does not tell
// the member what the company has left, a reminder that goes to the people who
// run the organization, and the same answer for a task that settles long after
// its request is over.

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// orgWalletError rewrites "not enough balance" for a request made with an
// organization key. The platform's own message quotes the balance that was
// found too small — the company's here, which is the owner's to know and not
// the member's whose call was refused. The error code is kept, so a client
// that reacts to it still does. Every other error, and every error of a
// personal key, passes through untouched.
func orgWalletError(c *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.NewAPIError) *types.NewAPIError {
	if relayInfo.OrgId == 0 || apiErr.GetErrorCode() != types.ErrorCodeInsufficientUserQuota {
		return apiErr
	}
	return types.NewErrorWithStatusCode(
		errors.New(common.TranslateMessage(c, i18n.MsgOrgWalletInsufficient)),
		types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
		types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

// notifyOrgWalletLow tells the owner and the admins of an organization that
// the company wallet is running low, in place of the reminder a personal
// account gets about its own balance. The member whose request was just
// settled hears nothing: the balance is not theirs.
//
// "Low" is the wallet holder's own warning threshold, or the platform's when
// they set none. Each watcher is told through the channel of their own
// settings, in their own language, and the per-user notification limit applies
// to each as it does to anyone.
func notifyOrgWalletLow(relayInfo *relaycommon.RelayInfo) {
	orgID, walletUserID := relayInfo.OrgId, relayInfo.OrgWalletUserId
	gopool.Go(func() {
		// Read now, not taken from relayInfo: its UserQuota is the wallet's
		// balance only on the paths that checked it before the request.
		wallet, err := model.GetUserCache(walletUserID)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to read the wallet of organization %d: %s", orgID, err.Error()))
			return
		}
		threshold := common.QuotaRemindThreshold
		if custom := wallet.GetSetting().QuotaWarningThreshold; custom != 0 {
			threshold = int(custom)
		}
		if wallet.Quota >= threshold {
			return
		}
		watchers, err := orgservice.WalletWatchers(model.DB, orgID, walletUserID)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to list who watches the wallet of organization %d: %s", orgID, err.Error()))
			return
		}
		for _, watcher := range watchers {
			setting := watcher.GetSetting()
			if err := sendWalletNotice(watcher.Id, watcher.Email, setting, orgWalletLowNotice(setting, wallet.Quota)); err != nil {
				common.SysError(fmt.Sprintf("failed to send wallet notify to user %d: %s", watcher.Id, err.Error()))
			}
		}
	})
}

// sendWalletNotice delivers the reminder to one watcher. It is NotifyUser; a
// variable so that tests can see who was told what without sending anything.
var sendWalletNotice = NotifyUser

// orgWalletLowNotice words the low-balance reminder for one watcher: in the
// language they saved, as plain text for the channels that cannot show HTML.
func orgWalletLowNotice(setting dto.UserSetting, quota int) dto.Notify {
	body := i18n.MsgOrgWalletLowBodyHTML
	if setting.NotifyType == dto.NotifyTypeBark || setting.NotifyType == dto.NotifyTypeGotify {
		body = i18n.MsgOrgWalletLowBody
	}
	details := map[string]any{
		"Balance": logger.FormatQuota(quota),
		"Link":    fmt.Sprintf("%s/console/topup", system_setting.ServerAddress),
	}
	return dto.NewNotify(dto.NotifyTypeQuotaExceed,
		i18n.Translate(setting.Language, i18n.MsgOrgWalletLowTitle),
		i18n.Translate(setting.Language, body, details), nil)
}

// taskWalletUserId is RelayInfo.WalletUserId for a task that is refunded or
// settled after its request is gone: the account that paid when it was
// submitted — the company wallet for an organization key, the task's own user
// for anything else.
func taskWalletUserId(task *model.Task) int {
	if task.PrivateData.OrgWalletUserId != 0 {
		return task.PrivateData.OrgWalletUserId
	}
	return task.UserId
}
