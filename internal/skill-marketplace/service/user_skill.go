package service

import (
	"time"

	"gorm.io/gorm"
)

// UserSkillService serves the two logged-in personal lists: My Skills and
// Purchase History (PRD §6.2).
type UserSkillService struct {
	db *gorm.DB
}

func NewUserSkillService(db *gorm.DB) *UserSkillService {
	return &UserSkillService{db: db}
}

// --- response types ---

type UserSkillEntry struct {
	SkillID int64  `json:"skill_id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	// Version is the semver the user downloaded (locked at download time).
	Version   string    `json:"version"`
	EnabledAt time.Time `json:"enabled_at"`
	// SkillStatus lets the frontend render the 已下架 badge when the skill
	// has been deprecated since the download.
	SkillStatus string `json:"skill_status"`
}

type UserSkillsResponse struct {
	Skills []UserSkillEntry `json:"skills"`
	Total  int64            `json:"total"`
	Page   int              `json:"page"`
	Limit  int              `json:"limit"`
}

type UserPurchaseEntry struct {
	SkillID     int64     `json:"skill_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	PriceUSD    float64   `json:"price_usd"`
	PurchasedAt time.Time `json:"purchased_at"`
}

type UserPurchasesResponse struct {
	Purchases []UserPurchaseEntry `json:"purchases"`
	Total     int64               `json:"total"`
	Page      int                 `json:"page"`
	Limit     int                 `json:"limit"`
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

// ListUserSkills returns the user's download records, newest first.
func (s *UserSkillService) ListUserSkills(userID int64, page, limit int) (*UserSkillsResponse, error) {
	page, limit = normalizePage(page, limit)

	base := s.db.Table("user_enabled_skills ues").
		Select("ues.skill_id, sk.slug, sk.name, sk.status AS skill_status, sv.version, ues.enabled_at").
		Joins("JOIN skills sk ON sk.id = ues.skill_id").
		Joins("LEFT JOIN skill_versions sv ON sv.id = ues.version_id").
		Where("ues.user_id = ?", userID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var entries []UserSkillEntry
	offset := (page - 1) * limit
	if err := base.Order("ues.enabled_at DESC, ues.id DESC").
		Offset(offset).Limit(limit).Scan(&entries).Error; err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []UserSkillEntry{}
	}
	return &UserSkillsResponse{Skills: entries, Total: total, Page: page, Limit: limit}, nil
}

// ListUserPurchases returns the user's paid purchase records, newest first.
func (s *UserSkillService) ListUserPurchases(userID int64, page, limit int) (*UserPurchasesResponse, error) {
	page, limit = normalizePage(page, limit)

	base := s.db.Table("skill_purchases sp").
		Select("sp.skill_id, sk.slug, sk.name, sp.price_usd, sp.purchased_at").
		Joins("JOIN skills sk ON sk.id = sp.skill_id").
		Where("sp.user_id = ?", userID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var entries []UserPurchaseEntry
	offset := (page - 1) * limit
	if err := base.Order("sp.purchased_at DESC, sp.id DESC").
		Offset(offset).Limit(limit).Scan(&entries).Error; err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []UserPurchaseEntry{}
	}
	return &UserPurchasesResponse{Purchases: entries, Total: total, Page: page, Limit: limit}, nil
}
