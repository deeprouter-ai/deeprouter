package model

import (
	"time"

	"github.com/lib/pq"
)

const (
	SkillStatusDraft      = "draft"
	SkillStatusPublished  = "published"
	SkillStatusDeprecated = "deprecated"

	SkillListingTypeHosted    = "hosted"
	SkillListingTypeReference = "reference"
)

type Skill struct {
	ID          int64  `gorm:"primarykey"                               json:"id"`
	Slug        string `gorm:"type:varchar(100);uniqueIndex;not null"   json:"slug"`
	Name        string `gorm:"type:varchar(200);not null"               json:"name"`
	Description string `gorm:"type:text;not null"                       json:"description"`
	// pq.StringArray, not []string: a bare Go slice reaches PG as a record
	// (SQLSTATE 42804) and cannot be scanned back — creating a skill failed
	// on every real PG database (found 2026-09-04 during P3 verification).
	Tags             pq.StringArray `gorm:"type:text[];default:'{}'"                 json:"tags"`
	Status           string         `gorm:"type:varchar(20);not null;default:draft"  json:"status"`
	MonetizationType string         `gorm:"type:varchar(10);not null;default:free"   json:"monetization_type"`
	PriceUSD         float64        `gorm:"type:numeric(10,2);not null;default:0"    json:"price_usd"`
	FeaturedFlag     bool           `gorm:"default:false"                            json:"featured_flag"`
	FeaturedRank     int            `gorm:"default:0"                                json:"featured_rank"`
	ActiveVersionID  *int64         `gorm:"column:active_version_id"                 json:"active_version_id,omitempty"`
	// ListingType/SourceURL: PRD §13 reference listings (e.g. video skills
	// linked out to their upstream GitHub repo instead of being packaged).
	// hosted (default) leaves SourceURL nil; reference requires it non-empty
	// (skills_reference_source_check) and free-only (skills_reference_free_check).
	ListingType string    `gorm:"type:varchar(10);not null;default:hosted" json:"listing_type"`
	SourceURL   *string   `gorm:"type:varchar(500);column:source_url"      json:"source_url,omitempty"`
	CreatedBy   int       `gorm:"not null"                                 json:"created_by"`
	CreatedAt   time.Time `gorm:"not null;default:now()"                   json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"                   json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }
