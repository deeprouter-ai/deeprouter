package service_test

import (
	"testing"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/glebarez/sqlite"
	"github.com/lib/pq"
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
			tags              TEXT NOT NULL DEFAULT '{}',
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
			manifest_json    BLOB NOT NULL DEFAULT '{}',
			package_zip      BLOB,
			package_sha256   TEXT,
			package_built_at DATETIME,
			changelog        TEXT NOT NULL DEFAULT '',
			created_by       INTEGER NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (skill_id, version)
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

// ── ListSkills ────────────────────────────────────────────────────────────────
//
// Never tested before this: it's the query the list page actually runs on
// every load — status filter, category filter, pagination and the
// active_version join — and none of it had a single dedicated test. The
// controller test only checks Total==1 on an unfiltered call.

func insertSkillWithCategory(t *testing.T, db *gorm.DB, slug, status, category string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, category, status, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, "Test Skill", "d", category, status, 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func TestListSkills_FiltersByStatus(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkill(t, db, "draft-one", "draft")
	insertSkill(t, db, "published-one", "published")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Status: "published", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "published-one", resp.Skills[0].Slug)
}

func TestListSkills_FiltersByCategory(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithCategory(t, db, "writing-skill", "draft", "writing")
	insertSkillWithCategory(t, db, "code-skill", "draft", "code")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Category: "code", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "code-skill", resp.Skills[0].Slug)
}

func TestListSkills_StatusAndCategoryCombine(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithCategory(t, db, "match", "published", "code")
	insertSkillWithCategory(t, db, "wrong-status", "draft", "code")
	insertSkillWithCategory(t, db, "wrong-category", "published", "writing")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Status: "published", Category: "code", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "match", resp.Skills[0].Slug)
}

func TestListSkills_PaginationRespectsPageSize(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkill(t, db, "skill-a", "draft")
	insertSkill(t, db, "skill-b", "draft")
	insertSkill(t, db, "skill-c", "draft")

	page1, err := svc.ListSkills(mktsvc.ListSkillsRequest{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page1.Total, "Total reflects the full match count, not the page size")
	assert.Len(t, page1.Skills, 2)

	page2, err := svc.ListSkills(mktsvc.ListSkillsRequest{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page2.Total)
	assert.Len(t, page2.Skills, 1, "3 rows, page size 2 -> page 2 holds the remaining 1")
}

func TestListSkills_ZeroOrNegativePage_DefaultsToPageOne(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkill(t, db, "only-skill", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Page: 0, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, resp.Skills, 1, "page 0 must not compute a negative offset and return nothing")
}

func TestListSkills_PageSizeOutOfRange_ClampsToDefault20(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	for i := 0; i < 3; i++ {
		insertSkill(t, db, "skill-"+string(rune('a'+i)), "draft")
	}

	tooSmall, err := svc.ListSkills(mktsvc.ListSkillsRequest{PageSize: 0})
	require.NoError(t, err)
	assert.Len(t, tooSmall.Skills, 3, "page_size 0 should clamp to the 20 default, not return 0 rows")

	tooBig, err := svc.ListSkills(mktsvc.ListSkillsRequest{PageSize: 500})
	require.NoError(t, err)
	assert.Len(t, tooBig.Skills, 3, "page_size above 100 should also clamp to 20, not try to return 500")
}

func TestListSkills_IncludesActiveVersionPerRow(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	withVersion := insertSkill(t, db, "has-version", "published")
	setActiveVersion(t, db, withVersion)
	insertSkill(t, db, "no-version", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, resp.Skills, 2)

	byslug := map[string]mktsvc.SkillSummary{}
	for _, s := range resp.Skills {
		byslug[s.Slug] = s
	}
	assert.Equal(t, "1.0.0", byslug["has-version"].ActiveVersion)
	assert.Empty(t, byslug["no-version"].ActiveVersion)
}

func TestListSkills_EmptyWhenNoSkills(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Skills)
}

// ── CreateSkill ───────────────────────────────────────────────────────────────
//
// Never tested before this: db.Create() for a Skill needs a `tags` column in
// the SQLite fixture (see the historical note atop unique_violation_test.go),
// and nobody added one — so CreateSkill's happy path, its defaulting, and its
// duplicate-slug handling all went untested since P1 shipped.

func TestCreateSkill_Success(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug:             "new-skill",
		Name:             "New Skill",
		Description:      "Does things",
		Category:         "code",
		Tags:             []string{"code", "review"},
		MonetizationType: "paid",
		PriceUSD:         4.99,
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, "draft", skill.Status)
	assert.Equal(t, pq.StringArray{"code", "review"}, skill.Tags)
	assert.Equal(t, 4.99, skill.PriceUSD)
	assert.Equal(t, int64(1), logCount(t, db, skill.ID, "create"))
}

func TestCreateSkill_NilTags_StoresAsEmptyNotNil(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "no-tags-skill", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)
	// This is the exact shape of the bug the P5 walkthrough hit: a nil
	// Tags here must not round-trip as SQL NULL / come back nil.
	require.NotNil(t, skill.Tags)
	assert.Empty(t, skill.Tags)

	var reloaded model.Skill
	require.NoError(t, db.First(&reloaded, skill.ID).Error)
	require.NotNil(t, reloaded.Tags)
	assert.Empty(t, reloaded.Tags)
}

