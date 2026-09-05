package service_test

import (
	"fmt"
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

// insertSkillNamed lets a test control `name` independently of `slug` —
// insertSkill hardcodes name to "Test Skill", which is useless for the Q
// (search) tests below where the match target is the name itself.
func insertSkillNamed(t *testing.T, db *gorm.DB, slug, name, status string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, category, status, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, name, "d", "test", status, 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

// The admin list page's search box (skills-table.tsx) was found, during
// live browser testing with more skills than fit on one page, to only ever
// filter the currently-loaded page client-side — a skill on page 2 was
// unfindable while sitting on page 1, because ListSkills had no `q` param
// at all for the frontend to even pass. These tests pin the fix.
func TestListSkills_QMatchesName(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillNamed(t, db, "a-slug", "Code Review Expert", "draft")
	insertSkillNamed(t, db, "b-slug", "Translation Helper", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Q: "Review", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "a-slug", resp.Skills[0].Slug)
}

func TestListSkills_QMatchesSlug(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillNamed(t, db, "code-review-expert", "Name A", "draft")
	insertSkillNamed(t, db, "translation-helper", "Name B", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Q: "code-review", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "code-review-expert", resp.Skills[0].Slug)
}

func TestListSkills_QFindsSkillBeyondTheFirstPage(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	for i := 0; i < 20; i++ {
		insertSkillNamed(t, db, fmt.Sprintf("filler-%d", i), "Filler", "draft")
	}
	insertSkillNamed(t, db, "seed-skill-23", "Seed Skill 23", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Q: "seed-skill-23", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "seed-skill-23", resp.Skills[0].Slug)
}

func TestListSkills_QNoMatch_ReturnsEmpty(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillNamed(t, db, "a-slug", "Code Review Expert", "draft")

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Q: "nonexistent", Page: 1, PageSize: 20})
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

// The slug format regex previously lived only in the frontend zod schema
// (SKILL_SLUG_PATTERN) — a direct API call could create a skill with a slug
// containing spaces or uppercase letters, which P3's public URLs won't
// tolerate. This pins the backend-side mirror of that same pattern.
func TestCreateSkill_InvalidSlugFormat_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	for _, bad := range []string{"Has Spaces", "UPPERCASE", "trailing-", "-leading", "double--hyphen", ""} {
		_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
			Slug: bad, Name: "n", Description: "d", Category: "c",
		}, 1)
		require.ErrorIsf(t, err, mktsvc.ErrInvalidSlugFormat, "slug %q should have been rejected", bad)
	}
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

// AC-9: "Skill 处于 draft 时可修改 slug；published 后修改 slug 返回 409".
// UpdateSkillRequest previously had no Slug field at all — this path did
// not exist in either direction. These four tests pin the fix.
func TestUpdateSkill_SlugChangesWhileDraft(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "old-slug", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Slug: "new-slug"})
	require.NoError(t, err)
	assert.Equal(t, "new-slug", updated.Slug)
}

func TestUpdateSkill_SlugLockedOncePublished(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "pub-slug", "published")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{Slug: "renamed-slug"})
	require.ErrorIs(t, err, mktsvc.ErrSlugLocked)

	var reloaded model.Skill
	require.NoError(t, db.First(&reloaded, id).Error)
	assert.Equal(t, "pub-slug", reloaded.Slug, "slug must not have changed")
}

func TestUpdateSkill_SlugLockedOnceDeprecated(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-slug", "deprecated")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{Slug: "renamed-slug"})
	require.ErrorIs(t, err, mktsvc.ErrSlugLocked)
}

func TestUpdateSkill_SlugInvalidFormat_RejectedEvenInDraft(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "valid-slug", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)

	_, err = svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Slug: "Not Valid"})
	require.ErrorIs(t, err, mktsvc.ErrInvalidSlugFormat)
}

func TestUpdateSkill_SlugDuplicate_ReturnsErrSlugTaken(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "taken-slug", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)
	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "other-slug", Name: "n", Description: "d", Category: "c",
	}, 1)
	require.NoError(t, err)

	_, err = svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Slug: "taken-slug"})
	require.ErrorIs(t, err, mktsvc.ErrSlugTaken)
}

func TestUpdateSkill_SameSlugResubmitted_NoOp(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	// Resubmitting the skill's own current slug (e.g. a form that always
	// includes it) must not trip the draft-only lock on a published skill.
	id := insertSkill(t, db, "same-slug", "published")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{Slug: "same-slug", Name: "renamed"})
	require.NoError(t, err)
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

// UpdateFeatured now rejects non-published skills (see TestUpdateFeatured_
// Rejects* below) — a stale featured_flag=true left over from before a
// skill was deprecated could otherwise silently resurface on republish
// without the Admin ever having re-checked it. Deprecate resets it instead.
func TestDeprecateSkill_ResetsFeaturedFlag(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "dep-featured", "published")
	_, err := svc.UpdateFeatured(id, mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}, 1)
	require.NoError(t, err)

	skill, err := svc.DeprecateSkill(id, 1)
	require.NoError(t, err)
	assert.False(t, skill.FeaturedFlag)
	assert.Equal(t, 0, skill.FeaturedRank)

	var reloaded model.Skill
	require.NoError(t, db.First(&reloaded, id).Error)
	assert.False(t, reloaded.FeaturedFlag)
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

// AC-8 reads "Admin can toggle featured on a *published* skill" — the
// frontend disables the control for draft/deprecated rows, but nothing on
// the backend stopped a direct API call from featuring a skill nobody can
// even see yet. These two pin the fix; nothing gets written on rejection.
func TestUpdateFeatured_RejectsDraftSkill(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "feat-draft", "draft")
	_, err := svc.UpdateFeatured(id, mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}, 1)
	require.ErrorIs(t, err, mktsvc.ErrSkillNotPublished)
	assert.Equal(t, int64(0), logCount(t, db, id, "featured_update"))
}

func TestUpdateFeatured_RejectsDeprecatedSkill(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "feat-dep", "deprecated")
	_, err := svc.UpdateFeatured(id, mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}, 1)
	require.ErrorIs(t, err, mktsvc.ErrSkillNotPublished)
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
