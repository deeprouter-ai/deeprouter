package model

// Tests for P10: skills-marketplace timestamp columns were TIMESTAMP (no
// timezone) from P1 through P9 instead of TIMESTAMPTZ. Every environment that
// runs this migration sets the application container's TZ to Asia/Shanghai
// while Postgres defaults to UTC, so a value written as a correct, offset-
// aware instant got silently truncated to its raw Asia/Shanghai wall-clock
// digits on write, then mislabeled as UTC on read — a client that honors the
// resulting "...Z" suffix converts *again*, landing up to 16h off the real
// time. See fixNaiveTimestampColumns in migrate.go for the fix itself.
//
// package model (not model_test), same reason as legacy_v1_test.go:
// fixNaiveTimestampColumns is unexported, and this needs the strict migrate()
// so a failure actually fails the test instead of just logging.
//
// Gated on TEST_POSTGRES_DSN, same convention as migrate_test.go /
// legacy_v1_test.go — this is real Postgres catalog/timezone behaviour, none
// of which SQLite models.

import (
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// preP10FixtureDDL is what migrate.go's CREATE TABLE statements looked like
// before P10 — every timestamp column is TIMESTAMP, not TIMESTAMPTZ. A
// database that ran Migrate() any time between P1 and P9 has exactly this
// shape; P10's job is to upgrade it in place without losing or re-shifting
// data.
const preP10FixtureDDL = `
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
  updated_at        TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT skills_status_check CHECK (status IN ('draft', 'published', 'deprecated')),
  CONSTRAINT skills_monetization_check CHECK (monetization_type IN ('free', 'paid')),
  CONSTRAINT skills_price_check CHECK (monetization_type = 'free' OR price_usd > 0)
);
CREATE INDEX idx_skills_status   ON skills(status);
CREATE INDEX idx_skills_featured ON skills(featured_flag, featured_rank) WHERE status = 'published';
CREATE INDEX idx_skills_created  ON skills(created_at DESC)              WHERE status = 'published';
CREATE TABLE skill_versions (
  id               BIGSERIAL PRIMARY KEY,
  skill_id         BIGINT NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  version          VARCHAR(20) NOT NULL,
  status           VARCHAR(20) NOT NULL DEFAULT 'draft',
  skill_md_content TEXT NOT NULL,
  manifest_json    JSONB NOT NULL,
  package_zip      BYTEA,
  package_sha256   VARCHAR(64),
  package_built_at TIMESTAMP,
  changelog        TEXT DEFAULT '',
  created_by       BIGINT NOT NULL REFERENCES users(id),
  created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (skill_id, version),
  CONSTRAINT skill_versions_status_check CHECK (status IN ('draft', 'active', 'archived')),
  CONSTRAINT skill_versions_semver_check CHECK (version ~ '^\d+\.\d+\.\d+$')
);
ALTER TABLE skills ADD CONSTRAINT fk_skills_active_version
  FOREIGN KEY (active_version_id) REFERENCES skill_versions(id);
CREATE TABLE user_enabled_skills (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id),
  skill_id   BIGINT NOT NULL REFERENCES skills(id),
  version_id BIGINT NOT NULL REFERENCES skill_versions(id),
  enabled_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, skill_id)
);
CREATE INDEX idx_ues_user ON user_enabled_skills(user_id);
CREATE TABLE skill_purchases (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT NOT NULL REFERENCES users(id),
  skill_id       BIGINT NOT NULL REFERENCES skills(id),
  price_usd      NUMERIC(10,2) NOT NULL,
  quota_deducted BIGINT NOT NULL,
  purchased_at   TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, skill_id)
);
CREATE INDEX idx_sp_user  ON skill_purchases(user_id);
CREATE INDEX idx_sp_skill ON skill_purchases(skill_id);
CREATE TABLE skill_admin_logs (
  id         BIGSERIAL PRIMARY KEY,
  admin_id   BIGINT NOT NULL REFERENCES users(id),
  skill_id   BIGINT REFERENCES skills(id) ON DELETE SET NULL,
  action     VARCHAR(50) NOT NULL,
  details    JSONB DEFAULT '{}',
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sal_skill  ON skill_admin_logs(skill_id);
CREATE INDEX idx_sal_admin  ON skill_admin_logs(admin_id);
`

// newPreP10Database is a scratch database carrying the pre-P10 schema shape
// (TIMESTAMP, not TIMESTAMPTZ), with one row per table seeded at a known
// wall-clock instant so the fix's correctness can be checked against real
// data, not just an empty table.
func newPreP10Database(t *testing.T) *gorm.DB {
	t.Helper()
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	for _, stmt := range splitStatements(preP10FixtureDDL) {
		require.NoError(t, db.Exec(stmt).Error, "pre-P10 fixture: %s", stmt)
	}

	// Seed one row per timestamp column with an explicit raw wall-clock
	// value — this is exactly what the bug produced: Go's time.Now() in an
	// Asia/Shanghai container, truncated to its wall-clock digits by the
	// TIMESTAMP column. 17:42:53 Asia/Shanghai == 09:42:53 UTC.
	require.NoError(t, db.Exec(`
		INSERT INTO skills (slug, name, description, category, created_by, created_at, updated_at)
		VALUES ('seed-skill', 'n', 'd', 'c', 1, '2026-09-12 17:42:53', '2026-09-12 17:42:53')
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO skill_versions (skill_id, version, skill_md_content, manifest_json, package_built_at, created_by, created_at)
		VALUES (1, '1.0.0', '# x', '{}', '2026-09-12 17:42:53', 1, '2026-09-12 17:42:53')
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_enabled_skills (user_id, skill_id, version_id, enabled_at)
		VALUES (1, 1, 1, '2026-09-12 17:42:53')
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO skill_purchases (user_id, skill_id, price_usd, quota_deducted, purchased_at)
		VALUES (1, 1, 2.00, 1000000, '2026-09-12 17:42:53')
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO skill_admin_logs (admin_id, skill_id, action, created_at)
		VALUES (1, 1, 'create', '2026-09-12 17:42:53')
	`).Error)
	return db
}

// wantUTC is the correct UTC instant for every seeded row: 2026-09-12
// 17:42:53 Asia/Shanghai (UTC+8) == 2026-09-12 09:42:53 UTC.
var wantUTC = time.Date(2026, 9, 12, 9, 42, 53, 0, time.UTC)

func TestFixNaiveTimestampColumns_ConvertsExistingDataToTheCorrectInstant(t *testing.T) {
	db := newPreP10Database(t)

	require.NoError(t, fixNaiveTimestampColumns(db))

	for _, c := range timestampColumnsNeedingTZFix {
		actual, err := columnType(db, c.table, c.column)
		require.NoError(t, err)
		assert.Equal(t, "timestamp with time zone", actual,
			"%s.%s should be TIMESTAMPTZ after the fix", c.table, c.column)
	}

	cases := []struct{ table, column string }{
		{"skills", "created_at"},
		{"skills", "updated_at"},
		{"skill_versions", "created_at"},
		{"skill_versions", "package_built_at"},
		{"user_enabled_skills", "enabled_at"},
		{"skill_purchases", "purchased_at"},
		{"skill_admin_logs", "created_at"},
	}
	for _, c := range cases {
		var got time.Time
		err := db.Raw(`SELECT ` + c.column + ` FROM ` + c.table + ` LIMIT 1`).Scan(&got).Error
		require.NoError(t, err, "%s.%s", c.table, c.column)
		assert.True(t, got.UTC().Equal(wantUTC),
			"%s.%s: got %s, want %s (the raw wall-clock digits reinterpreted as Asia/Shanghai, not UTC)",
			c.table, c.column, got.UTC(), wantUTC)
	}
}

// The fix must not shift an already-correct value a second time: once a
// column is TIMESTAMPTZ, re-running `AT TIME ZONE 'Asia/Shanghai'` against
// it would be wrong (it would treat an already-correct absolute instant as
// if it were still a naive Asia/Shanghai wall-clock reading).
func TestFixNaiveTimestampColumns_IsIdempotent(t *testing.T) {
	db := newPreP10Database(t)

	require.NoError(t, fixNaiveTimestampColumns(db))
	var afterFirst time.Time
	require.NoError(t, db.Raw(`SELECT enabled_at FROM user_enabled_skills LIMIT 1`).Scan(&afterFirst).Error)

	require.NoError(t, fixNaiveTimestampColumns(db))
	var afterSecond time.Time
	require.NoError(t, db.Raw(`SELECT enabled_at FROM user_enabled_skills LIMIT 1`).Scan(&afterSecond).Error)

	assert.True(t, afterFirst.Equal(afterSecond),
		"a second run must not shift an already-correct value: first %s, second %s",
		afterFirst, afterSecond)
	assert.True(t, afterSecond.UTC().Equal(wantUTC))
}

func TestFixNaiveTimestampColumns_NoOpOnNonPostgres(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, fixNaiveTimestampColumns(db))
}

// End-to-end: migrate() (the strict version, not the fail-soft Migrate())
// against a database that already ran P1-through-P9's migrations, proving
// the fix works as part of the real boot sequence — not just in isolation —
// and that every other step (assertSchema in particular, since it now
// asserts TIMESTAMPTZ) still passes against the upgraded schema.
func TestMigrate_UpgradesAPreP10Database(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres migration test")
	}
	db := newPreP10Database(t)

	require.NoError(t, migrate(db), "migrate() against a pre-P10 database")

	var got time.Time
	require.NoError(t, db.Raw(`SELECT enabled_at FROM user_enabled_skills LIMIT 1`).Scan(&got).Error)
	assert.True(t, got.UTC().Equal(wantUTC))

	// Second boot — the same database, no fixture re-applied — must also be
	// a clean no-op end to end, matching every other idempotency test in
	// this package.
	require.NoError(t, migrate(db), "second migrate() run must be idempotent")
}
