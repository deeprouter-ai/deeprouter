package service_test

import (
	"fmt"
	"strings"
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
			status  INTEGER NOT NULL DEFAULT 1,
			quota   INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			slug              TEXT UNIQUE NOT NULL,
			tags              TEXT NOT NULL DEFAULT '{}',
			name              TEXT NOT NULL DEFAULT '',
			description       TEXT NOT NULL DEFAULT '',
			status            TEXT NOT NULL DEFAULT 'draft',
			monetization_type TEXT NOT NULL DEFAULT 'free',
			price_usd         REAL NOT NULL DEFAULT 0,
			featured_flag     INTEGER DEFAULT 0,
			featured_rank     INTEGER DEFAULT 0,
			active_version_id INTEGER,
			listing_type      TEXT NOT NULL DEFAULT 'hosted',
			source_url        TEXT,
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
		`CREATE TABLE IF NOT EXISTS user_enabled_skills (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			skill_id   INTEGER NOT NULL,
			version_id INTEGER NOT NULL,
			enabled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, skill_id)
		)`,
		`CREATE TABLE IF NOT EXISTS skill_purchases (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id        INTEGER NOT NULL,
			skill_id       INTEGER NOT NULL,
			price_usd      REAL NOT NULL,
			quota_deducted INTEGER NOT NULL,
			purchased_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, skill_id)
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
		`INSERT INTO skills (slug, name, description, status, created_by) VALUES (?, ?, ?, ?, ?)`,
		slug, "Test Skill", "Test Description", status, 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func insertReferenceSkill(t *testing.T, db *gorm.DB, slug, status, sourceURL string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, status, listing_type, source_url, created_by)
		 VALUES (?, ?, ?, ?, 'reference', ?, ?)`,
		slug, "Test Skill", "Test Description", status, sourceURL, 1,
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
// every load — status filter, tags filter, pagination and the
// active_version join — and none of it had a single dedicated test. The
// controller test only checks Total==1 on an unfiltered call.

// insertSkillWithTags writes tags in the same serialized shape
// pq.StringArray.Value() produces ({"a","b"}), matching what
// tagsOverlapWhere's SQLite branch expects to LIKE against.
func insertSkillWithTags(t *testing.T, db *gorm.DB, slug, status string, tags []string) int64 {
	t.Helper()
	serialized := `{"` + strings.Join(tags, `","`) + `"}`
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, tags, status, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, "Test Skill", "d", serialized, status, 1,
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

func TestListSkills_FiltersByTags(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithTags(t, db, "writing-skill", "draft", []string{"writing"})
	insertSkillWithTags(t, db, "code-skill", "draft", []string{"code"})

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Tags: []string{"code"}, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "code-skill", resp.Skills[0].Slug)
}

// A skill matching any one of the requested tags is returned (OR semantics,
// Skill Marketplace V2 PRD §16.2) — not only a skill matching all of them.
func TestListSkills_FiltersByTags_ORSemantics(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithTags(t, db, "writing-only", "draft", []string{"writing"})
	insertSkillWithTags(t, db, "code-only", "draft", []string{"code"})
	insertSkillWithTags(t, db, "neither", "draft", []string{"research"})

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Tags: []string{"writing", "code"}, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	slugs := []string{resp.Skills[0].Slug, resp.Skills[1].Slug}
	assert.ElementsMatch(t, []string{"writing-only", "code-only"}, slugs)
}

// See the identical case on the public listing side
// (TestListPublishedSkills_TagsFilter_CaseInsensitive) for why this matters:
// tags are free text, the filter buttons always send their fixed lowercase
// value.
func TestListSkills_FiltersByTags_CaseInsensitive(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithTags(t, db, "writing-skill", "draft", []string{"Writing"})

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Tags: []string{"writing"}, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "writing-skill", resp.Skills[0].Slug)
}

func TestListSkills_StatusAndTagsCombine(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)
	insertSkillWithTags(t, db, "match", "published", []string{"code"})
	insertSkillWithTags(t, db, "wrong-status", "draft", []string{"code"})
	insertSkillWithTags(t, db, "wrong-tag", "published", []string{"writing"})

	resp, err := svc.ListSkills(mktsvc.ListSkillsRequest{Status: "published", Tags: []string{"code"}, Page: 1, PageSize: 20})
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
		`INSERT INTO skills (slug, name, description, status, created_by) VALUES (?, ?, ?, ?, ?)`,
		slug, name, "d", status, 1,
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
// duplicate-slug handling all went untested since P1 shipped. Also covers the
// tags mapping regression fixed alongside it: a bare []string reached PG as a
// "record" (SQLSTATE 42804), so CreateSkill failed on every real PG database;
// pq.StringArray round-trips through GORM's Create and read-back correctly.

