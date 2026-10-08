package service

import (
	"encoding/json"
	"strings"

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
	Id          int    `json:"id"`
	ActorUserId int    `json:"actor_user_id"`
	Actor       string `json:"actor"` // who that is today; empty when the account no longer exists
	Action      string `json:"action"`
	TargetType  string `json:"target_type"`
	TargetId    int    `json:"target_id"`
	// Target is what it was done to, by name: a member as they are called
	// today, an alert by the key it is about, anything else by the name its
	// record carries. Empty when the record has none, as for an invite link.
	Target      string          `json:"target"`
	Detail      json.RawMessage `json:"detail"`
	Ip          string          `json:"ip"`
	CreatedTime int64           `json:"created_time"`
}

// AuditFilter narrows the audit log to the records someone is looking for.
// A zero field does not narrow.
type AuditFilter struct {
	// Actor is part of the name — display name or username, in any case — of
	// whoever did it.
	Actor string
	// TargetType is one of the orgmodel.AuditTarget… types.
	TargetType string
	// Start and End bound when it was done, in Unix seconds, both included.
	Start int64
	End   int64
}

// likeEscape is the escape character of the LIKE patterns in this package.
const likeEscape = "!"

// containsPattern returns the LIKE pattern that matches whatever contains
// text, with the characters LIKE gives a meaning to taken literally.
func containsPattern(text string) string {
	escaped := strings.NewReplacer(likeEscape, likeEscape+likeEscape, "%", likeEscape+"%", "_", likeEscape+"_").Replace(text)
	return "%" + escaped + "%"
}

// ListAuditLogs returns one page of the organization's audit log, newest
// first, and the total number of records the filter lets through. The log
// belongs to the organization as a whole, so reading it takes audit.read in a
// role that reaches all of it.
func ListAuditLogs(db *gorm.DB, actor *Actor, filter AuditFilter, offset int, limit int) ([]AuditLogView, int64, error) {
	if err := actor.allow("audit.read", wholeOrganization); err != nil {
		return nil, 0, err
	}
	within := func() *gorm.DB {
		query := db.Model(&orgmodel.OrgAuditLog{}).Where("org_id = ?", actor.OrgId)
		if name := strings.ToLower(strings.TrimSpace(filter.Actor)); name != "" {
			// Unscoped: what a member did stays findable by their name after
			// they have been removed.
			named := db.Unscoped().Model(&platformmodel.User{}).Select("id").
				Where("org_id = ?", actor.OrgId).
				Where("(LOWER(display_name) LIKE ? ESCAPE '"+likeEscape+"' OR LOWER(username) LIKE ? ESCAPE '"+likeEscape+"')",
					containsPattern(name), containsPattern(name))
			query = query.Where("actor_user_id IN (?)", named)
		}
		if filter.TargetType != "" {
			query = query.Where("target_type = ?", filter.TargetType)
		}
		if filter.Start > 0 {
			query = query.Where("created_time >= ?", filter.Start)
		}
		if filter.End > 0 {
			query = query.Where("created_time <= ?", filter.End)
		}
		return query
	}
	var total int64
	if err := within().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []orgmodel.OrgAuditLog
	if err := within().Order("id DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	// A record about a member says what changed and not who they are, so
	// members are named from the accounts, along with the actors.
	userIDs := make([]int, 0, 2*len(records))
	for _, record := range records {
		userIDs = append(userIDs, record.ActorUserId)
		if record.TargetType == orgmodel.AuditTargetMember {
			userIDs = append(userIDs, record.TargetId)
		}
	}
	names, err := userNames(db, userIDs)
	if err != nil {
		return nil, 0, err
	}
	views := make([]AuditLogView, 0, len(records))
	for _, record := range records {
		target := recorded(record.Detail, "name")
		switch record.TargetType {
		case orgmodel.AuditTargetMember:
			target = names[record.TargetId]
		case orgmodel.AuditTargetAlert:
			// An alert has no name of its own; it is known by its key.
			target = recorded(record.Detail, "key")
		}
		views = append(views, AuditLogView{
			Id:          record.Id,
			ActorUserId: record.ActorUserId,
			Actor:       names[record.ActorUserId],
			Action:      record.Action,
			TargetType:  record.TargetType,
			TargetId:    record.TargetId,
			Target:      target,
			Detail:      json.RawMessage(record.Detail),
			Ip:          record.Ip,
			CreatedTime: record.CreatedTime,
		})
	}
	return views, total, nil
}

// recorded returns what an audit record holds in one field of its target, as
// text: the value after the change or, when the target is gone, before it.
func recorded(detail string, field string) string {
	var change struct {
		Before map[string]any `json:"before"`
		After  map[string]any `json:"after"`
	}
	// A record whose detail does not parse still has an action and a target
	// type to show.
	_ = common.UnmarshalJsonStr(detail, &change)
	if value, ok := change.After[field].(string); ok && value != "" {
		return value
	}
	value, _ := change.Before[field].(string)
	return value
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
