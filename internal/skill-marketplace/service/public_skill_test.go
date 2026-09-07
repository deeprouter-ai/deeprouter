package service_test

import (
	"testing"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// insertSkillRow inserts a skill with full control over the listing-relevant
// columns, returning its id. Raw SQL for the same reason as insertSkill.
func insertSkillRow(t *testing.T, db *gorm.DB, slug, status, category string, featured bool, rank int, createdAt string) int64 {
	t.Helper()
	f := 0
	if featured {
		f = 1
	}
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, category, status, featured_flag, featured_rank, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		slug, "Name "+slug, "Description "+slug, category, status, f, rank, createdAt,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func listSlugs(resp *mktsvc.PublicListResponse) []string {
	slugs := make([]string, len(resp.Skills))
	for i, s := range resp.Skills {
		slugs[i] = s.Slug
	}
	return slugs
}

func TestListPublishedSkills_FeaturedFirstThenCreatedDesc(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "old-plain", "published", "code", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "new-plain", "published", "code", false, 0, "2026-03-01 00:00:00")
	insertSkillRow(t, db, "feat-rank2", "published", "code", true, 2, "2026-02-01 00:00:00")
	insertSkillRow(t, db, "feat-rank1", "published", "code", true, 1, "2026-01-15 00:00:00")
	// A stale rank on a non-featured skill must not reorder the plain group.
	insertSkillRow(t, db, "stale-rank", "published", "code", false, 5, "2026-02-15 00:00:00")

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.Total)
	assert.Equal(t,
		[]string{"feat-rank1", "feat-rank2", "new-plain", "stale-rank", "old-plain"},
		listSlugs(resp))
}

func TestListPublishedSkills_ExcludesDraftAndDeprecated(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "pub", "published", "code", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "dra", "draft", "code", false, 0, "2026-01-02 00:00:00")
	insertSkillRow(t, db, "dep", "deprecated", "code", false, 0, "2026-01-03 00:00:00")

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, []string{"pub"}, listSlugs(resp))
}

func TestListPublishedSkills_Pagination(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "s1", "published", "code", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "s2", "published", "code", false, 0, "2026-01-02 00:00:00")
	insertSkillRow(t, db, "s3", "published", "code", false, 0, "2026-01-03 00:00:00")

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{Page: 2, Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 2, resp.Limit)
	assert.Equal(t, []string{"s1"}, listSlugs(resp))
}

func TestListPublishedSkills_LimitClampedTo100(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{Limit: 500})
	require.NoError(t, err)
	assert.Equal(t, 100, resp.Limit)
}

func TestListPublishedSkills_CategoryFilter(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "w1", "published", "writing", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "c1", "published", "code", false, 0, "2026-01-02 00:00:00")

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{Category: "writing"})
	require.NoError(t, err)
	assert.Equal(t, []string{"w1"}, listSlugs(resp))
}

func TestListPublishedSkills_SearchIsCaseInsensitive(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	id := insertSkillRow(t, db, "review", "published", "code", false, 0, "2026-01-01 00:00:00")
	require.NoError(t, db.Exec(
		`UPDATE skills SET name = 'Code Review Expert', description = 'Reviews pull requests' WHERE id = ?`, id).Error)
	insertSkillRow(t, db, "other", "published", "code", false, 0, "2026-01-02 00:00:00")

	// Matches name, any case.
	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{Q: "cOdE rEvIeW"})
	require.NoError(t, err)
	assert.Equal(t, []string{"review"}, listSlugs(resp))

	// Matches description too.
	resp, err = svc.ListPublishedSkills(mktsvc.PublicListRequest{Q: "PULL REQUESTS"})
	require.NoError(t, err)
	assert.Equal(t, []string{"review"}, listSlugs(resp))
}

func TestListPublishedSkills_IncludesActiveVersionString(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	skillID := insertSkillRow(t, db, "versioned", "published", "code", false, 0, "2026-01-01 00:00:00")
	versionID := insertVersion(t, db, skillID, "1.2.0", "active")
	require.NoError(t, db.Exec(`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID).Error)

	resp, err := svc.ListPublishedSkills(mktsvc.PublicListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, "1.2.0", resp.Skills[0].Version)
}

// ── Detail ────────────────────────────────────────────────────────────────────

func TestGetSkillBySlug_PublishedAndDeprecatedReturn(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "pub", "published", "code", false, 0, "2026-01-01 00:00:00")
	insertSkillRow(t, db, "dep", "deprecated", "code", false, 0, "2026-01-02 00:00:00")

	detail, err := svc.GetSkillBySlug("pub")
	require.NoError(t, err)
	assert.Equal(t, "published", detail.Status)

	detail, err = svc.GetSkillBySlug("dep")
	require.NoError(t, err)
	assert.Equal(t, "deprecated", detail.Status)
}

func TestGetSkillBySlug_DraftAndUnknownAreNotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	insertSkillRow(t, db, "dra", "draft", "code", false, 0, "2026-01-01 00:00:00")

	_, err := svc.GetSkillBySlug("dra")
	assert.ErrorIs(t, err, mktsvc.ErrSkillNotAvailable)

	_, err = svc.GetSkillBySlug("nope")
	assert.ErrorIs(t, err, mktsvc.ErrSkillNotAvailable)
}

func TestGetSkillBySlug_CarriesVersionAndChangelog(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewPublicSkillService(db)

	skillID := insertSkillRow(t, db, "detailed", "published", "code", false, 0, "2026-01-01 00:00:00")
	versionID := insertVersion(t, db, skillID, "2.0.0", "active")
	require.NoError(t, db.Exec(`UPDATE skill_versions SET changelog = 'Big rewrite' WHERE id = ?`, versionID).Error)
	require.NoError(t, db.Exec(`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID).Error)

	detail, err := svc.GetSkillBySlug("detailed")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", detail.Version)
	assert.Equal(t, "Big rewrite", detail.Changelog)
}
