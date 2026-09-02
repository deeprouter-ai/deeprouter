package model

import "time"

type UserEnabledSkill struct {
	ID        int64     `gorm:"primarykey"             json:"id"`
	UserID    int64     `gorm:"not null;index"         json:"user_id"`
	SkillID   int64     `gorm:"not null"               json:"skill_id"`
	VersionID int64     `gorm:"not null"               json:"version_id"`
	EnabledAt time.Time `gorm:"not null;default:now()" json:"enabled_at"`
}

func (UserEnabledSkill) TableName() string { return "user_enabled_skills" }

type SkillPurchase struct {
	ID            int64     `gorm:"primarykey"                  json:"id"`
	UserID        int64     `gorm:"not null;index"              json:"user_id"`
	SkillID       int64     `gorm:"not null;index"              json:"skill_id"`
	PriceUSD      float64   `gorm:"type:numeric(10,2);not null" json:"price_usd"`
	QuotaDeducted int64     `gorm:"not null"                    json:"quota_deducted"`
	PurchasedAt   time.Time `gorm:"not null;default:now()"      json:"purchased_at"`
}

func (SkillPurchase) TableName() string { return "skill_purchases" }
