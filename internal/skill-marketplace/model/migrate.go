package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Migrate creates all Skill Marketplace V2 tables in dependency order.
// Uses raw DDL (PostgreSQL-only; TEXT[], JSONB, partial indexes).
// Called once from model/main.go during application startup.
//
// 🔴 Never returns an error, by design. See migrateOrReport.
func Migrate(db *gorm.DB) error {
	return migrateOrReport(db, migrate)
}

// migrateOrReport downgrades a marketplace migration failure from "the gateway
// does not start" to "the marketplace is broken and says so in the log".
//
// The marketplace is the last step of model.migrateDB, so by the time this runs
// every other migration has already succeeded and nothing downstream of it
// depends on the result. Yet a failure here used to reach model/main.go's
// FatalLog and kill the process — which, under `restart: unless-stopped`, is a
// crash loop, and behind Caddy a permanent 502. That is how a brand new,
// entirely optional feature took the whole LLM gateway offline for six days in
// September 2026 (see legacy_v1.go for the mechanism).
//
// Whether a marketplace migration failure should be fatal is the marketplace's
// own call, which is why the decision lives here and not at the call site in
// model/main.go — that file stays untouched.
//
// ⚠️ The cost is real: from here on, a marketplace schema failure is silent.
// There is no deploy health check and no log access, so the only external
// symptom is the public /api/skills handler returning the underlying Postgres
// error, and only for failures that actually break a query. A missing index or
// foreign key produces no symptom at all. Accepted deliberately on 2026-09-08;
// schema_assert.go carries the longer note.
func migrateOrReport(db *gorm.DB, run func(*gorm.DB) error) error {
	if err := run(db); err != nil {
		// Written out at length because of where it lands: whoever finds this
		// line is looking at a gateway that starts fine and a marketplace that
		// does not work, with no other clue pointing here.
		common.SysError("SKILL MARKETPLACE MIGRATION FAILED — the gateway is " +
			"running but marketplace tables are missing or wrong, so /api/skills " +
			"and the admin skill pages will error. This is not fatal by design " +
			"(internal/skill-marketplace/model/migrate.go, migrateOrReport). " +
			"Cause: " + err.Error())
	}
	return nil
}

func migrate(db *gorm.DB) error {
	// Step 0: move any leftover V1 tables out of the way, so the CREATE TABLE
	// IF NOT EXISTS statements below build V2's schema instead of silently
	// accepting V1's. No-op on every database that has never seen V1.
	if err := quarantineLegacyV1Schema(db); err != nil {
		return err
	}
	// Step 1: skills table (active_version_id FK added later â€” circular reference)
	if err := migrateSkills(db); err != nil {
		return err
	}
	// Step 1b: PRD §13 reference-listing columns, added to the now-existing
	// skills table before anything downstream reads it.
	if err := addReferenceListingColumns(db); err != nil {
		return err
	}
	// Step 2: skill_versions (references skills)
	if err := migrateSkillVersions(db); err != nil {
		return err
	}
	// Step 3: add circular FK now that both tables exist
	if err := addActiveVersionFK(db); err != nil {
		return err
	}
	// Step 4: remaining tables
	if err := migrateUserEnabledSkills(db); err != nil {
		return err
	}
	if err := migrateSkillPurchases(db); err != nil {
		return err
	}
	if err := migrateSkillAdminLogs(db); err != nil {
		return err
	}
	// Step 5: confirm the database actually holds what the steps above built.
	// IF NOT EXISTS accepts a wrong-shaped table as an existing one, so without
	// this the migration can report success over a schema it never created.
	return assertSchema(db)
}