func TestCreateSkill_Success(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug:             "new-skill",
		Name:             "New Skill",
		Description:      "Does things",
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
		Slug: "no-tags-skill", Name: "n", Description: "d",
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
		Slug: "default-monetization", Name: "n", Description: "d",
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, "free", skill.MonetizationType)
}

func TestCreateSkill_DuplicateSlug_ReturnsErrSlugTaken(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	req := mktsvc.CreateSkillRequest{Slug: "dup-slug", Name: "n", Description: "d"}
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
			Slug: bad, Name: "n", Description: "d",
		}, 1)
		require.ErrorIsf(t, err, mktsvc.ErrInvalidSlugFormat, "slug %q should have been rejected", bad)
	}
}

// monetization_type/price_usd were previously only checked by the DB's
// skills_monetization_check/skills_price_check constraints — a bypass of
// the frontend's zod schema got a raw Postgres error wrapped in a 500
// instead of a clean 400. These pin the Go-level mirror of that same rule.
func TestCreateSkill_InvalidMonetizationType_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "bad-monetization", Name: "n", Description: "d",
		MonetizationType: "subscription",
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidMonetizationType)
}

func TestCreateSkill_PaidWithZeroPrice_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "paid-zero-price", Name: "n", Description: "d",
		MonetizationType: "paid", PriceUSD: 0,
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrPriceRequiredForPaid)
}

// ── CreateSkill: reference listings (PRD §13) ───────────────────────────────

func TestCreateSkill_EmptyListingType_DefaultsToHosted(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "default-listing-type", Name: "n", Description: "d",
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, model.SkillListingTypeHosted, skill.ListingType)
	assert.Nil(t, skill.SourceURL)
}

func TestCreateSkill_InvalidListingType_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "bad-listing-type", Name: "n", Description: "d",
		ListingType: "hosted-and-reference",
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrInvalidListingType)
}

func TestCreateSkill_Reference_Success(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "ref-skill", Name: "n", Description: "d",
		ListingType: "reference", SourceURL: "https://github.com/owner/repo",
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, model.SkillListingTypeReference, skill.ListingType)
	require.NotNil(t, skill.SourceURL)
	assert.Equal(t, "https://github.com/owner/repo", *skill.SourceURL)
	assert.Equal(t, "free", skill.MonetizationType)
}

func TestCreateSkill_Reference_MissingSourceURL_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "ref-no-url", Name: "n", Description: "d",
		ListingType: "reference",
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrSourceURLRequired)
}

// isValidSourceURL is deliberately loose (PRD §13.3) — these pin exactly
// which malformed inputs it rejects vs. accepts, since "loose" is easy to
// accidentally make "accepts anything".
func TestCreateSkill_Reference_InvalidSourceURLFormat_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	for _, bad := range []string{
		"github.com/owner/repo", // missing scheme
		"not a url at all",
		"://missing-scheme.com",
	} {
		_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
			Slug: "ref-bad-url", Name: "n", Description: "d",
			ListingType: "reference", SourceURL: bad,
		}, 1)
		require.ErrorIsf(t, err, mktsvc.ErrInvalidSourceURLFormat, "source_url %q should have been rejected", bad)
	}
}

// No domain allowlist and no https-only requirement — PRD §13.3 explicitly
// rejected both restrictions, so this pins http:// and a non-GitHub host as
// accepted, not just documents the rejection cases above.
func TestCreateSkill_Reference_AcceptsHTTPAndNonGitHubHosts(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	for i, ok := range []string{
		"http://example.com/repo",
		"https://gitlab.com/owner/repo",
	} {
		skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
			Slug: fmt.Sprintf("ref-ok-url-%d", i), Name: "n", Description: "d",
			ListingType: "reference", SourceURL: ok,
		}, 1)
		require.NoErrorf(t, err, "source_url %q should have been accepted", ok)
		require.NotNil(t, skill.SourceURL)
		assert.Equal(t, ok, *skill.SourceURL)
	}
}

