package service

import (
	"errors"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"gorm.io/gorm"
)

// PublicSkillService serves the anonymous marketplace surface: the published
// skill listing and the per-slug detail page (PRD §6.1).
type PublicSkillService struct {
	db *gorm.DB
}

func NewPublicSkillService(db *gorm.DB) *PublicSkillService {
	return &PublicSkillService{db: db}
}

// ErrSkillNotAvailable covers both "no such slug" and "exists but draft" —
// the two must be indistinguishable to the public API (PRD §6.1: draft → 404).
var ErrSkillNotAvailable = errors.New("skill not found")

// --- request / response types ---

type PublicListRequest struct {
	Category string `form:"category"`
	Q        string `form:"q"`
	Page     int    `form:"page,default=1"`
	Limit    int    `form:"limit,default=20"`
}

type PublicSkillSummary struct {
	model.Skill
	// Version is the active version's semver string (empty when the skill has
	// no active version, which a published skill normally always has).
	Version string `json:"version"`
}

type PublicListResponse struct {
	Skills []PublicSkillSummary `json:"skills"`
	Total  int64                `json:"total"`
	Page   int                  `json:"page"`
	Limit  int                  `json:"limit"`
}

type PublicSkillDetail struct {
	model.Skill
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
}

// --- service methods ---

// ListPublishedSkills returns published skills only, featured first by
// featured_rank ASC, then the rest by created_at DESC (PRD §6.1).
func (s *PublicSkillService) ListPublishedSkills(req PublicListRequest) (*PublicListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	type row struct {
		model.Skill
		Version string
	}

	base := s.db.Table("skills sk").
		Select("sk.*, sv.version AS version").
		Joins("LEFT JOIN skill_versions sv ON sv.id = sk.active_version_id").
		Where("sk.status = ?", model.SkillStatusPublished)

	if req.Category != "" {
		base = base.Where("sk.category = ?", req.Category)
	}
	if req.Q != "" {
		// PRD asks for case-insensitive matching (it names ILIKE); LOWER/LIKE
		// gives the same semantics and also runs on the SQLite test DB.
		pattern := "%" + req.Q + "%"
		base = base.Where("LOWER(sk.name) LIKE LOWER(?) OR LOWER(sk.description) LIKE LOWER(?)", pattern, pattern)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	// Non-featured skills may carry a stale featured_rank from before the flag
	// was toggled off, so the rank only participates while the flag is set.
	var rows []row
	offset := (req.Page - 1) * req.Limit
	if err := base.
		Order("sk.featured_flag DESC, CASE WHEN sk.featured_flag THEN sk.featured_rank ELSE 0 END ASC, sk.created_at DESC").
		Offset(offset).Limit(req.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	skills := make([]PublicSkillSummary, len(rows))
	for i, r := range rows {
		skills[i] = PublicSkillSummary{Skill: r.Skill, Version: r.Version}
	}
	return &PublicListResponse{Skills: skills, Total: total, Page: req.Page, Limit: req.Limit}, nil
}

// GetSkillBySlug returns the detail for a published or deprecated skill
// (the response carries status so the frontend can render the 已下架 banner);
// draft and unknown slugs are both ErrSkillNotAvailable (PRD §6.1).
func (s *PublicSkillService) GetSkillBySlug(slug string) (*PublicSkillDetail, error) {
	var skill model.Skill
	err := s.db.Where("slug = ? AND status IN ?", slug,
		[]string{model.SkillStatusPublished, model.SkillStatusDeprecated}).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotAvailable
		}
		return nil, err
	}

	detail := &PublicSkillDetail{Skill: skill}
	if skill.ActiveVersionID != nil {
		var version model.SkillVersion
		if err := s.db.Select("version", "changelog").
			First(&version, *skill.ActiveVersionID).Error; err == nil {
			detail.Version = version.Version
			detail.Changelog = version.Changelog
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return detail, nil
}