func migrateSkills(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS skills (
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
		  CONSTRAINT skills_status_check
		    CHECK (status IN ('draft', 'published', 'deprecated')),
		  CONSTRAINT skills_monetization_check
		    CHECK (monetization_type IN ('free', 'paid')),
		  CONSTRAINT skills_price_check
		    CHECK (monetization_type = 'free' OR price_usd > 0)
		)
	`).Error; err != nil {
		return err
	}
	// One statement per Exec: gorm's pgx connection sends Exec through the
	// extended protocol, and PG rejects multiple commands in one prepared
	// statement (SQLSTATE 42601) — batching these crashed the gateway on
	// boot (found 2026-09-04, took production down with it).
	return execEach(db,
		`CREATE INDEX IF NOT EXISTS idx_skills_status   ON skills(status)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_featured ON skills(featured_flag, featured_rank) WHERE status = 'published'`,
		`CREATE INDEX IF NOT EXISTS idx_skills_created  ON skills(created_at DESC)              WHERE status = 'published'`,
	)
}

// addReferenceListingColumns adds the PRD §13 reference-listing columns
// (listing_type, source_url) and their three CHECK constraints to skills.
// ADD COLUMN IF NOT EXISTS and the pg_constraint guards (same pattern as
// addActiveVersionFK below) make every statement safe to run on every
// startup, whether the table is brand new or already has these columns.
func addReferenceListingColumns(db *gorm.DB) error {
	if err := execEach(db,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS listing_type VARCHAR(10) NOT NULL DEFAULT 'hosted'`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS source_url VARCHAR(500)`,
	); err != nil {
		return err
	}
	return execEach(db,
		`DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_constraint
		    WHERE conname = 'skills_listing_type_check'
		      AND conrelid = to_regclass('skills')
		  ) THEN
		    ALTER TABLE skills ADD CONSTRAINT skills_listing_type_check
		      CHECK (listing_type IN ('hosted', 'reference'));
		  END IF;
		END$$`,
		`DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_constraint
		    WHERE conname = 'skills_reference_source_check'
		      AND conrelid = to_regclass('skills')
		  ) THEN
		    ALTER TABLE skills ADD CONSTRAINT skills_reference_source_check
		      CHECK (listing_type = 'hosted' OR source_url IS NOT NULL);
		  END IF;
		END$$`,
		`DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_constraint
		    WHERE conname = 'skills_reference_free_check'
		      AND conrelid = to_regclass('skills')
		  ) THEN
		    ALTER TABLE skills ADD CONSTRAINT skills_reference_free_check
		      CHECK (listing_type = 'hosted' OR monetization_type = 'free');
		  END IF;
		END$$`,
	)
}

// execEach runs each DDL statement as its own Exec — see the SQLSTATE 42601
// note above.
func execEach(db *gorm.DB, stmts ...string) error {
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateSkillVersions(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_versions (
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
		  CONSTRAINT skill_versions_status_check
		    CHECK (status IN ('draft', 'active', 'archived')),
		  CONSTRAINT skill_versions_semver_check
		    CHECK (version ~ '^\d+\.\d+\.\d+$')
		)
	`).Error; err != nil {
		return err
	}
	return nil
}

// addActiveVersionFK resolves the circular reference between skills and skill_versions.
// skills.active_version_id â†’ skill_versions(id) can only be added after both tables exist.
//
// The existence check is scoped to the skills table with conrelid. Constraint
// names are unique per table, not per schema, and they follow a table through a
// rename — so an unscoped `WHERE conname = ...` treats a constraint sitting on
// some other relation as proof that this one exists. That is not hypothetical:
// quarantining V1 moves its copy of this exact name onto _v1_bak_skills, and
// without conrelid here the skills table would never get its foreign key again.
func addActiveVersionFK(db *gorm.DB) error {
	return db.Exec(`
		DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_constraint
		    WHERE conname = 'fk_skills_active_version'
		      AND conrelid = to_regclass('skills')
		  ) THEN
		    ALTER TABLE skills
		      ADD CONSTRAINT fk_skills_active_version
		      FOREIGN KEY (active_version_id) REFERENCES skill_versions(id);
		  END IF;
		END$$;
	`).Error
}

func migrateUserEnabledSkills(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_enabled_skills (
		  id         BIGSERIAL PRIMARY KEY,
		  user_id    BIGINT NOT NULL REFERENCES users(id),
		  skill_id   BIGINT NOT NULL REFERENCES skills(id),
		  version_id BIGINT NOT NULL REFERENCES skill_versions(id),
		  enabled_at TIMESTAMP NOT NULL DEFAULT NOW(),
		  UNIQUE (user_id, skill_id)
		)
	`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_ues_user ON user_enabled_skills(user_id)`).Error
}

func migrateSkillPurchases(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_purchases (
		  id             BIGSERIAL PRIMARY KEY,
		  user_id        BIGINT NOT NULL REFERENCES users(id),
		  skill_id       BIGINT NOT NULL REFERENCES skills(id),
		  price_usd      NUMERIC(10,2) NOT NULL,
		  quota_deducted BIGINT NOT NULL,
		  purchased_at   TIMESTAMP NOT NULL DEFAULT NOW(),
		  UNIQUE (user_id, skill_id)
		)
	`).Error; err != nil {
		return err
	}
	return execEach(db,
		`CREATE INDEX IF NOT EXISTS idx_sp_user  ON skill_purchases(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sp_skill ON skill_purchases(skill_id)`,
	)
}

func migrateSkillAdminLogs(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_admin_logs (
		  id         BIGSERIAL PRIMARY KEY,
		  admin_id   BIGINT NOT NULL REFERENCES users(id),
		  skill_id   BIGINT REFERENCES skills(id) ON DELETE SET NULL,
		  action     VARCHAR(50) NOT NULL,
		  details    JSONB DEFAULT '{}',
		  created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		return err
	}
	return execEach(db,
		`CREATE INDEX IF NOT EXISTS idx_sal_skill  ON skill_admin_logs(skill_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sal_admin  ON skill_admin_logs(admin_id)`,
	)
}
