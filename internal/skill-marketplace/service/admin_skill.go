package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// slugPattern mirrors web/default/src/features/skills-admin/constants.ts's
// SKILL_SLUG_PATTERN — the frontend zod schema was the only place enforcing
// this; a direct API call could create/rename a skill to a slug containing
// spaces, uppercase letters or other characters P3's public URLs won't
// tolerate later.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type AdminSkillService struct {
	db *gorm.DB
}

func NewAdminSkillService(db *gorm.DB) *AdminSkillService {
	return &AdminSkillService{db: db}
}

// --- request / response types ---

type ListSkillsRequest struct {
	Status   string `form:"status"`
	Category string `form:"category"`
	Q        string `form:"q"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
}

type SkillSummary struct {
	model.Skill
	ActiveVersion string `json:"active_version,omitempty"`
}

type ListSkillsResponse struct {
	Skills []SkillSummary `json:"skills"`
	Total  int64          `json:"total"`
}

type CreateSkillRequest struct {
	Slug             string   `json:"slug"             binding:"required"`
	Name             string   `json:"name"             binding:"required"`
	Description      string   `json:"description"      binding:"required"`
	Category         string   `json:"category"         binding:"required"`
	Tags             []string `json:"tags"`
	MonetizationType string   `json:"monetization_type"`
	PriceUSD         float64  `json:"price_usd"`
}

type UpdateSkillRequest struct {
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Tags             []string `json:"tags"`
	MonetizationType string   `json:"monetization_type"`
	PriceUSD         *float64 `json:"price_usd"`
}

type FeaturedRequest struct {
	FeaturedFlag bool `json:"featured_flag"`
	FeaturedRank int  `json:"featured_rank"`
}

// --- service methods ---

func (s *AdminSkillService) ListSkills(req ListSkillsRequest) (*ListSkillsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	type row struct {
		model.Skill
		ActiveVersion string
	}

	base := s.db.Table("skills sk").
		Select("sk.*, sv.version AS active_version").
		Joins("LEFT JOIN skill_versions sv ON sv.id = sk.active_version_id")

	if req.Status != "" {
		base = base.Where("sk.status = ?", req.Status)
	}
	if req.Category != "" {
		base = base.Where("sk.category = ?", req.Category)
	}
	if req.Q != "" {
		// Plain LIKE (not ILIKE) — cross-DB compatible with the rest of the
		// codebase's search implementations (e.g. model.SearchUsers).
		like := "%" + req.Q + "%"
		base = base.Where("sk.name LIKE ? OR sk.slug LIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []row
	offset := (req.Page - 1) * req.PageSize
	if err := base.Order("sk.created_at DESC").Offset(offset).Limit(req.PageSize).Scan(&rows).Error; err != nil {
		return nil, err
	}

	summaries := make([]SkillSummary, len(rows))
	for i, r := range rows {
		summaries[i] = SkillSummary{Skill: r.Skill, ActiveVersion: r.ActiveVersion}
	}
	return &ListSkillsResponse{Skills: summaries, Total: total}, nil
}

// GetSkill returns one skill by id, joined with its active version's semver
// string — same shape as the rows ListSkills returns, so the admin edit page
// can render metadata + "currently active: X.Y.Z" from a single call.
func (s *AdminSkillService) GetSkill(id int64) (*SkillSummary, error) {
	type row struct {
		model.Skill
		ActiveVersion string
	}

	var r row
	err := s.db.Table("skills sk").
		Select("sk.*, sv.version AS active_version").
		Joins("LEFT JOIN skill_versions sv ON sv.id = sk.active_version_id").
		Where("sk.id = ?", id).
		Scan(&r).Error
	if err != nil {
		return nil, err
	}
	if r.Skill.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &SkillSummary{Skill: r.Skill, ActiveVersion: r.ActiveVersion}, nil
}

func (s *AdminSkillService) CreateSkill(req CreateSkillRequest, adminID int) (*model.Skill, error) {
	if !slugPattern.MatchString(req.Slug) {
		return nil, ErrInvalidSlugFormat
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	monetization := req.MonetizationType
	if monetization == "" {
		monetization = "free"
	}
	// The `skills_monetization_check`/`skills_price_check` DB constraints
	// already stop bad data from being written, but a violation surfaces as
	// a raw Postgres error wrapped in a 500 — validate here so a bypass of
	// the frontend's zod schema gets the same clean 400 a normal client sees.
	if monetization != "free" && monetization != "paid" {
		return nil, ErrInvalidMonetizationType
	}
	if monetization == "paid" && req.PriceUSD <= 0 {
		return nil, ErrPriceRequiredForPaid
	}

	skill := &model.Skill{
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      req.Description,
		Category:         req.Category,
		Tags:             pq.StringArray(tags),
		Status:           model.SkillStatusDraft,
		MonetizationType: monetization,
		PriceUSD:         req.PriceUSD,
		CreatedBy:        adminID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.db.Create(skill).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionCreate, map[string]any{
		"slug": skill.Slug,
		"name": skill.Name,
	})
	return skill, nil
}

func (s *AdminSkillService) UpdateSkill(id int64, req UpdateSkillRequest) (*model.Skill, error) {
	var skill model.Skill
	if err := s.db.First(&skill, id).Error; err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now()}
	if req.Slug != "" && req.Slug != skill.Slug {
		// PRD §5.1 / AC-9: slug is editable while draft, locked (409) once
		// published — a published skill's slug is baked into its download
		// URL and the ZIP's manifest, so changing it later would strand
		// those references.
		if skill.Status != model.SkillStatusDraft {
			return nil, ErrSlugLocked
		}
		if !slugPattern.MatchString(req.Slug) {
			return nil, ErrInvalidSlugFormat
		}
		updates["slug"] = req.Slug
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Tags != nil {
		updates["tags"] = pq.StringArray(req.Tags)
	}
	// Validate against the *effective* post-update state, not just whatever
	// field this particular request happens to touch — a partial update that
	// only sends monetization_type:"paid" must still be checked against the
	// skill's existing price_usd (and vice versa), the same way the
	// frontend's zod .refine() checks the form's combined final values.
	effectiveMonetization := skill.MonetizationType
	if req.MonetizationType != "" {
		if req.MonetizationType != "free" && req.MonetizationType != "paid" {
			return nil, ErrInvalidMonetizationType
		}
		updates["monetization_type"] = req.MonetizationType
		effectiveMonetization = req.MonetizationType
	}
	effectivePrice := skill.PriceUSD
	if req.PriceUSD != nil {
		updates["price_usd"] = *req.PriceUSD
		effectivePrice = *req.PriceUSD
	}
	if effectiveMonetization == "paid" && effectivePrice <= 0 {
		return nil, ErrPriceRequiredForPaid
	}

	if err := s.db.Model(&skill).Updates(updates).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}
	// Re-fetch to get the updated record
	if err := s.db.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// PublishSkill transitions draft→published or deprecated→published (republish).
// Returns ErrNoActiveVersion if active_version_id is NULL (P2 will populate it).
var ErrNoActiveVersion = errors.New("cannot publish: skill has no active version")
var ErrInvalidTransition = errors.New("invalid state transition")
var ErrSlugTaken = errors.New("slug already taken")
var ErrSlugLocked = errors.New("slug cannot be changed once the skill has been published")
var ErrInvalidSlugFormat = errors.New("slug must be lowercase letters, numbers and hyphens only")
var ErrSkillNotPublished = errors.New("only published skills can be featured")
var ErrInvalidMonetizationType = errors.New("monetization_type must be 'free' or 'paid'")
var ErrPriceRequiredForPaid = errors.New("price_usd must be greater than 0 for a paid skill")

// isUniqueViolation detects unique constraint errors from both PostgreSQL
// ("duplicate key value violates unique constraint") and SQLite
// ("UNIQUE constraint failed"), so unit tests using an in-memory SQLite DB
// exercise the same code path as production.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "duplicate key value violates unique constraint") ||
		strings.Contains(s, "UNIQUE constraint failed")
}

func (s *AdminSkillService) PublishSkill(id int64, adminID int) (*model.Skill, error) {
	var skill model.Skill
	if err := s.db.First(&skill, id).Error; err != nil {
		return nil, err
	}

	if skill.Status == model.SkillStatusPublished {
		return &skill, nil
	}
	if skill.Status != model.SkillStatusDraft && skill.Status != model.SkillStatusDeprecated {
		return nil, fmt.Errorf("%w: %s → published", ErrInvalidTransition, skill.Status)
	}

	// PRD §7.1: draft/deprecated -> published requires active_version_id
	// to be set. This check was deferred at P1 ("P2 will enforce") and P2
	// never came back to add it — ErrNoActiveVersion existed but was never
	// returned anywhere, so the API would happily publish a versionless
	// skill. Found auditing controller-layer test coverage.
	if skill.ActiveVersionID == nil {
		return nil, ErrNoActiveVersion
	}

	fromStatus := skill.Status
	action := model.LogActionPublish
	if fromStatus == model.SkillStatusDeprecated {
		action = model.LogActionRepublish
	}

	if err := s.db.Model(&skill).Updates(map[string]any{
		"status":     model.SkillStatusPublished,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, action, map[string]any{
		"from_status":       fromStatus,
		"to_status":         model.SkillStatusPublished,
		"active_version_id": skill.ActiveVersionID,
	})

	skill.Status = model.SkillStatusPublished
	return &skill, nil
}

// DeprecateSkill transitions published→deprecated.
func (s *AdminSkillService) DeprecateSkill(id int64, adminID int) (*model.Skill, error) {
	var skill model.Skill
	if err := s.db.First(&skill, id).Error; err != nil {
		return nil, err
	}

	if skill.Status != model.SkillStatusPublished {
		return nil, fmt.Errorf("%w: %s → deprecated (only published skills can be deprecated)", ErrInvalidTransition, skill.Status)
	}

	// Deprecated skills are no longer eligible to be featured (UpdateFeatured
	// now rejects non-published skills too) — reset here so a stale
	// featured_flag=true doesn't silently resurface if the skill is later
	// republished without the Admin re-checking it.
	wasFeatured := skill.FeaturedFlag
	if err := s.db.Model(&skill).Updates(map[string]any{
		"status":        model.SkillStatusDeprecated,
		"featured_flag": false,
		"featured_rank": 0,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionDeprecate, map[string]any{
		"from_status":         model.SkillStatusPublished,
		"to_status":           model.SkillStatusDeprecated,
		"featured_flag_reset": wasFeatured,
	})

	skill.Status = model.SkillStatusDeprecated
	skill.FeaturedFlag = false
	skill.FeaturedRank = 0
	return &skill, nil
}

// DeleteSkill hard-deletes a draft skill (published/deprecated are protected).
func (s *AdminSkillService) DeleteSkill(id int64, adminID int) error {
	var skill model.Skill
	if err := s.db.First(&skill, id).Error; err != nil {
		return err
	}

	if skill.Status != model.SkillStatusDraft {
		return fmt.Errorf("%w: only draft skills can be deleted (status=%s)", ErrInvalidTransition, skill.Status)
	}

	// Log before delete so we keep the skillID reference
	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionDelete, map[string]any{
		"slug": skill.Slug,
		"name": skill.Name,
	})

	return s.db.Delete(&skill).Error
}

// UpdateFeatured sets featured_flag and featured_rank.
func (s *AdminSkillService) UpdateFeatured(id int64, req FeaturedRequest, adminID int) (*model.Skill, error) {
	var skill model.Skill
	if err := s.db.First(&skill, id).Error; err != nil {
		return nil, err
	}

	// AC-8: "Admin can toggle featured on a *published* skill" — the frontend
	// disables the control for draft/deprecated rows, but nothing stopped a
	// direct API call from featuring a skill nobody can even see yet.
	if skill.Status != model.SkillStatusPublished {
		return nil, ErrSkillNotPublished
	}

	if err := s.db.Model(&skill).Updates(map[string]any{
		"featured_flag": req.FeaturedFlag,
		"featured_rank": req.FeaturedRank,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionFeaturedUpdate, map[string]any{
		"featured_flag": req.FeaturedFlag,
		"featured_rank": req.FeaturedRank,
	})

	skill.FeaturedFlag = req.FeaturedFlag
	skill.FeaturedRank = req.FeaturedRank
	return &skill, nil
}

// GetLogs returns the last 20 admin log entries for a skill.
func (s *AdminSkillService) GetLogs(skillID int64) ([]model.SkillAdminLog, error) {
	var logs []model.SkillAdminLog
	if err := s.db.Where("skill_id = ?", skillID).
		Order("created_at DESC, id DESC").Limit(20).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
