package model_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Migrate() is PostgreSQL-only raw DDL (TEXT[], JSONB, partial indexes) and
// was never once run against a real Postgres server before this test existed
// — every other test in this package uses SQLite, which tolerates things a
// real PG driver rejects. Gated on TEST_POSTGRES_DSN (an admin/superuser DSN
// pointed at the `postgres` maintenance database, e.g.
// postgresql://root:123456@localhost:5432/postgres) so it only runs when a
// real server is available; skipped otherwise, same convention as
// controller/token_test.go's TEST_POSTGRES_DSN-gated tests.
func TestMigrate_RunsAgainstRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres migration test")
	}

	testDSN := createScratchDatabase(t, adminDSN)

	// PrepareStmt: true matches model/main.go's real connection config
	// exactly — it's *why* the bug this test exists for only shows up
	// against a real server: PrepareStmt forces the extended query
	// protocol, which is what rejects multiple commands in one Exec.
	// Without it, the same buggy SQL runs (simple protocol tolerates
	// multi-statement), and this test would pass while production breaks.
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Migrate()'s DDL references users(id) (created_by/admin_id FKs). In the
	// real app this table always exists first — upstream's own migration
	// creates it long before internal/skill-marketplace's Migrate() runs —
	// so a minimal stand-in is the correct fixture here, not a gap.
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)

	// This call alone reproduces the bug this test exists for: three of
	// Migrate()'s six DDL steps packed multiple `CREATE INDEX ...;` commands
	// into a single Exec, which PostgreSQL's extended query protocol
	// rejects outright (SQLSTATE 42601, "cannot insert multiple commands
	// into a prepared statement") — caught in the P5 real-service
	// walkthrough (2026-09-04); Migrate() had never run against real PG
	// before that.
	require.NoError(t, model.Migrate(db), "first Migrate() run")

	for _, table := range []string{
		"skills",
		"skill_versions",
		"user_enabled_skills",
		"skill_purchases",
		"skill_admin_logs",
	} {
		require.True(t, db.Migrator().HasTable(table), "table %q should exist after Migrate()", table)
	}

	// Every app boot calls Migrate() again against the same database — the
	// IF NOT EXISTS / guarded-DO-block design only holds up if a second run
	// is a genuine no-op.
	require.NoError(t, model.Migrate(db), "second Migrate() run must be idempotent")
}

// TestAddReferenceListingColumns_OnTableWithExistingRows simulates the actual
// upgrade this PR ships into: a database already migrated by a pre-§13
// binary, with real rows in skills, seeing addReferenceListingColumns for the
// first time. The idempotency check above only ever adds these columns to a
// table it just created itself (zero rows) — that proves the statements are
// safe to re-run, not that ADD COLUMN ... NOT NULL DEFAULT behaves on a
// populated table, which is what actually happens on deploy.
func TestAddReferenceListingColumns_OnTableWithExistingRows(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres migration test")
	}

	testDSN := createScratchDatabase(t, adminDSN)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)

	// A skills table shaped exactly like migrateSkills' CREATE TABLE, but
	// built by hand and pre-populated — standing in for a database that a
	// pre-§13 binary already migrated and real Admins already used.
	require.NoError(t, db.Exec(`
		CREATE TABLE skills (
		  id                BIGSERIAL PRIMARY KEY,
		  slug              VARCHAR(100) UNIQUE NOT NULL,
		  name              VARCHAR(200) NOT NULL,
		  description       TEXT NOT NULL,
		  category          VARCHAR(50) NOT NULL,
		  tags              TEXT[] DEFAULT '{}',
		  status            VARCHAR(20) NOT NULL DEFAULT 'draft',
		  monetization_type VARCHAR(10) NOT NULL DEFAULT 'free',
		  price_usd         NUMERIC(10,2) NOT NULL DEFAULT 0,
		  featured_flag     BOOLEAN DEFAULT FALSE,
		  featured_rank     INTEGER DEFAULT 0,
		  active_version_id BIGINT,
		  created_by        BIGINT NOT NULL REFERENCES users(id),
		  created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
		  updated_at        TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO skills (slug, name, description, category, created_by)
		VALUES ('pre-existing', 'Pre-existing skill', 'd', 'writing', 1)
	`).Error)

	require.NoError(t, model.Migrate(db), "Migrate() against skills with a pre-existing row")

	var listingType string
	var sourceURL *string
	require.NoError(t,
		db.Raw(`SELECT listing_type, source_url FROM skills WHERE slug = 'pre-existing'`).
			Row().Scan(&listingType, &sourceURL))
	require.Equal(t, "hosted", listingType, "pre-existing row must backfill to hosted, not break NOT NULL")
	require.Nil(t, sourceURL, "pre-existing row's source_url must stay NULL, not some empty-string default")

	var categoryColumnCount int
	require.NoError(t,
		db.Raw(`SELECT count(*) FROM information_schema.columns
		        WHERE table_name = 'skills' AND column_name = 'category'`).
			Row().Scan(&categoryColumnCount))
	require.Zero(t, categoryColumnCount,
		"dropCategoryColumn must remove category even from a table that had real pre-existing rows")
}

