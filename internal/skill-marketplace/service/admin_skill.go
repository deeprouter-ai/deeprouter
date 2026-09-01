package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"gorm.io/gorm"
)

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

func (s *AdminSkillService) CreateSkill(req CreateSkillRequest, adminID int) (*model.Skill, error) {
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	monetization := req.MonetizationType
	if monetization == "" {
		monetization = "free"
	}

	skill := &model.Skill{
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      req.Description,
		Category:         req.Category,
		Tags:             tags,
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
		updates["tags"] = req.Tags
	}
	if req.MonetizationType != "" {
		updates["monetization_type"] = req.MonetizationType
	}
	if req.PriceUSD != nil {
		updates["price_usd"] = *req.PriceUSD
	}

	if err := s.db.Model(&skill).Updates(updates).Error; err != nil {
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

	// P1: no versions yet, so active_version_id is always NULL.
	// Allow publish without version so admin UI can be tested end-to-end.
	// P2 will enforce active_version_id != NULL before publish.

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

	if err := s.db.Model(&skill).Updates(map[string]any{
		"status":     model.SkillStatusDeprecated,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	skillID := skill.ID
	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionDeprecate, map[string]any{
		"from_status": model.SkillStatusPublished,
		"to_status":   model.SkillStatusDeprecated,
	})

	skill.Status = model.SkillStatusDeprecated
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