func TestCreateSkill_Reference_Paid_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "ref-paid", Name: "n", Description: "d",
		ListingType: "reference", SourceURL: "https://github.com/owner/repo",
		MonetizationType: "paid", PriceUSD: 5,
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrReferenceMustBeFree)
}

func TestCreateSkill_Hosted_SourceURLProvided_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "hosted-with-url", Name: "n", Description: "d",
		SourceURL: "https://github.com/owner/repo",
	}, 1)
	require.ErrorIs(t, err, mktsvc.ErrSourceURLOnHostedSkill)
}

// ── UpdateSkill ───────────────────────────────────────────────────────────────

func TestUpdateSkill_PartialUpdate_LeavesOtherFieldsAlone(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "partial-update", Name: "Old Name", Description: "Old Description",
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Name: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "Old Description", updated.Description)
}

func TestUpdateSkill_TagsReplaced(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "tags-update", Name: "n", Description: "d", Tags: []string{"old"},
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{
		Tags: []string{"new", "tags"},
	})
	require.NoError(t, err)
	assert.Equal(t, pq.StringArray{"new", "tags"}, updated.Tags)
}

// Tags are typed free-hand by the Admin but the marketplace filter buttons
// send fixed lowercase values, so storage is canonical: trimmed, lowercase,
// no blanks, no duplicates, first-seen order kept.
func TestCreateSkill_NormalizesTags(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	skill, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "norm-create", Name: "n", Description: "d",
		Tags: []string{" Writing ", "CODE", "writing", "", "  ", "Data-Analysis"},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, pq.StringArray{"writing", "code", "data-analysis"}, skill.Tags)

	var reloaded model.Skill
	require.NoError(t, db.First(&reloaded, skill.ID).Error)
	assert.Equal(t, pq.StringArray{"writing", "code", "data-analysis"}, reloaded.Tags)
}

func TestUpdateSkill_NormalizesTags(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "norm-update", Name: "n", Description: "d", Tags: []string{"old"},
	}, 1)
	require.NoError(t, err)

	updated, err := svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{
		Tags: []string{"Research", " research", "LEGAL"},
	})
	require.NoError(t, err)
	assert.Equal(t, pq.StringArray{"research", "legal"}, updated.Tags)
}

func TestUpdateSkill_PriceUSDPointer_NilLeavesPriceUnchanged(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "price-unchanged", Name: "n", Description: "d",
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
		Slug: "old-slug", Name: "n", Description: "d",
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
		Slug: "valid-slug", Name: "n", Description: "d",
	}, 1)
	require.NoError(t, err)

	_, err = svc.UpdateSkill(created.ID, mktsvc.UpdateSkillRequest{Slug: "Not Valid"})
	require.ErrorIs(t, err, mktsvc.ErrInvalidSlugFormat)
}

func TestUpdateSkill_SlugDuplicate_ReturnsErrSlugTaken(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	_, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "taken-slug", Name: "n", Description: "d",
	}, 1)
	require.NoError(t, err)
	created, err := svc.CreateSkill(mktsvc.CreateSkillRequest{
		Slug: "other-slug", Name: "n", Description: "d",
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

func TestUpdateSkill_InvalidMonetizationType_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "bad-monetization-update", "draft")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{MonetizationType: "subscription"})
	require.ErrorIs(t, err, mktsvc.ErrInvalidMonetizationType)
}

// Validation must check the *effective* post-update state (existing DB
// value + whatever this partial request changes), not just whichever field
// this particular request happens to touch.
func TestUpdateSkill_SwitchToPaid_RejectedWhenExistingPriceIsStillZero(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	// insertSkill leaves monetization_type/price_usd at their DB defaults:
	// 'free' / 0.
	id := insertSkill(t, db, "switch-paid-no-price", "draft")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{MonetizationType: "paid"})
	require.ErrorIs(t, err, mktsvc.ErrPriceRequiredForPaid)
}

func TestUpdateSkill_SwitchToPaid_AllowedWhenExistingPriceIsAlreadyPositive(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "switch-paid-has-price", "draft")
	require.NoError(t, db.Exec(`UPDATE skills SET price_usd = 4.99 WHERE id = ?`, id).Error)

	updated, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{MonetizationType: "paid"})
	require.NoError(t, err)
	assert.Equal(t, "paid", updated.MonetizationType)
}

