package model_test

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// A plain []string has no driver.Valuer/sql.Scanner for a Postgres text[]
// column, so GORM/pgx silently writes SQL NULL for it instead of erroring —
// caught in the P5 real-service walkthrough (2026-09-04) when a skill
// created with no tags came back from GetSkill as `"tags": null`, which
// crashed the admin edit page's formatTagsInput(null.join). Fixed by typing
// model.Skill.Tags as pq.StringArray; this test guards the DB round trip
// specifically (SQLite's test schema for this package has no tags column at
// all, so no existing test exercised this column against a real driver).
func TestSkill_TagsRoundTripThroughRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres tags round-trip test")
	}

	testDSN := createScratchDatabase(t, adminDSN)

	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, model.Migrate(db))
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)

	t.Run("empty tags round-trip as an empty slice, not nil", func(t *testing.T) {
		skill := &model.Skill{
			Slug:        "empty-tags-skill",
			Name:        "Empty Tags Skill",
			Description: "d",
			Category:    "code",
			Tags:        pq.StringArray{},
			CreatedBy:   1,
		}
		require.NoError(t, db.Create(skill).Error)

		var reloaded model.Skill
		require.NoError(t, db.First(&reloaded, skill.ID).Error)
		require.NotNil(t, reloaded.Tags, "Tags must round-trip as an empty slice — nil crashes the admin edit page")
		require.Empty(t, reloaded.Tags)
	})

	t.Run("non-empty tags round-trip with the same values", func(t *testing.T) {
		skill := &model.Skill{
			Slug:        "tagged-skill",
			Name:        "Tagged Skill",
			Description: "d",
			Category:    "code",
			Tags:        pq.StringArray{"code", "review"},
			CreatedBy:   1,
		}
		require.NoError(t, db.Create(skill).Error)

		var reloaded model.Skill
		require.NoError(t, db.First(&reloaded, skill.ID).Error)
		require.Equal(t, pq.StringArray{"code", "review"}, reloaded.Tags)
	})
}
