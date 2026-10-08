package controller

import (
	"github.com/QuantumNous/new-api/common"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Enterprise Org usage report (meta-repo docs/enterprise-org-prd.md §6).

// GetOrgUsage returns the organization's usage over a period, summed by
// department, member, key or model (?group_by=), as far as the caller's role
// lets them see it. ?start_timestamp= and ?end_timestamp= bound the period and
// ?department_id= narrows it to one department — one outside the caller's
// reach is a 403.
func GetOrgUsage(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	start, end, ok := orgPeriod(c)
	if !ok {
		return
	}
	departmentID, ok := orgQueryNumber(c, "department_id")
	if !ok {
		return
	}
	report, err := orgservice.ReportUsage(model.DB, model.LOG_DB, actor, orgservice.UsageQuery{
		GroupBy:      c.Query("group_by"),
		Start:        start,
		End:          end,
		DepartmentId: int(departmentID),
	})
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}