func TestUpdateSkill_SetPriceZero_RejectedWhileAlreadyPaid(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "already-paid-zero-out", "draft")
	require.NoError(t, db.Exec(
		`UPDATE skills SET monetization_type = 'paid', price_usd = 4.99 WHERE id = ?`, id,
	).Error)

	zero := 0.0
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{PriceUSD: &zero})
	require.ErrorIs(t, err, mktsvc.ErrPriceRequiredForPaid)
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
		Slug: "no-log-on-update", Name: "n", Description: "d",
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

// ── UpdateSkill: reference listings (PRD §13) ───────────────────────────────

func TestUpdateSkill_Reference_SourceURLUpdated(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertReferenceSkill(t, db, "ref-update", "draft", "https://github.com/owner/old")
	skill, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{
		SourceURL: "https://github.com/owner/new",
	})
	require.NoError(t, err)
	require.NotNil(t, skill.SourceURL)
	assert.Equal(t, "https://github.com/owner/new", *skill.SourceURL)
}

func TestUpdateSkill_Reference_InvalidSourceURLFormat_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertReferenceSkill(t, db, "ref-update-bad", "draft", "https://github.com/owner/old")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{SourceURL: "not a url"})
	require.ErrorIs(t, err, mktsvc.ErrInvalidSourceURLFormat)
}

// Same protection as CreateSkill's version: without this, editing an
// existing hosted skill could silently give it a source_url, leaving
// listing_type="hosted" disagreeing with "has a source_url" from that point
// on — the exact inconsistency listing_type exists to prevent.
// listing_type can't be changed via UpdateSkillRequest, so this checks the
// same rule CreateSkill enforces (a reference listing must stay free) from
// the update side, where skill.ListingType (not a request field) is what's
// authoritative.
func TestUpdateSkill_Reference_SwitchToPaid_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertReferenceSkill(t, db, "ref-no-paid", "draft", "https://github.com/owner/repo")
	price := 5.0
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{
		MonetizationType: "paid", PriceUSD: &price,
	})
	require.ErrorIs(t, err, mktsvc.ErrReferenceMustBeFree)
}

func TestUpdateSkill_Hosted_SourceURLProvided_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "hosted-update", "draft")
	_, err := svc.UpdateSkill(id, mktsvc.UpdateSkillRequest{
		SourceURL: "https://github.com/owner/repo",
	})
	require.ErrorIs(t, err, mktsvc.ErrSourceURLOnHostedSkill)
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

// ── PublishSkill: reference listings (PRD §13) ──────────────────────────────

func TestPublishSkill_Reference_WithSourceURL_Succeeds(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertReferenceSkill(t, db, "ref-publish", "draft", "https://github.com/owner/repo")
	skill, err := svc.PublishSkill(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "published", skill.Status)
	assert.Equal(t, int64(1), logCount(t, db, id, "publish"))
}

// Defends canPublish's branch itself, not just the CHECK constraint: a
// reference skill can never actually reach this state through CreateSkill
// (skills_reference_source_check forbids it), so this constructs one
// directly to prove PublishSkill's own gate would still catch it if that
// constraint were ever bypassed — mirrors
// TestPublishSkill_NoActiveVersion_FromDraft_Rejected's hosted-side coverage.
func TestPublishSkill_Reference_NoSourceURL_Rejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertReferenceSkill(t, db, "ref-no-url-publish", "draft", "https://github.com/owner/repo")
	require.NoError(t, db.Exec(`UPDATE skills SET source_url = NULL WHERE id = ?`, id).Error)

	_, err := svc.PublishSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrSourceURLRequired)
	assert.Equal(t, int64(0), logCount(t, db, id, "publish"))
}

// hosted's existing gate (ErrNoActiveVersion) must still fire on its own
// terms — a reference skill's absent active_version_id must not leak into
// the hosted branch's check.
func TestPublishSkill_Hosted_StillRequiresActiveVersion(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminSkillService(db)

	id := insertSkill(t, db, "hosted-no-version", "draft")
	_, err := svc.PublishSkill(id, 1)
	require.ErrorIs(t, err, mktsvc.ErrNoActiveVersion)
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
