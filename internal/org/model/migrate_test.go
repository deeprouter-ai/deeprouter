package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// prdSchema is PRD §7.2 written out by hand, so a renamed or dropped column
// fails here instead of surfacing in whichever later card first needs it.
var prdSchema = map[string][]string{
	"organizations":       {"id", "name", "owner_user_id", "created_time"},
	"departments":         {"id", "org_id", "name", "is_default", "parent_id", "preset_key", "deleted_at"},
	"org_roles":           {"id", "org_id", "name", "scope", "permissions", "is_preset", "deleted_at"},
	"department_managers": {"department_id", "user_id"},
	"org_invites":         {"id", "org_id", "code", "role_id", "department_id", "expires_time"},
	"org_alerts":          {"id", "org_id", "rule", "token_id", "user_id", "detail", "created_time", "acked_by"},
	"org_audit_logs":      {"id", "org_id", "actor_user_id", "action", "target_type", "target_id", "detail", "ip", "created_time"},
}

// prdPresetRoles is the PRD §2 role table written out by hand — deliberately
// not derived from PresetRoles, so a typo there cannot agree with itself.
var prdPresetRoles = map[string]struct {
	scope       string
	permissions string
}{
	"owner":    {"org", "key.read,key.create,key.update,key.assign,key.rotate,key.freeze,key.delete,member.read,member.invite,member.remove,usage.read,alert.read,audit.read"},
	"admin":    {"org", "key.read,key.create,key.update,key.assign,key.rotate,key.freeze,key.delete,member.read,member.invite,member.remove,usage.read,alert.read,audit.read"},
	"manager":  {"dept", "key.read,key.assign,member.read,member.invite,usage.read,alert.read"},
	"staff":    {"self", ""},
	"readonly": {"org", "key.read,member.read,usage.read,alert.read,audit.read"},
}

// presetRows loads the platform preset rows keyed by name.
func presetRows(t *testing.T, db *gorm.DB) map[string]OrgRole {
	t.Helper()
	var rows []OrgRole
	require.NoError(t, db.Where("org_id = ?", 0).Find(&rows).Error)
	byName := make(map[string]OrgRole, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	require.Len(t, byName, len(rows), "preset role names must be unique")
	return byName
}

func TestMigrate_CreatesThePRDTables(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, migrate(db))
		for table, columns := range prdSchema {
			require.True(t, db.Migrator().HasTable(table), "table %s", table)
			for _, column := range columns {
				require.True(t, db.Migrator().HasColumn(table, column), "column %s.%s", table, column)
			}
		}
	})
}

func TestMigrate_SeedsTheFivePresetRoles(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, migrate(db))
		rows := presetRows(t, db)
		require.Len(t, rows, len(prdPresetRoles))
		for name, want := range prdPresetRoles {
			row, ok := rows[name]
			require.True(t, ok, "preset role %s", name)
			require.Equal(t, want.scope, row.Scope, "scope of %s", name)
			require.Equal(t, want.permissions, row.Permissions, "permissions of %s", name)
			require.True(t, row.IsPreset, "%s must be flagged as a preset", name)
		}
	})
}

// Every boot runs the migration again, so a second run must change nothing —
// above all not the preset ids, which users.role_id points at.
func TestMigrate_IsIdempotent(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, migrate(db))
		first := presetRows(t, db)
		require.NoError(t, migrate(db))
		require.Equal(t, first, presetRows(t, db))
	})
}

// The code is the single definition of the presets: a row that drifted (a
// hand edit, or a definition changed between releases) is put back in place
// without changing its id.
func TestSyncPresetRoles_RepairsADriftedPresetInPlace(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, migrate(db))
		before := presetRows(t, db)
		require.NoError(t, db.Model(&OrgRole{}).
			Where("org_id = ? AND name = ?", 0, RoleReadonly).
			Updates(map[string]any{"scope": ScopeDept, "permissions": "key.delete", "is_preset": false}).Error)
		// Staff's permissions are the empty string, the value a struct update
		// would silently skip.
		require.NoError(t, db.Model(&OrgRole{}).
			Where("org_id = ? AND name = ?", 0, RoleStaff).
			Update("permissions", "key.create").Error)

		require.NoError(t, migrate(db))
		require.Equal(t, before, presetRows(t, db))
	})
}

// An organization may name a custom role "owner". It must be neither
// overwritten by the sync nor returned as the platform preset — the bug a
// struct condition would cause, because GORM drops the zero-valued org_id.
func TestSyncPresetRoles_LeavesCustomRolesAlone(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, db.AutoMigrate(&OrgRole{}))
		custom := OrgRole{OrgId: 7, Name: RoleOwner, Scope: ScopeDept, Permissions: "usage.read"}
		require.NoError(t, db.Create(&custom).Error)

		require.NoError(t, migrate(db))

		var reloaded OrgRole
		require.NoError(t, db.First(&reloaded, custom.Id).Error)
		require.Equal(t, custom, reloaded)
		presetID, err := PresetRoleID(db, RoleOwner)
		require.NoError(t, err)
		require.NotEqual(t, custom.Id, presetID)
		require.Len(t, presetRows(t, db), len(prdPresetRoles))
	})
}

func TestPresetRoleID_UnknownNameIsAnError(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, migrate(db))
		_, err := PresetRoleID(db, "superuser")
		require.Error(t, err)
	})
}

// An optional feature must not be able to stop the gateway from booting —
// neither by returning an error from the migration...
func TestMigrate_ReportsAnErrorInsteadOfFailingTheBoot(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, db.Callback().Create().Before("gorm:create").
			Register("orgtest:fail", func(tx *gorm.DB) {
				_ = tx.AddError(errors.New("injected seed failure"))
			}))
		require.ErrorContains(t, migrate(db), "injected seed failure")
		require.NoError(t, Migrate(db))
	})
}

// ...nor by panicking, which is what the SQLite migrator does on a closed
// connection.
func TestMigrate_SurvivesAPanicInsteadOfFailingTheBoot(t *testing.T) {
	db := orgtest.OpenSQLite(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.Panics(t, func() { _ = migrate(db) })
	require.NotPanics(t, func() { require.NoError(t, Migrate(db)) })
}

func TestPrimitives_AreThirteenUniqueNames(t *testing.T) {
	require.Len(t, Primitives, 13)
	seen := map[string]bool{}
	for _, primitive := range Primitives {
		require.False(t, seen[primitive], "duplicate primitive %s", primitive)
		seen[primitive] = true
	}
}

func TestPresetRoles_GrantOnlyKnownPrimitives(t *testing.T) {
	known := map[string]bool{}
	for _, primitive := range Primitives {
		known[primitive] = true
	}
	for _, preset := range PresetRoles {
		for _, permission := range preset.Permissions {
			require.True(t, known[permission], "%s grants unknown primitive %q", preset.Name, permission)
		}
	}
}

func TestPermissions_RoundTripThroughStorage(t *testing.T) {
	for _, preset := range PresetRoles {
		stored := JoinPermissions(preset.Permissions)
		require.ElementsMatch(t, preset.Permissions, SplitPermissions(stored), preset.Name)
	}
	require.Empty(t, SplitPermissions(""))
	require.Equal(t, []string{"key.read", "usage.read"}, SplitPermissions(" key.read , ,usage.read"))
}
