package service

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"gorm.io/gorm"
)

var (
	// ErrAlertNotFound means the alert is not one of this organization's.
	ErrAlertNotFound = errors.New("alert not found in this organization")
	// ErrInvalidAlertState means an alert was to be marked as something other
	// than handled or a false alarm.
	ErrInvalidAlertState = errors.New("an alert is marked handled or a false alarm")
)

// AlertView is one alert as the organization reads it back.
type AlertView struct {
	Id   int    `json:"id"`
	Rule string `json:"rule"`
	// Level is the share a warning is about, in percent; 0 for an anomaly.
	Level int `json:"level"`
	KeyId int `json:"key_id"`
	// HolderId, Holder, DepartmentId and Department are who held the key when
	// the alert was raised and where they sat, named as they are called today.
	HolderId     int    `json:"holder_id"`
	Holder       string `json:"holder"`
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
	// Detail is the numbers behind the alert — orgmodel.AlertDetail, which
	// also carries the key's name.
	Detail      json.RawMessage `json:"detail"`
	CreatedTime int64           `json:"created_time"`
	State       string          `json:"state"`
	AckedBy     int             `json:"acked_by"`
	AckedByName string          `json:"acked_by_name"`
	AckedTime   int64           `json:"acked_time"`
}

// alertRecord is what the audit log keeps of an alert being dealt with.
type alertRecord struct {
	Rule  string `json:"rule"`
	Key   string `json:"key"`
	State string `json:"state"`
}

// departmentNamesEver is departmentNames with the deleted departments too: an
// alert goes on naming the department it was raised in.
func departmentNamesEver(db *gorm.DB, orgID int) (map[int]string, error) {
	var departments []orgmodel.Department
	if err := db.Unscoped().Select("id", "name").Where("org_id = ?", orgID).Find(&departments).Error; err != nil {
		return nil, err
	}
	names := make(map[int]string, len(departments))
	for _, department := range departments {
		names[department.Id] = department.Name
	}
	return names, nil
}

// alertDetail reads back the numbers an alert was raised with.
func alertDetail(alert *orgmodel.OrgAlert) orgmodel.AlertDetail {
	var detail orgmodel.AlertDetail
	// What is stored was written by this package; an alert whose detail does
	// not parse still has a rule and a key to show.
	_ = common.UnmarshalJsonStr(alert.Detail, &detail)
	return detail
}

// ListAlerts returns one page of the organization's alerts, newest first, and
// how many there are. How much of the list the caller is sent follows their
// role and is part of the query: every alert for a role that reads alerts
// across the organization, and the ones raised on keys held in its departments
// for a department-scoped one. Everyone else is sent the warnings raised on
// the keys they held themselves (PRD D47) — what they are notified of, and not
// the anomalies, which stay with whoever looks into them. openOnly leaves out
// the alerts somebody has dealt with.
func ListAlerts(db *gorm.DB, actor *Actor, openOnly bool, offset int, limit int) ([]AlertView, int64, error) {
	everywhere, departments := orgmodel.Reach(actor.Subject, "alert.read")
	ownOnly := !everywhere && len(departments) == 0
	if ownOnly {
		if err := actor.allow("alert.read", orgmodel.Target{UserId: actor.UserId}); err != nil {
			return nil, 0, err
		}
	}
	within := func() *gorm.DB {
		query := db.Model(&orgmodel.OrgAlert{}).Where("org_id = ?", actor.OrgId)
		switch {
		case everywhere:
		case ownOnly:
			query = query.Where("user_id = ? AND rule IN ?", actor.UserId, orgmodel.WarningRules)
		default:
			query = query.Where("department_id IN ?", departments)
		}
		if openOnly {
			query = query.Where("state = ?", orgmodel.AlertStateOpen)
		}
		return query
	}
	var total int64
	if err := within().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var alerts []orgmodel.OrgAlert
	if err := within().Order("id DESC").Offset(offset).Limit(limit).Find(&alerts).Error; err != nil {
		return nil, 0, err
	}
	userIDs := make([]int, 0, 2*len(alerts))
	for _, alert := range alerts {
		userIDs = append(userIDs, alert.UserId, alert.AckedBy)
	}
	names, err := userNames(db, userIDs)
	if err != nil {
		return nil, 0, err
	}
	departmentName, err := departmentNamesEver(db, actor.OrgId)
	if err != nil {
		return nil, 0, err
	}
	views := make([]AlertView, 0, len(alerts))
	for _, alert := range alerts {
		views = append(views, AlertView{
			Id:           alert.Id,
			Rule:         alert.Rule,
			Level:        alert.Level,
			KeyId:        alert.TokenId,
			HolderId:     alert.UserId,
			Holder:       names[alert.UserId],
			DepartmentId: alert.DepartmentId,
			Department:   departmentName[alert.DepartmentId],
			Detail:       json.RawMessage(alert.Detail),
			CreatedTime:  alert.CreatedTime,
			State:        alert.State,
			AckedBy:      alert.AckedBy,
			AckedByName:  names[alert.AckedBy],
			AckedTime:    alert.AckedTime,
		})
	}
	return views, total, nil
}

// HandleAlert marks an alert as dealt with — handled, or a false alarm — which
// is the owner's and the admins' to do (PRD §2). Marking it what it already is
// changes nothing and records nothing.
func HandleAlert(db *gorm.DB, actor *Actor, alertID int, state string) error {
	permit, err := actor.permit(orgmodel.PowerAlerts, wholeOrganization)
	if err != nil {
		return err
	}
	if state != orgmodel.AlertStateHandled && state != orgmodel.AlertStateFalseAlarm {
		return ErrInvalidAlertState
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var found []orgmodel.OrgAlert
		if err := tx.Where("id = ? AND org_id = ?", alertID, actor.OrgId).Limit(1).Find(&found).Error; err != nil {
			return err
		}
		if len(found) == 0 {
			return ErrAlertNotFound
		}
		alert := found[0]
		if alert.State == state {
			return nil
		}
		if err := tx.Model(&orgmodel.OrgAlert{}).Where("id = ?", alert.Id).Updates(map[string]any{
			"state":      state,
			"acked_by":   actor.UserId,
			"acked_time": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		key := alertDetail(&alert).Key
		return permit.record(tx, orgmodel.AuditAlertHandle, orgmodel.AuditTargetAlert, alert.Id,
			alertRecord{Rule: alert.Rule, Key: key, State: alert.State},
			alertRecord{Rule: alert.Rule, Key: key, State: state})
	})
}
