package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Minimal stand-ins for the tables seedInitialSkills touches. Raw SQL rather
// than AutoMigrate because the real DDL (TEXT[], JSONB, NOW()) is
// Postgres-only; the real-Postgres test below runs the real Migrate().
func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, role INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL,
			tags TEXT, status TEXT NOT NULL, monetization_type TEXT NOT NULL,
			price_usd REAL NOT NULL DEFAULT 0, listing_type TEXT NOT NULL,
			source_url TEXT, created_by INTEGER NOT NULL,
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
		`CREATE TABLE skill_admin_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL,
			skill_id INTEGER, action TEXT NOT NULL, details TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	return db
}

func seededSlugs(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var slugs []string
	require.NoError(t, db.Raw(`SELECT slug FROM skills ORDER BY slug`).Scan(&slugs).Error)
	return slugs
}

func TestSeedInitialSkills_WritesAllAsPublishedFreeReferences(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, role) VALUES (1, 1), (7, 100)`).Error)

	require.NoError(t, seedInitialSkills(db))

	require.Len(t, initialSkills, 6)
	var rows []struct {
		Slug, Status, ListingType, MonetizationType string
		SourceURL                                   string
		CreatedBy                                   int
	}
	require.NoError(t, db.Raw(`SELECT slug, status, listing_type, monetization_type, source_url, created_by FROM skills`).Scan(&rows).Error)
	require.Len(t, rows, len(initialSkills))
	for _, r := range rows {
		assert.Equal(t, SkillStatusPublished, r.Status, r.Slug)
		assert.Equal(t, SkillListingTypeReference, r.ListingType, r.Slug)
		assert.Equal(t, "free", r.MonetizationType, r.Slug)
		assert.Contains(t, r.SourceURL, "https://github.com/", r.Slug)
		assert.Equal(t, 7, r.CreatedBy, "%s must be attributed to the root admin", r.Slug)
	}

	var logs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM skill_admin_logs WHERE action = ? AND admin_id = 7`, LogActionCreate).Scan(&logs).Error)
	assert.Equal(t, int64(len(initialSkills)), logs, "every seeded skill gets an audit log entry")
}

// Seeding is one-shot: once recorded, an Admin deleting a seeded skill must
// not see it come back on the next restart.
func TestSeedInitialSkills_RunsOnlyOnce(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, role) VALUES (1, 100)`).Error)
	require.NoError(t, seedInitialSkills(db))

	require.NoError(t, db.Exec(`DELETE FROM skills WHERE slug = ?`, initialSkills[0].Slug).Error)
	require.NoError(t, seedInitialSkills(db))

	assert.Len(t, seededSlugs(t, db), len(initialSkills)-1)
}

// No root admin yet (a brand-new install before first login): skip without
// recording the run, so the next start tries again.
func TestSeedInitialSkills_NoRootAdminSkipsAndRetriesLater(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, role) VALUES (1, 1)`).Error)

	require.NoError(t, seedInitialSkills(db))
	assert.Empty(t, seededSlugs(t, db))

	require.NoError(t, db.Exec(`UPDATE users SET role = 100 WHERE id = 1`).Error)
	require.NoError(t, seedInitialSkills(db))
	assert.Len(t, seededSlugs(t, db), len(initialSkills))
}

// A skill an Admin already created under the same slug is left untouched.
func TestSeedInitialSkills_DoesNotOverwriteExistingSlug(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, role) VALUES (1, 100)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, tags, status, monetization_type, listing_type, created_by, created_at, updated_at)
		 VALUES (?, 'Mine', 'admin made this', '{}', 'draft', 'free', 'hosted', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		initialSkills[0].Slug).Error)

	require.NoError(t, seedInitialSkills(db))

	var name, status string
	require.NoError(t, db.Raw(`SELECT name, status FROM skills WHERE slug = ?`, initialSkills[0].Slug).Row().Scan(&name, &status))
	assert.Equal(t, "Mine", name)
	assert.Equal(t, "draft", status)
	assert.Len(t, seededSlugs(t, db), len(initialSkills))
}

// Seeded tags must already be in the canonical stored form (lowercase,
// trimmed, no duplicates) and drawn from the marketplace's filter labels.
func TestInitialSkills_TagsAreCanonicalFilterLabels(t *testing.T) {
	labels := map[string]bool{
		"writing": true, "translation": true, "code": true, "data-analysis": true,
		"research": true, "legal": true, "finance": true, "video": true,
	}
	for _, s := range initialSkills {
		require.NotEmpty(t, s.Tags, s.Slug)
		seen := map[string]bool{}
		for _, tag := range s.Tags {
			assert.True(t, labels[tag], "%s: tag %q is not a marketplace filter label", s.Slug, tag)
			assert.False(t, seen[tag], "%s: duplicate tag %q", s.Slug, tag)
			seen[tag] = true
		}
	}
}

func TestSeedInitialSkills_OnRealPostgres(t *testing.T) {
	db := newPostgresDatabase(t)

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, role INT NOT NULL DEFAULT 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (role) VALUES (100)`).Error)

	require.NoError(t, Migrate(db))
	assert.Len(t, seededSlugs(t, db), len(initialSkills))

	var tags []string
	require.NoError(t, db.Raw(`SELECT unnest(tags) FROM skills WHERE slug = ?`, "youtube-script-writer").Scan(&tags).Error)
	assert.ElementsMatch(t, []string{"video", "writing"}, tags)

	require.NoError(t, db.Exec(`DELETE FROM skills WHERE slug = ?`, initialSkills[0].Slug).Error)
	require.NoError(t, Migrate(db))
	assert.Len(t, seededSlugs(t, db), len(initialSkills)-1, "a deleted seeded skill must not come back")
}
