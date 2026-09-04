package model

import (
	"time"

	"github.com/lib/pq"
)

const (
	SkillStatusDraft      = "draft"
	SkillStatusPublished  = "published"
	SkillStatusDeprecated = "deprecated"
)

type Skill struct {
	ID          int64  `gorm:"primarykey"                             json:"id"`
	Slug        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	Name        string `gorm:"type:varchar(200);not null"             json:"name"`
	Description string `gorm:"type:text;not null"                     json:"description"`
	Category    string `gorm:"type:varchar(50);not null"              json:"category"`
	// pq.StringArray (not plain []string) is required for GORM/pgx to
	// correctly read and write a Postgres text[] column — a bare
	// []string has no driver.Valuer/sql.Scanner for array literals, so
	// every write silently landed as SQL NULL instead of '{}' or the
	// real tags, and every read crashed the admin edit page the moment
	// a null tags value hit formatTagsInput() client-side (found during
	// the P5 real-service walkthrough, 2026-09-04).
	Tags             pq.StringArray `gorm:"type:text[];default:'{}'"                 json:"tags"`
	Status           string         `gorm:"type:varchar(20);not null;default:draft"  json:"status"`
	MonetizationType string         `gorm:"type:varchar(10);not null;default:free"   json:"monetization_type"`
	PriceUSD         float64        `gorm:"type:numeric(10,2);not null;default:0"    json:"price_usd"`
	FeaturedFlag     bool           `gorm:"default:false"                            json:"featured_flag"`
	FeaturedRank     int            `gorm:"default:0"                                json:"featured_rank"`
	ActiveVersionID  *int64         `gorm:"column:active_version_id"                 json:"active_version_id,omitempty"`
	CreatedBy        int            `gorm:"not null"                                 json:"created_by"`
	CreatedAt        time.Time      `gorm:"not null;default:now()"                   json:"created_at"`
	UpdatedAt        time.Time      `gorm:"not null;default:now()"                   json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }
