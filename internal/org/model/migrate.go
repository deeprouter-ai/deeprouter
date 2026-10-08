package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Migrate creates the Enterprise Org tables and syncs the preset roles. Called
// once from model/main.go during startup.
//
// 🔴 Never returns an error, by design. These tables serve an optional feature,
// and a failure here must not stop the gateway from booting — the same call
// skill-marketplace made after its migration put the whole gateway into a
// crash loop in 2026-09 (see its migrateOrReport). Without these tables only
// organization sign-up breaks; personal accounts never read them.
//
// The org columns on users / tokens / logs are a different matter: they are
// fields of the platform structs, so they migrate with the core AutoMigrate in
// model/main.go and a failure there is fatal, as it should be.
func Migrate(db *gorm.DB) error {
	// A panic would stop the boot just as surely as an error: the SQLite
	// migrator, for one, panics instead of returning when its connection is
	// gone.
	defer func() {
		if r := recover(); r != nil {
			reportMigrationFailure(fmt.Sprintf("panic: %v", r))
		}
	}()
	if err := migrate(db); err != nil {
		reportMigrationFailure(err.Error())
	}
	return nil
}

// reportMigrationFailure logs a failed org migration loudly, because the log
// line is the only trace it leaves on a gateway that otherwise starts fine.
func reportMigrationFailure(cause string) {
	common.SysError("ENTERPRISE ORG MIGRATION FAILED — the gateway is running " +
		"but the organization tables are missing or wrong, so signing up with " +
		"an organization and every /api/org endpoint will error. Personal " +
		"accounts are unaffected. This is not fatal by design " +
		"(internal/org/model/migrate.go). Cause: " + cause)
}

// migrate is Migrate without the fail-soft wrapper, so tests can see the error.
func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Organization{},
		&Department{},
		&OrgRole{},
		&DepartmentManager{},
		&OrgInvite{},
		&OrgAlert{},
		&OrgAuditLog{},
	); err != nil {
		return fmt.Errorf("AutoMigrate org tables: %w", err)
	}
	return syncPresetRoles(db)
}

// syncPresetRoles makes the org_id = 0 rows of org_roles match PresetRoles:
// missing presets are created and drifted ones are corrected in place, so a
// preset keeps its id (members point at it through users.role_id).
func syncPresetRoles(db *gorm.DB) error {
	for _, preset := range PresetRoles {
		permissions := JoinPermissions(preset.Permissions)
		// Find, not First: a missing preset is the normal case on the first
		// boot, and First would print a red "record not found" for each one.
		var found []OrgRole
		if err := db.Where("org_id = ? AND name = ?", 0, preset.Name).
			Order("id").Limit(1).Find(&found).Error; err != nil {
			return fmt.Errorf("load preset role %s: %w", preset.Name, err)
		}
		if len(found) == 0 {
			role := OrgRole{Name: preset.Name, Scope: preset.Scope, Permissions: permissions, IsPreset: true}
			if err := db.Create(&role).Error; err != nil {
				return fmt.Errorf("seed preset role %s: %w", preset.Name, err)
			}
			continue
		}
		role := found[0]
		if role.Scope == preset.Scope && role.Permissions == permissions && role.IsPreset {
			continue
		}
		// A map, not a struct: Updates(struct) skips zero values, and staff's
		// permissions are the empty string.
		if err := db.Model(&role).Updates(map[string]any{
			"scope":       preset.Scope,
			"permissions": permissions,
			"is_preset":   true,
		}).Error; err != nil {
			return fmt.Errorf("sync preset role %s: %w", preset.Name, err)
		}
	}
	return nil
}

// PresetRoleID returns the id of the platform preset role with the given name.
func PresetRoleID(db *gorm.DB, name string) (int, error) {
	var role OrgRole
	if err := db.Select("id").Where("org_id = ? AND name = ?", 0, name).First(&role).Error; err != nil {
		return 0, fmt.Errorf("preset role %s: %w", name, err)
	}
	return role.Id, nil
}