// TestReferenceListing_EndToEndOnRealPostgres exercises the whole PRD §13
// path through the real service layer against a real Postgres database that
// went through the actual Migrate() — not the SQLite fixtures the service
// package's own tests use, which cannot enforce the skills_* CHECK
// constraints and would pass even if migrate.go's DDL were wrong.
func TestReferenceListing_EndToEndOnRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres migration test")
	}

	testDSN := createScratchDatabase(t, adminDSN)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	require.NoError(t, model.Migrate(db))

	adminSvc := service.NewAdminSkillService(db)
	skill, err := adminSvc.CreateSkill(service.CreateSkillRequest{
		Slug: "e2e-ref-skill", Name: "n", Description: "d",
		ListingType: "reference", SourceURL: "https://github.com/owner/repo",
	}, 1)
	require.NoError(t, err, "CreateSkill against real Postgres")

	published, err := adminSvc.PublishSkill(skill.ID, 1)
	require.NoError(t, err, "PublishSkill against real Postgres")
	require.Equal(t, "published", published.Status)

	downloadSvc := service.NewDownloadService(db, func() float64 { return 500_000 })
	_, err = downloadSvc.Download(1, "e2e-ref-skill")
	require.ErrorIs(t, err, service.ErrReferenceListingNotDownloadable,
		"a published reference listing must still refuse to be downloaded")

	// The CHECK constraint, not just Go-level validation: a direct write that
	// bypasses AdminSkillService entirely must still be refused by the
	// database itself.
	err = db.Exec(`
		INSERT INTO skills (slug, name, description, listing_type, source_url, created_by)
		VALUES ('e2e-bad-ref', 'n', 'd', 'reference', NULL, 1)
	`).Error
	require.Error(t, err, "skills_reference_source_check must reject a reference row with no source_url")

	err = db.Exec(`
		INSERT INTO skills (slug, name, description, listing_type, source_url, monetization_type, price_usd, created_by)
		VALUES ('e2e-bad-ref-paid', 'n', 'd', 'reference', 'https://github.com/o/r', 'paid', 5, 1)
	`).Error
	require.Error(t, err, "skills_reference_free_check must reject a paid reference row")
}

// TestTagsFilter_ORSemanticsOnRealPostgres exercises the actual `&&` array
// overlap operator (Skill Marketplace V2 PRD §16) against a real Postgres
// tags text[] column — the service package's own tests only ever run this
// filter through the SQLite LIKE-based approximation in tags_filter.go,
// which proves the branch logic but says nothing about whether `&&` itself
// does what the code assumes on the database it actually runs on.
func TestTagsFilter_ORSemanticsOnRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres tags filter test")
	}

	testDSN := createScratchDatabase(t, adminDSN)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	require.NoError(t, model.Migrate(db))

	adminSvc := service.NewAdminSkillService(db)
	makePublished := func(slug string, tags []string) {
		skill, err := adminSvc.CreateSkill(service.CreateSkillRequest{
			Slug: slug, Name: slug, Description: "d", Tags: tags,
		}, 1)
		require.NoError(t, err, "CreateSkill against real Postgres")
		// A skill needs an active version to publish; skip that ceremony
		// and flip status directly — this test is about the tags filter,
		// not the publish state machine (already covered elsewhere).
		require.NoError(t, db.Exec(
			`UPDATE skills SET status = 'published' WHERE id = ?`, skill.ID,
		).Error)
	}
	makePublished("writing-only", []string{"writing"})
	makePublished("code-only", []string{"code"})
	makePublished("neither", []string{"research"})

	publicSvc := service.NewPublicSkillService(db)
	resp, err := publicSvc.ListPublishedSkills(service.PublicListRequest{
		Tags: []string{"writing", "code"},
	})
	require.NoError(t, err, "ListPublishedSkills against real Postgres")

	var slugs []string
	for _, s := range resp.Skills {
		slugs = append(slugs, s.Slug)
	}
	require.ElementsMatch(t, []string{"writing-only", "code-only"}, slugs,
		"&& must match a skill with ANY requested tag (OR), and must not also return a skill with neither")
}

