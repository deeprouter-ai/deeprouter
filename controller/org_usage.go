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
// the period (?start_timestamp=, ?end_timestamp=), ?department_id=, and the
// one member (?user_id=) or key (?token_id=) it is narrowed to. When one of
// them cannot be read it answers the request and reports false.
func orgUsageQuery(c *gin.Context) (orgservice.UsageQuery, bool) {
	start, end, ok := orgPeriod(c)
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	departmentID, ok := orgQueryNumber(c, "department_id")
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	userID, ok := orgQueryNumber(c, "user_id")
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	tokenID, ok := orgQueryNumber(c, "token_id")
	if !ok {
		return orgservice.UsageQuery{}, false
	}
	return orgservice.UsageQuery{
		GroupBy:      c.Query("group_by"),
		Start:        start,
		End:          end,
		DepartmentId: int(departmentID),
		UserId:       int(userID),
		TokenId:      int(tokenID),
	}, true
}

// GetOrgUsage returns the organization's usage over a period, summed by
// department, member, key or model (?group_by=), as far as the caller's role
// lets them see it. ?start_timestamp= and ?end_timestamp= bound the period;
// ?department_id= narrows it to one department — one outside the caller's
// reach is a 403 — and ?user_id= or ?token_id= to what one member or one key
// spent of what the caller may see.
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
// zone (?timezone=, an IANA name), with one series per department, model,
// member or key (?group_by=) — narrowed to one member or key, that one's
// line. The period, the filters and how much of the company the caller is
// shown are those of GetOrgUsage.
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
