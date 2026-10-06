package service

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// AuditEntry is one management action to write to the audit log (PRD D17).
type AuditEntry struct {
	OrgId       int
	ActorUserId int
	Ip          string
	Action      string // one of the orgmodel.Audit… actions
	TargetType  string // one of the orgmodel.AuditTarget… types
	TargetId    int
	Before      any // what the target was; nil when it did not exist
	After       any // what it became; nil when it is gone
}

// auditChange is the JSON an audit record keeps in its detail column.
type auditChange struct {
	Before any `json:"before,omitempty"`
	After  any `json:"after,omitempty"`
}

// RecordAudit appends one record to the audit log. Management writes reach it
// through a permit (actor.go); it is exported for the few recorded events that
// are nobody's management action, such as someone joining by invite link.
//
// The log is append-only: nothing in this package updates or deletes a record.
// Pass the transaction of the change itself, so that an action and its record
// commit or roll back together.
func RecordAudit(tx *gorm.DB, entry AuditEntry) error {
	detail, err := common.Marshal(auditChange{Before: entry.Before, After: entry.After})
	if err != nil {
		return err
	}
	return tx.Create(&orgmodel.OrgAuditLog{
		OrgId:       entry.OrgId,
		ActorUserId: entry.ActorUserId,
		Action:      entry.Action,
		TargetType:  entry.TargetType,
		TargetId:    entry.TargetId,
		Detail:      string(detail),
		Ip:          entry.Ip,
		CreatedTime: common.GetTimestamp(),
	}).Error
}

// AuditLogView is one audit record as the organization reads it back.
type AuditLogView struct {
	Id          int             `json:"id"`
	ActorUserId int             `json:"actor_user_id"`
	Actor       string          `json:"actor"` // who that is today; empty when the account no longer exists
	Action      string          `json:"action"`
	TargetType  string          `json:"target_type"`
	TargetId    int             `json:"target_id"`
	Detail      json.RawMessage `json:"detail"`
	Ip          string          `json:"ip"`
	CreatedTime int64           `json:"created_time"`
}

// ListAuditLogs returns one page of the organization's audit log, newest
// first, and the total number of records. The log belongs to the organization
// as a whole, so reading it takes audit.read in a role that reaches all of it.
func ListAuditLogs(db *gorm.DB, actor *Actor, offset int, limit int) ([]AuditLogView, int64, error) {
	if err := actor.allow("audit.read", wholeOrganization); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := db.Model(&orgmodel.OrgAuditLog{}).Where("org_id = ?", actor.OrgId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []orgmodel.OrgAuditLog
	if err := db.Where("org_id = ?", actor.OrgId).Order("id DESC").
		Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	actorIDs := make([]int, 0, len(records))
	for _, record := range records {
		actorIDs = append(actorIDs, record.ActorUserId)
	}
	names, err := userNames(db, actorIDs)
	if err != nil {
		return nil, 0, err
	}
	views := make([]AuditLogView, 0, len(records))
	for _, record := range records {
		views = append(views, AuditLogView{
			Id:          record.Id,
			ActorUserId: record.ActorUserId,
			Actor:       names[record.ActorUserId],
			Action:      record.Action,
			TargetType:  record.TargetType,
			TargetId:    record.TargetId,
			Detail:      json.RawMessage(record.Detail),
			Ip:          record.Ip,
			CreatedTime: record.CreatedTime,
		})
	}
	return views, total, nil
}

// userNames returns what to call each user: the display name, or the username
// when they never set one. Accounts deleted since are still named — the log
// must go on saying who did what.
func userNames(db *gorm.DB, userIDs []int) (map[int]string, error) {
	names := make(map[int]string, len(userIDs))
	if len(userIDs) == 0 {
		return names, nil
	}
	var users []platformmodel.User
	if err := db.Unscoped().Select("id", "username", "display_name").
		Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		names[user.Id] = user.DisplayName
		if user.DisplayName == "" {
			names[user.Id] = user.Username
		}
	}
	return names, nil
}
