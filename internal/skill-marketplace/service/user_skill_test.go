package service_test

import (
	"testing"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUserSkills_ReturnsOwnRowsWithStatusAndVersion(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)
	seedUser(t, db, 43, 0)

	skillID, versionID := seedFreeSkill(t, db, "mine")
	_, err := svc.Download(42, "mine")
	require.NoError(t, err)
	_, err = svc.Download(43, "mine")
	require.NoError(t, err)
	// The skill gets deprecated after the download — the list must say so.
	require.NoError(t, db.Exec(`UPDATE skills SET status = 'deprecated' WHERE id = ?`, skillID).Error)

	resp, err := mktsvc.NewUserSkillService(db).ListUserSkills(42, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total, "only the requesting user's rows")
	require.Len(t, resp.Skills, 1)
	entry := resp.Skills[0]
	assert.Equal(t, skillID, entry.SkillID)
	assert.Equal(t, "mine", entry.Slug)
	assert.Equal(t, "1.0.0", entry.Version)
	assert.Equal(t, "deprecated", entry.SkillStatus)
	_ = versionID
}

// PRD §13 AC-11 — a reference listing must not appear in My Skills. It never
// gets a user_enabled_skills row in the first place (Download rejects it
// before that write), so ListUserSkills' inner join can never surface one;
// this pins that guarantee at the query level rather than trusting the
// absence of a write path to hold forever.
func TestListUserSkills_ExcludesReferenceListings(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)

	hostedID, _ := seedFreeSkill(t, db, "hosted-one")
	_, err := svc.Download(42, "hosted-one")
	require.NoError(t, err)

	refID := insertSkillRow(t, db, "ref-one", "published", "video", false, 0, "2026-01-01 00:00:00")
	require.NoError(t, db.Exec(
		`UPDATE skills SET listing_type = 'reference', source_url = 'https://github.com/owner/repo' WHERE id = ?`,
		refID).Error)
	_, err = svc.Download(42, "ref-one")
	require.ErrorIs(t, err, mktsvc.ErrReferenceListingNotDownloadable)

	resp, err := mktsvc.NewUserSkillService(db).ListUserSkills(42, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total, "the reference listing must not count")
	require.Len(t, resp.Skills, 1)
	assert.Equal(t, hostedID, resp.Skills[0].SkillID)
}

func TestListUserSkills_EmptyIsAnEmptyListNotNull(t *testing.T) {
	db := setupDownloadDB(t)
	seedUser(t, db, 42, 0)

	resp, err := mktsvc.NewUserSkillService(db).ListUserSkills(42, 1, 20)
	require.NoError(t, err)
	assert.NotNil(t, resp.Skills)
	assert.Equal(t, int64(0), resp.Total)
}

func TestListUserPurchases_OnlyPaidPurchases(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 5_000_000)

	seedFreeSkill(t, db, "free-one")
	paidID, _ := seedPaidSkill(t, db, "paid-one", 2.00)

	_, err := svc.Download(42, "free-one")
	require.NoError(t, err)
	_, err = svc.Download(42, "paid-one")
	require.NoError(t, err)

	resp, err := mktsvc.NewUserSkillService(db).ListUserPurchases(42, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total, "free downloads never show up as purchases")
	require.Len(t, resp.Purchases, 1)
	entry := resp.Purchases[0]
	assert.Equal(t, paidID, entry.SkillID)
	assert.Equal(t, "paid-one", entry.Slug)
	assert.Equal(t, 2.00, entry.PriceUSD)
}

func TestListUserSkills_Pagination(t *testing.T) {
	db := setupDownloadDB(t)
	svc := newDownloadSvc(db)
	seedUser(t, db, 42, 0)

	seedFreeSkill(t, db, "a-skill")
	seedFreeSkill(t, db, "b-skill")
	seedFreeSkill(t, db, "c-skill")
	for _, slug := range []string{"a-skill", "b-skill", "c-skill"} {
		_, err := svc.Download(42, slug)
		require.NoError(t, err)
	}

	resp, err := mktsvc.NewUserSkillService(db).ListUserSkills(42, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total)
	assert.Len(t, resp.Skills, 1)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 2, resp.Limit)
}
