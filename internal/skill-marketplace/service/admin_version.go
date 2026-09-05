package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AdminVersionService struct {
	db *gorm.DB
}

func NewAdminVersionService(db *gorm.DB) *AdminVersionService {
	return &AdminVersionService{db: db}
}

var (
	ErrVersionNotDraft         = errors.New("version is not in draft status")
	ErrManifestSlugMismatch    = errors.New("manifest_json.slug does not match skill.slug")
	ErrManifestVersionMismatch = errors.New("manifest_json.version does not match the version being saved")
	ErrInvalidVersionFormat    = errors.New("version must match semver format X.Y.Z")
	ErrManifestInvalid         = errors.New("manifest_json is missing a required field")
	ErrVersionAlreadyExists    = errors.New("version already exists for this skill")
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// validateManifestUploadFields is PRD §9 "stage 1" — checked at upload time,
// on the manifest metadata only (package *content* security scanning is a
// separate, later check that runs at activation, see packaging.go).
func validateManifestUploadFields(manifest map[string]interface{}) error {
	if slug, _ := manifest["slug"].(string); slug == "" {
		return fmt.Errorf("%w: slug", ErrManifestInvalid)
	}
	if version, _ := manifest["version"].(string); version == "" {
		return fmt.Errorf("%w: version", ErrManifestInvalid)
	}
	if manifest["requires_deeprouter_key"] != true {
		return fmt.Errorf("%w: requires_deeprouter_key must be true", ErrManifestInvalid)
	}
	if endpoint, _ := manifest["deeprouter_routing_endpoint"].(string); endpoint == "" {
		return fmt.Errorf("%w: deeprouter_routing_endpoint", ErrManifestInvalid)
	}
	return nil
}

// validateManifestContent runs the full §9 stage-1 check plus the two
// consistency checks (slug, version) against the skill/version this
// manifest_json is being saved onto. Shared by UploadVersion and
// UpdateVersion — content correctness must hold whenever manifest_json
// changes, not just the first time it's set, because BuildSkillPackage
// (packaging.go) never re-runs these field checks at activation time; it
// only runs the two *security* guards. Skipping this on edit would let an
// Admin silently corrupt a previously-valid manifest and have it ship in
// the ZIP unnoticed until a user's runner rejects it at runtime.
func validateManifestContent(manifest map[string]interface{}, skillSlug, expectedVersion string) error {
	if err := validateManifestUploadFields(manifest); err != nil {
		return err
	}
	if slug, _ := manifest["slug"].(string); slug != skillSlug {
		return ErrManifestSlugMismatch
	}
	if v, _ := manifest["version"].(string); v != expectedVersion {
		return ErrManifestVersionMismatch
	}
	return nil
}

// --- request types ---

type UploadVersionRequest struct {
	Version        string          `json:"version"`
	SkillMDContent string          `json:"skill_md_content"`
	ManifestJSON   json.RawMessage `json:"manifest_json"`
	Changelog      string          `json:"changelog"`
}

type UpdateVersionRequest struct {
	SkillMDContent *string          `json:"skill_md_content"`
	ManifestJSON   *json.RawMessage `json:"manifest_json"`
	Changelog      *string          `json:"changelog"`
}

// ListVersions returns every version of a skill, newest first. package_zip is
// omitted at the query level — PRD §9 requires it never appear in an API
// response body, and it can run tens of KB per row.
func (s *AdminVersionService) ListVersions(skillID int64) ([]model.SkillVersion, error) {
	var versions []model.SkillVersion
	err := s.db.Omit("package_zip").
		Where("skill_id = ?", skillID).
		Order("created_at DESC, id DESC").
		Find(&versions).Error
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// UploadVersion creates a new draft version. PRD §6.3 / §9 stage 1.
func (s *AdminVersionService) UploadVersion(
	skillID int64, req UploadVersionRequest, adminID int,
) (*model.SkillVersion, error) {
	var skill model.Skill
	if err := s.db.First(&skill, skillID).Error; err != nil {
		return nil, err
	}

	if !semverPattern.MatchString(req.Version) {
		return nil, ErrInvalidVersionFormat
	}

	var manifest map[string]interface{}
	if err := common.Unmarshal(req.ManifestJSON, &manifest); err != nil {
		return nil, fmt.Errorf("%w: manifest_json is not valid JSON", ErrManifestInvalid)
	}
	if err := validateManifestContent(manifest, skill.Slug, req.Version); err != nil {
		return nil, err
	}

	version := model.SkillVersion{
		SkillID:        skillID,
		Version:        req.Version,
		Status:         model.SkillVersionStatusDraft,
		SkillMDContent: req.SkillMDContent,
		ManifestJSON:   req.ManifestJSON,
		Changelog:      req.Changelog,
		CreatedBy:      adminID,
	}
	if err := s.db.Create(&version).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrVersionAlreadyExists
		}
		return nil, err
	}

	_ = model.WriteLog(s.db, int64(adminID), &skillID, model.LogActionVersionUpload, map[string]any{
		"version_id": version.ID,
		"version":    version.Version,
	})

	return &version, nil
}

