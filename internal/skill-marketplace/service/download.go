package service

import (
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DownloadService implements the six-step download flow of PRD §6.2:
// auth (done by middleware) → published check → entitlement (paid skills
// charge the user's DR quota once, inside one transaction) → read the
// packaged ZIP → upsert user_enabled_skills → stream the ZIP back.
type DownloadService struct {
	db *gorm.DB
	// quotaPerUnit returns how many quota units one USD converts to.
	// Injected so tests don't depend on the runtime option; production wires
	// it to common.QuotaPerUnit (runtime-configurable, default 500,000).
	quotaPerUnit func() float64
}

func NewDownloadService(db *gorm.DB, quotaPerUnit func() float64) *DownloadService {
	return &DownloadService{db: db, quotaPerUnit: quotaPerUnit}
}

var (
	// ErrInsufficientQuota maps to HTTP 402 (PRD §6.2 step 3).
	ErrInsufficientQuota = errors.New("insufficient balance, please top up in Wallet")
	// ErrPackageMissing maps to HTTP 500 with the exact message PRD §6.2
	// step 4 specifies.
	ErrPackageMissing = errors.New("Skill package not available, please contact support")
)

// InsufficientQuotaError carries the price the 402 response attaches as
// `data` so the frontend can show what topping up needs to cover. It matches
// errors.Is(err, ErrInsufficientQuota).
type InsufficientQuotaError struct {
	PriceUSD    float64
	QuotaNeeded int64
}

func (e *InsufficientQuotaError) Error() string { return ErrInsufficientQuota.Error() }

func (e *InsufficientQuotaError) Is(target error) bool { return target == ErrInsufficientQuota }

type DownloadResult struct {
	Zip      []byte
	Filename string
	// PurchaseMade tells the caller a paid purchase deducted quota in this
	// request, so the controller can refresh the user's quota cache — the
	// service deliberately doesn't touch the gateway's Redis layer itself.
	PurchaseMade  bool
	QuotaDeducted int64
}

// Download runs the full flow for one user + slug. The paid branch deducts
// quota and inserts the purchase row in a single transaction: the conditional
// UPDATE (quota >= needed) closes the check-then-act race on the balance, and
// the UNIQUE(user_id, skill_id) constraint on skill_purchases turns a
// concurrent double-buy into a rollback that we treat as "already licensed".
func (s *DownloadService) Download(userID int64, slug string) (*DownloadResult, error) {
	// Step 2: the skill must exist and be published.
	var skill model.Skill
	err := s.db.Where("slug = ? AND status = ?", slug, model.SkillStatusPublished).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotAvailable
		}
		return nil, err
	}
	if skill.ActiveVersionID == nil {
		return nil, ErrPackageMissing
	}

	// Step 4 hoisted before step 3: verify the package is actually deliverable
	// BEFORE charging. PRD lists the read after the deduction, but charging a
	// user and then failing with 500 on a NULL package is strictly worse, and
	// no acceptance item depends on the order.
	var version model.SkillVersion
	if err := s.db.First(&version, *skill.ActiveVersionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageMissing
		}
		return nil, err
	}
	if len(version.PackageZip) == 0 {
		return nil, ErrPackageMissing
	}

	// Step 3: entitlement check. Free skills never create purchase rows.
	result := &DownloadResult{Zip: version.PackageZip, Filename: skill.Slug + ".zip"}
	if skill.MonetizationType == "paid" {
		var owned int64
		if err := s.db.Model(&model.SkillPurchase{}).
			Where("user_id = ? AND skill_id = ?", userID, skill.ID).Count(&owned).Error; err != nil {
			return nil, err
		}
		if owned == 0 {
			quotaNeeded := int64(math.Round(skill.PriceUSD * s.quotaPerUnit()))
			txErr := s.db.Transaction(func(tx *gorm.DB) error {
				res := tx.Exec(
					"UPDATE users SET quota = quota - ? WHERE id = ? AND quota >= ?",
					quotaNeeded, userID, quotaNeeded,
				)
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					return &InsufficientQuotaError{PriceUSD: skill.PriceUSD, QuotaNeeded: quotaNeeded}
				}
				return tx.Create(&model.SkillPurchase{
					UserID:        userID,
					SkillID:       skill.ID,
					PriceUSD:      skill.PriceUSD,
					QuotaDeducted: quotaNeeded,
					PurchasedAt:   time.Now(),
				}).Error
			})
			switch {
			case txErr == nil:
				result.PurchaseMade = true
				result.QuotaDeducted = quotaNeeded
			case isUniqueViolation(txErr):
				// A concurrent request bought it first; our transaction rolled
				// back (quota restored), and the license already exists.
			default:
				return nil, txErr
			}
		}
	}

	// Step 5: record/refresh the download (latest download wins).
	enabled := model.UserEnabledSkill{
		UserID:    userID,
		SkillID:   skill.ID,
		VersionID: version.ID,
		EnabledAt: time.Now(),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "skill_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"version_id", "enabled_at"}),
	}).Create(&enabled).Error; err != nil {
		return nil, err
	}

	return result, nil
}
