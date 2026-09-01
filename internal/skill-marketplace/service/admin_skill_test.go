package service_test

import (
	"testing"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupDB opens an in-memory SQLite and creates the minimal tables needed
// for the service. We cannot call model.Migrate() here because it emits
// PostgreSQL-specific DDL (TEXT[], JSONB, partial indexes, DO $$ blocks).
func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id      INTEGER PRIMARY KEY,
			status  INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			slug              TEXT UNIQUE NOT NULL,
			name              TEXT NOT NULL DEFAULT '',
			description       TEXT NOT NULL DEFAULT '',
			category          TEXT NOT NULL DEFAULT '',
			status            TEXT NOT NULL DEFAULT 'draft',
			monetization_type TEXT NOT NULL DEFAULT 'free',
			price_usd         REAL NOT NULL DEFAULT 0,
			featured_flag     INTEGER DEFAULT 0,
			featured_rank     INTEGER DEFAULT 0,
			active_version_id INTEGER,
			created_by        INTEGER NOT NULL,
			created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skill_versions (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_id         INTEGER NOT NULL,
			version          TEXT NOT NULL,
			status           TEXT NOT NULL DEFAULT 'draft',
			skill_md_content TEXT NOT NULL DEFAULT '',
			manifest_json    TEXT NOT NULL DEFAULT '{}',
			created_by       INTEGER NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skill_admin_logs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id   INTEGER NOT NULL,
			skill_id   INTEGER,
			action     TEXT NOT NULL,
			details    TEXT DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		require.NoError(t, db.Exec(s).Error)
	}
	// seed one admin user so foreign keys resolve
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	return db
}

// insertSkill inserts a row via raw SQL, returning its id.
// Avoids GORM struct mapping so TEXT[] / JSONB differences don't matter.
func insertSkill(t *testing.T, db *gorm.DB, slug, status string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, category, status, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, "Test Skill", "Test Description", "test", status, 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func logCount(t *testing.T, db *gorm.DB, skillID int64, action string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM skill_admin_logs WHERE skill_id = ? AND action = ?`,
		skillID, action,
	).Scan(&n).Error)
	return n
}

// ── Publish ───────────────────────────────────────────────────────────────────

func TestPublishSkill_FromDraft(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "test-slug", "draft")
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, int64(1), logCount(t, db, id, "publish"))
}

func TestPublishSkill_FromDeprecated_WritesRepublish(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-slug", "deprecated")
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, int64(0), logCount(t, db, id, "publish"))
	assert.Equal(t, int64(1), logCount(t, db, id, "republish"))
}

func TestPublishSkill_AlreadyPublished_IsIdempotent(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "pub-slug", "published")
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	// idempotent: no log written for a no-op
	assert.Equal(t, int64(0), logCount(t, db, id, "publish"))
}

func TestPublishSkill_NotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.PublishSkill(9999, 1)
	require.Error(t, err)
}

// ── Deprecate ─────────────────────────────────────────────────────────────────

func TestDeprecateSkill_FromPublished(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-pub", "published")
	skill, err := svc.DeprecateSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "deprecated", skill.Status)
	assert.Equal(t, int64(1), logCount(t, db, id, "deprecate"))
}

func TestDeprecateSkill_FromDraft_ReturnsInvalidTransition(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-draft", "draft")
	_, err := svc.DeprecateSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidTransition)
	assert.Equal(t, int64(0), logCount(t, db, id, "deprecate"))
}

func TestDeprecateSkill_AlreadyDeprecated_ReturnsInvalidTransition(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "already-dep", "deprecated")
	_, err := svc.DeprecateSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidTransition)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDeleteSkill_DraftSucceeds(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "del-draft", "draft")
	require.NoError(t, svc.DeleteSkill(id, 1))

	// Row gone from DB
	var count int64
	db.Raw(`SELECT COUNT(*) FROM skills WHERE id = ?`, id).Scan(&count)
	assert.Equal(t, int64(0), count)
	// Audit log written before delete
	assert.Equal(t, int64(1), logCount(t, db, id, "delete"))
}

func TestDeleteSkill_PublishedBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "del-pub", "published")
	err := svc.DeleteSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidTransition)

	var count int64
	db.Raw(`SELECT COUNT(*) FROM skills WHERE id = ?`, id).Scan(&count)
	assert.Equal(t, int64(1), count, "published skill must not be deleted")
}

func TestDeleteSkill_DeprecatedBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "del-dep", "deprecated")
	err := svc.DeleteSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidTransition)
}

// ── Featured ──────────────────────────────────────────────────────────────────

func TestUpdateFeatured_SetsFieldsAndLogs(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "feat-slug", "published")
	skill, err := svc.UpdateFeatured(id, mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 3}, 1)
	require.NoError(t, err)
	assert.True(t, skill.FeaturedFlag)
	assert.Equal(t, 3, skill.FeaturedRank)
	assert.Equal(t, int64(1), logCount(t, db, id, "featured_update"))
}

// ── GetLogs ───────────────────────────────────────────────────────────────────

func TestGetLogs_ReturnsLogsForSkill(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "log-slug", "draft")
	// Trigger two state transitions to produce log entries
	_, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	_, err = svc.DeprecateSkill(id, 1)
	require.NoError(t, err)

	logs, err := svc.GetLogs(id)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
	// Most recent first
	assert.Equal(t, "deprecate", logs[0].Action)
	assert.Equal(t, "publish", logs[1].Action)
}

func TestGetLogs_EmptyWhenNoLogs(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "no-log-slug", "draft")
	logs, err := svc.GetLogs(id)
	require.NoError(t, err)
	assert.Empty(t, logs)
}
