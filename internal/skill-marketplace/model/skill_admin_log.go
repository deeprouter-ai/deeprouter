package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	LogActionCreate          = "create"
	LogActionPublish         = "publish"
	LogActionRepublish       = "republish"
	LogActionDeprecate       = "deprecate"
	LogActionDelete          = "delete"
	LogActionFeaturedUpdate  = "featured_update"
	LogActionVersionUpload   = "version_upload"
	LogActionVersionActivate = "version_activate"
	LogActionVersionDelete   = "version_delete"
)

type SkillAdminLog struct {
	ID        int64           `gorm:"primarykey"        json:"id"`
	AdminID   int64           `gorm:"not null;index"    json:"admin_id"`
	SkillID   *int64          `gorm:"index"             json:"skill_id,omitempty"`
	Action    string          `gorm:"type:varchar(50);not null" json:"action"`
	Details   json.RawMessage `gorm:"type:jsonb;default:'{}'"   json:"details"`
	CreatedAt time.Time       `gorm:"not null;default:now()"    json:"created_at"`
}

func (SkillAdminLog) TableName() string { return "skill_admin_logs" }

func WriteLog(db *gorm.DB, adminID int64, skillID *int64, action string, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		raw = json.RawMessage("{}")
	}
	return db.Create(&SkillAdminLog{
		AdminID: adminID,
		SkillID: skillID,
		Action:  action,
		Details: raw,
	}).Error
}
