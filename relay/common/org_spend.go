package common

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

// OrgSpend says whose balance pays for a request and which organization and
// department its usage belongs to (Enterprise Org, meta-repo
// docs/enterprise-org-prd.md §7.4). TokenAuth resolves it for a request made
// with an organization key. It is all zeroes for every other request, and a
// zero OrgSpend changes nothing: that is what keeps a personal account's
// billing exactly as it was.
//
// An asynchronous task keeps a copy (model.TaskPrivateData), because its
// refund or its final settlement happens long after the request is gone and
// must reach the account that paid.
type OrgSpend struct {
	// OrgId is the organization the key belongs to; 0 for a personal key.
	OrgId int `json:"org_id,omitempty"`
	// OrgDepartmentId is the department the key's holder was in when the
	// request was made. The usage log is stamped with it.
	OrgDepartmentId int `json:"org_department_id,omitempty"`
	// OrgWalletUserId is the organization's owner: the company wallet is their
	// balance.
	OrgWalletUserId int `json:"org_wallet_user_id,omitempty"`
}

// orgSpendFromContext reads what TokenAuth resolved for an organization key.
func orgSpendFromContext(c *gin.Context) OrgSpend {
	return OrgSpend{
		OrgId:           common.GetContextKeyInt(c, constant.ContextKeyOrgId),
		OrgDepartmentId: common.GetContextKeyInt(c, constant.ContextKeyOrgDepartmentId),
		OrgWalletUserId: common.GetContextKeyInt(c, constant.ContextKeyOrgWalletUserId),
	}
}

// WalletUserId is the account whose balance pays for the request: the company
// wallet for an organization key, the user's own account for anything else.
// What was spent is still recorded under UserId, the person who spent it.
func (info *RelayInfo) WalletUserId() int {
	if info.OrgWalletUserId != 0 {
		return info.OrgWalletUserId
	}
	return info.UserId
}