func TestCreateSkill_EmptyMonetizationType_DefaultsToFree(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "default-monetization", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, "free", skill.MonetizationType)
}

func TestCreateSkill_DuplicateSlug_ReturnsErrSlugTaken(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	req := mktsvc.CreateSkillRequest{Slug: "dup-slug", Name: "n", Description: "d", Category: "c"}
	_, err := svc.CreateSkill(req, 1)
	require.NoError(t, err)

	_, err = svc.CreateSkill(req, 1)
	require.ErrorIs(t, err, mktsvc.ErrSlugTaken)
}

// ── UpdateSkill ───────────────────────────────────────────────────────────────

func TestUpdateSkill_PartialUpdate_LeavesOtherFieldsAlone(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "partial-update", Name: "Old Name", Description: "Old Description", Category: "old-cat",
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Name: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "Old Description", updated.Description)
	assert.Equal(t, "old-cat", updated.Category)
}

func TestUpdateSkill_TagsReplaced(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "tags-update", Name: "n", Description: "d", Category: "c", Tags: []string{"old"},
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{
		Tags: []string{"new", "tags"},
	})
	require.NoError(t, err)
	assert.Equal(t, pq.StringArray{"new", "tags"}, updated.Tags)
}

func TestUpdateSkill_PriceUSDPointer_NilLeavesPriceUnchanged(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "price-unchanged", Name: "n", Description: "d", Category: "c",
		MonetizationType: "paid", PriceUSD: 9.99,
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Name: "renamed"})
	require.NoError(t, err)
	assert.Equal(t, 9.99, updated.PriceUSD)
}

func TestUpdateSkill_NotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.UpdateSkill(9999, mktsvc.UpdateSkillRequest{Name: "x"})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// PRD §11 AC-6 lists exactly which actions are logged, and plain metadata
// edits are not among them — only publish/deprecate/republish/delete/
// version_*/featured_update are. Pin that down explicitly so a future
// "let's log everything" change doesn't silently drift from the PRD.
func TestUpdateSkill_DoesNotWriteAuditLog(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "no-log-on-update", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)

	_, err = svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Name: "renamed"})
	require.NoError(t, err)

	var total int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM skill_admin_logs WHERE skill_id = ?`, created.ID,
	).Scan(&total).Error)
	assert.Equal(t, int64(1), total, "only the create log should exist")
}

// ── GetSkill ──────────────────────────────────────────────────────────────────

func TestGetSkill_ReturnsSkillWithActiveVersion(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skillID := insertSkill(t, db, "get-skill", "published")
	versionID := insertVersion(t, db, skillID, "1.2.0", "active")
	require.NoError(t, db.Exec(
		`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID,
	).Error)

	skill, err := svc.GetSkill(skillID)
	require.NoError(t, err)
	assert.Equal(t, "get-skill", skill.Slug)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, "1.2.0", skill.ActiveVersion)
	require.NotNil(t, skill.ActiveVersionID)
	assert.Equal(t, versionID, *skill.ActiveVersionID)
}

func TestGetSkill_NoActiveVersion_ReturnsEmptyActiveVersion(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skillID := insertSkill(t, db, "no-active-version", "draft")

	skill, err := svc.GetSkill(skillID)
	require.NoError(t, err)
	assert.Equal(t, "", skill.ActiveVersion)
	assert.Nil(t, skill.ActiveVersionID)
}

func TestGetSkill_NotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.GetSkill(9999)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── Publish ───────────────────────────────────────────────────────────────────

// setActiveVersion inserts an active version for skillID and points
// skills.active_version_id at it — the precondition PublishSkill now enforces.
func setActiveVersion(t *testing.T, db *gorm.DB, skillID int64) int64 {
	t.Helper()
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")
	require.NoError(t, db.Exec(
		`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID,
	).Error)
	return versionID
}

func TestPublishSkill_FromDraft(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "test-slug", "draft")
	setActiveVersion(t, db, id)
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, int64(1), logCount(t, db, id, "publish"))
}

func TestPublishSkill_FromDeprecated_WritesRepublish(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-slug", "deprecated")
	setActiveVersion(t, db, id)
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, int64(0), logCount(t, db, id, "publish"))
	assert.Equal(t, int64(1), logCount(t, db, id, "republish"))
}

// Regression: ErrNoActiveVersion existed since P1 but was never actually
// returned — PublishSkill would happily publish a skill with no active
// version. PRD §7.1 requires active_version_id to be set before either
// draft or deprecated can transition to published.
func TestPublishSkill_NoActiveVersion_FromDraft_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "no-version-draft", "draft")
	_, err := svc.PublishSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrNoActiveVersion)
	assert.Equal(t, int64(0), logCount(t, db, id, "publish"))
}

func TestPublishSkill_NoActiveVersion_FromDeprecated_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "no-version-deprecated", "deprecated")
	_, err := svc.PublishSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrNoActiveVersion)
	assert.Equal(t, int64(0), logCount(t, db, id, "republish"))
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
	setActiveVersion(t, db, id)
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
