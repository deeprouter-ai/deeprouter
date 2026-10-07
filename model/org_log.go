package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	commonRelay "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

// Enterprise Org (meta-repo docs/enterprise-org-prd.md §7.4): a usage log line
// of an organization key is stamped with the organization and with the
// department its user was in at that moment. Reports group by the stamp, so
// moving someone to another department never rewrites what they already spent.
// A line that is not an organization key's keeps both columns at 0.

// stampOrgFromRequest labels a log line written while the request is still
// here, from what TokenAuth resolved for an organization key.
func (log *Log) stampOrgFromRequest(c *gin.Context) {
	log.OrgId = common.GetContextKeyInt(c, constant.ContextKeyOrgId)
	log.DepartmentId = common.GetContextKeyInt(c, constant.ContextKeyOrgDepartmentId)
}

// stampOrg labels a log line written after the request is gone — a task's
// refund or its final settlement — from the copy the task kept.
func (log *Log) stampOrg(spend commonRelay.OrgSpend) {
	log.OrgId = spend.OrgId
	log.DepartmentId = spend.OrgDepartmentId
}