// UpdateVersion partially updates a draft version. version cannot be
// changed (a version bump must go through UploadVersion). Not an audited
// action — PRD §5.6's action enum does not include an "edit" entry, only
// version_upload / version_activate / version_delete.
func (s *AdminVersionService) UpdateVersion(
	skillID, versionID int64, req UpdateVersionRequest, adminID int,
) (*model.SkillVersion, error) {
	var version model.SkillVersion
	if err := s.db.Where("id = ? AND skill_id = ?", versionID, skillID).First(&version).Error; err != nil {
		return nil, err
	}
	if version.Status != model.SkillVersionStatusDraft {
		return nil, ErrVersionNotDraft
	}

	updates := map[string]any{}
	if req.SkillMDContent != nil {
		updates["skill_md_content"] = *req.SkillMDContent
	}
	if req.ManifestJSON != nil {
		var manifest map[string]interface{}
		if err := common.Unmarshal(*req.ManifestJSON, &manifest); err != nil {
			return nil, fmt.Errorf("%w: manifest_json is not valid JSON", ErrManifestInvalid)
		}
		var skill model.Skill
		if err := s.db.Select("slug").First(&skill, skillID).Error; err != nil {
			return nil, err
		}
		if err := validateManifestContent(manifest, skill.Slug, version.Version); err != nil {
			return nil, err
		}
		updates["manifest_json"] = *req.ManifestJSON
	}
	if req.Changelog != nil {
		updates["changelog"] = *req.Changelog
	}
	if len(updates) > 0 {
		if err := s.db.Model(&version).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if err := s.db.First(&version, versionID).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// DeleteVersion removes a draft version. active/archived versions cannot be
// deleted (PRD §7.2 — archived ZIPs must survive for users who already
// downloaded them).
//
// Runs inside a transaction and locks the version row (matching
// ActivateVersion's locking of that same row) so a concurrent activate on
// this exact version can't read it as still-draft after it's been deleted:
// without both sides locking the same row, an unlocked read doesn't wait on
// the other transaction's lock, and the race stays open even if only one
// side takes it.
func (s *AdminVersionService) DeleteVersion(skillID, versionID int64, adminID int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var version model.SkillVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND skill_id = ?", versionID, skillID).First(&version).Error; err != nil {
			return err
		}
		if version.Status != model.SkillVersionStatusDraft {
			return ErrVersionNotDraft
		}
		if err := tx.Delete(&version).Error; err != nil {
			return err
		}
		return model.WriteLog(tx, int64(adminID), &skillID, model.LogActionVersionDelete, map[string]any{
			"version_id": versionID,
			"version":    version.Version,
		})
	})
}

// ActivateVersion runs the full 8-step activation transaction (PRD §7.2):
// lock the skill row, archive the previous active version, activate the
// target, build+guard the ZIP package, persist it, update
// skills.active_version_id, and write the audit log — all inside one
// transaction, so a guard failure or any other error rolls back every step
// and leaves nothing half-done.
func (s *AdminVersionService) ActivateVersion(skillID, versionID int64, adminID int) (*model.SkillVersion, error) {
	var activated model.SkillVersion

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Step 2: lock the skill row (not just the previous active version)
		// so every concurrent activation for this skill — including the
		// very first one, when active_version_id is still NULL — serializes
		// on the same lock.
		var skill model.Skill
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&skill, skillID).Error; err != nil {
			return err
		}

		// Also lock the target version row itself — not just the skill row —
		// so a concurrent DeleteVersion on this exact (still-draft) version
		// can't slip in between this read and step 4's activation. Locking
		// only one side of a race doesn't close it: an unlocked plain SELECT
		// doesn't wait on someone else's row lock, it just reads the last
		// committed snapshot.
		var version model.SkillVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND skill_id = ?", versionID, skillID).First(&version).Error; err != nil {
			return err
		}

		// Copy the *value*, not the pointer: GORM's Model(&skill).Update in
		// step 6 writes the new id back through skill.ActiveVersionID in
		// place, so a plain pointer copy here would alias the same memory
		// and end up showing the *new* value too.
		var previousActiveVersionID *int64
		if skill.ActiveVersionID != nil {
			v := *skill.ActiveVersionID
			previousActiveVersionID = &v
		}

		// Step 3: archive the previous active version, if any, and if it
		// isn't the version we're (re-)activating.
		if skill.ActiveVersionID != nil && *skill.ActiveVersionID != versionID {
			if err := tx.Model(&model.SkillVersion{}).
				Where("id = ?", *skill.ActiveVersionID).
				Update("status", model.SkillVersionStatusArchived).Error; err != nil {
				return err
			}
		}

		// Step 4: activate the target version.
		if err := tx.Model(&version).Update("status", model.SkillVersionStatusActive).Error; err != nil {
			return err
		}
		version.Status = model.SkillVersionStatusActive

		// Step 5: build the package in memory — both security guards run
		// inside BuildSkillPackage (see packaging.go). A guard failure here
		// aborts the whole transaction: the version never ends up "active"
		// without a package, and nothing from steps 3-4 is kept either.
		zipBytes, sha256Hex, err := BuildSkillPackage(&skill, &version)
		if err != nil {
			return err
		}
		builtAt := time.Now()
		if err := tx.Model(&version).Updates(map[string]any{
			"package_zip":      zipBytes,
			"package_sha256":   sha256Hex,
			"package_built_at": builtAt,
		}).Error; err != nil {
			return err
		}

		// Step 6: point the skill at the newly-active version.
		if err := tx.Model(&skill).Update("active_version_id", versionID).Error; err != nil {
			return err
		}

		// Step 7: audit log, written through tx so it commits/rolls back
		// with everything else. previous_active_version_id is skill's value
		// from before step 6's update — useful for tracing rollbacks.
		if err := model.WriteLog(tx, int64(adminID), &skillID, model.LogActionVersionActivate, map[string]any{
			"version_id":                 versionID,
			"version":                    version.Version,
			"previous_active_version_id": previousActiveVersionID,
		}); err != nil {
			return err
		}

		version.PackageSHA256 = sha256Hex
		version.PackageBuiltAt = &builtAt
		activated = version
		return nil
	})
	// Step 8: gorm commits automatically when the callback returns nil, and
	// rolls back automatically on any returned error.
	if err != nil {
		return nil, err
	}
	return &activated, nil
}
