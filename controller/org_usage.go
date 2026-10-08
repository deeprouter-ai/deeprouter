package controller

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Enterprise Org usage report and trend (meta-repo docs/enterprise-org-prd.md §6).

// orgUsageQuery reads what a usage report or trend is asked for: ?group_by=,
// the period (?start_timestamp=, ?end_timestamp=) and ?department_id=. When one
// of them cannot be read it answers the request and reports false.
func orgUsageQuery(c *gin.Context) (orgservice.UsageQuery, bool) {
	start, end, ok := orgPeriod(c)
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	departmentID, ok := orgQueryNumber(c, "department_id")
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	return orgservice.UsageQuery{
		GroupBy:      c.Query("group_by"),
		Start:        start,
		End:          end,
		DepartmentId: int(departmentID),
	}, true
}

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
	query, ok := orgUsageQuery(c)
	if !ok {
		return
	}
	report, err := orgservice.ReportUsage(model.DB, model.LOG_DB, actor, query)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

// GetOrgUsageTrend returns what the organization spent over a period, cut into
// days, weeks, months or years (?bucket=) on the calendar of the viewer's time
// zone (?timezone=, an IANA name), with one series per department or per model
// (?group_by=). The period, the department filter and how much of the company
// the caller is shown are those of GetOrgUsage.
func GetOrgUsageTrend(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	query, ok := orgUsageQuery(c)
	if !ok {
		return
	}
	trend, err := orgservice.TrendUsage(model.DB, model.LOG_DB, actor, orgservice.UsageTrendQuery{
		UsageQuery: query,
		Bucket:     c.Query("bucket"),
		Timezone:   c.Query("timezone"),
	}, time.Now())
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, trend)
}
