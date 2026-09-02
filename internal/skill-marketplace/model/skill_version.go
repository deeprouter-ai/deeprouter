package model

import (
	"encoding/json"
	"time"
)

const (
	SkillVersionStatusDraft    = "draft"
	SkillVersionStatusActive   = "active"
	SkillVersionStatusArchived = "archived"
)

type SkillVersion struct {
	ID             int64           `gorm:"primarykey"                              json:"id"`
	SkillID        int64           `gorm:"not null;index"                          json:"skill_id"`
	Version        string          `gorm:"type:varchar(20);not null"               json:"version"`
	Status         string          `gorm:"type:varchar(20);not null;default:draft" json:"status"`
	SkillMDContent string          `gorm:"type:text;not null"                      json:"skill_md_content"`
	ManifestJSON   json.RawMessage `gorm:"type:jsonb;not null"                     json:"manifest_json"`
	PackageZip     []byte          `gorm:"type:bytea"                              json:"-"`
	PackageSHA256  string          `gorm:"type:varchar(64)"                        json:"package_sha256,omitempty"`
	PackageBuiltAt *time.Time      `gorm:"column:package_built_at"                 json:"package_built_at,omitempty"`
	Changelog      string          `gorm:"type:text;default:''"                    json:"changelog"`
	CreatedBy      int             `gorm:"not null"                                json:"created_by"`
	CreatedAt      time.Time       `gorm:"not null;default:now()"                  json:"created_at"`
}

func (SkillVersion) TableName() string { return "skill_versions" }