// TestTagsFilter_CaseInsensitiveOnRealPostgres catches the exact bug a real
// browser click-through found (2026-09-30): the Admin's tags input is free
// text with no suggestions, so nothing stops "Writing" (capital) from being
// typed, while the fixed filter-button list always sends its canonical
// lowercase value. The first implementation used Postgres's raw &&
// operator, which is exact-match — a skill tagged "Writing" silently never
// matched a click on the "writing" button, with no error, just an
// ever-empty result. Real Postgres, not the SQLite approximation, because
// the fix (EXISTS + unnest + LOWER(t) = ANY(?)) is exactly the kind of
// query the SQLite LIKE-based stand-in can't actually verify.
func TestTagsFilter_CaseInsensitiveOnRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres tags filter test")
	}

	testDSN := createScratchDatabase(t, adminDSN)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	require.NoError(t, model.Migrate(db))

	adminSvc := service.NewAdminSkillService(db)
	skill, err := adminSvc.CreateSkill(service.CreateSkillRequest{
		Slug: "case-mismatch-skill", Name: "n", Description: "d", Tags: []string{"Writing"},
	}, 1)
	require.NoError(t, err, "CreateSkill against real Postgres")
	require.NoError(t, db.Exec(
		`UPDATE skills SET status = 'published' WHERE id = ?`, skill.ID,
	).Error)

	publicSvc := service.NewPublicSkillService(db)
	resp, err := publicSvc.ListPublishedSkills(service.PublicListRequest{
		Tags: []string{"writing"},
	})
	require.NoError(t, err, "ListPublishedSkills against real Postgres")
	require.Len(t, resp.Skills, 1,
		"a skill tagged 'Writing' must match a filter request for 'writing' — tags are free text, the filter list is fixed lowercase")
	require.Equal(t, "case-mismatch-skill", resp.Skills[0].Slug)
}

// TestHostedSkillDownload_EndToEndOnRealPostgres closes the one regression
// claim P11 (Skill Marketplace V2 PRD §15) shipped without a real-database
// check: "removing the runner from BuildSkillPackage doesn't break
// downloading an existing hosted skill." That claim rested on "download.go
// itself was never touched" plus the SQLite-backed unit suite — this runs
// the actual create → upload version → activate (where BuildSkillPackage
// runs) → publish → download path against real Postgres, including
// unzipping what comes back.
func TestHostedSkillDownload_EndToEndOnRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres download test")
	}

	testDSN := createScratchDatabase(t, adminDSN)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	require.NoError(t, model.Migrate(db))

	adminSvc := service.NewAdminSkillService(db)
	versionSvc := service.NewAdminVersionService(db)
	downloadSvc := service.NewDownloadService(db, func() float64 { return 500_000 })

	skill, err := adminSvc.CreateSkill(service.CreateSkillRequest{
		Slug: "e2e-hosted-skill", Name: "n", Description: "d", Tags: []string{"code"},
	}, 1)
	require.NoError(t, err, "CreateSkill against real Postgres")

	version, err := versionSvc.UploadVersion(skill.ID, service.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "# E2E Hosted Skill\n\nDoes things.",
		ManifestJSON:   json.RawMessage(`{"slug":"e2e-hosted-skill","version":"1.0.0"}`),
	}, 1)
	require.NoError(t, err, "UploadVersion against real Postgres")

	_, err = versionSvc.ActivateVersion(skill.ID, version.ID, 1)
	require.NoError(t, err, "ActivateVersion against real Postgres — this is where P11's BuildSkillPackage() runs")

	_, err = adminSvc.PublishSkill(skill.ID, 1)
	require.NoError(t, err, "PublishSkill against real Postgres")

	result, err := downloadSvc.Download(1, "e2e-hosted-skill")
	require.NoError(t, err, "Download against real Postgres — this is P11's regression surface")
	require.False(t, result.PurchaseMade, "a free skill must never create a purchase")

	zr, err := zip.NewReader(bytes.NewReader(result.Zip), int64(len(result.Zip)))
	require.NoError(t, err, "downloaded bytes must be a valid ZIP")
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	require.ElementsMatch(t,
		[]string{"e2e-hosted-skill/SKILL.md", "e2e-hosted-skill/manifest.json", "e2e-hosted-skill/README.md"},
		names,
		"P11: a real activation's downloaded ZIP must be exactly the 3 post-runner-removal files")
}

// createScratchDatabase opens the admin DSN's own (maintenance) database,
// creates a uniquely-named database for this test run, and registers
// cleanup to drop it — so this test never touches whatever database the
// admin DSN names, and never leaves debris on a shared server. Returns a
// DSN pointed at the new scratch database.
func createScratchDatabase(t *testing.T, adminDSN string) string {
	t.Helper()

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	require.NoError(t, err, "open admin DSN")
	adminSQLDB, err := adminDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminSQLDB.Close() })

	dbName := fmt.Sprintf("skill_marketplace_migrate_test_%d", time.Now().UnixNano())
	require.NoError(t, adminDB.Exec(fmt.Sprintf(`CREATE DATABASE %q`, dbName)).Error,
		"create scratch database %q", dbName)

	t.Cleanup(func() {
		adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, dbName))
	})

	return replaceDBName(adminDSN, dbName)
}

// replaceDBName swaps a postgres DSN's trailing /<dbname> path segment
// (everything after the last '/' and before an optional '?query').
func replaceDBName(dsn, newName string) string {
	idx := strings.LastIndex(dsn, "/")
	if idx == -1 {
		return dsn
	}
	base := dsn[:idx]
	query := ""
	if q := strings.IndexByte(dsn[idx:], '?'); q != -1 {
		query = dsn[idx:][q:]
	}
	return base + "/" + newName + query
}
