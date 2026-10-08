package controller

import (
	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Enterprise Org warnings and alerts (meta-repo docs/enterprise-org-prd.md §4):
// reading the organization's alert list, marking an alert as dealt with, and
// the settings the warnings and alerts run on. Raising and sending alerts is a
// background task (service/org_alerts.go); no request does either.

// ListOrgAlerts returns one page of the organization's alerts, newest first.
// ?state=open leaves out the ones somebody has dealt with.
func ListOrgAlerts(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	alerts, total, err := orgservice.ListAlerts(model.DB, actor, c.Query("state") == "open", pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		orgError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(alerts)
	common.ApiSuccess(c, pageInfo)
}

// orgAlertHandleRequest is the body of marking an alert: "handled" or
// "false_alarm".
type orgAlertHandleRequest struct {
	State string `json:"state"`
}

// HandleOrgAlert marks an alert as handled or as a false alarm.
func HandleOrgAlert(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	var req orgAlertHandleRequest
	if !orgBody(c, &req) {
		return
	}
	if err := orgservice.HandleAlert(model.DB, actor, id, req.State); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// GetOrgAlertSettings returns the organization's warning levels, alert rules
// and working hours.
func GetOrgAlertSettings(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	settings, err := orgservice.GetAlertSettings(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

// UpdateOrgAlertSettings replaces the organization's warning levels, alert
// rules and working hours, and answers with them as stored.
func UpdateOrgAlertSettings(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	var input orgmodel.AlertSettings
	if !orgBody(c, &input) {
		return
	}
	settings, err := orgservice.UpdateAlertSettings(model.DB, actor, input)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}
