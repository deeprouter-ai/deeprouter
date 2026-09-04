package model

import "gorm.io/gorm"

// Migrate creates all Skill Marketplace V2 tables in dependency order.
// Uses raw DDL (PostgreSQL-only; TEXT[], JSONB, partial indexes).
// Called once from model/main.go during application startup.
// Prerequisite: run scripts/migrations/2026-09-01-drop-v1-skill-tables.sql on production PG first.
func Migrate(db *gorm.DB) error {
	// Step 1: skills table (active_version_id FK added later â€” circular reference)
	if err := migrateSkills(db); err != nil {
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
	return migrateSkillAdminLogs(db)
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
	// PostgreSQL's extended query protocol (what GORM's pgx driver uses for
	// Exec) rejects multiple commands in one prepared statement
	// (SQLSTATE 42601) — each CREATE INDEX needs its own Exec call.
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_skills_status ON skills(status)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_skills_featured ON skills(featured_flag, featured_rank) WHERE status = 'published'`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_skills_created ON skills(created_at DESC) WHERE status = 'published'`).Error
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
func addActiveVersionFK(db *gorm.DB) error {
	return db.Exec(`
		DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_constraint WHERE conname = 'fk_skills_active_version'
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
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_sp_user ON skill_purchases(user_id)`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_sp_skill ON skill_purchases(skill_id)`).Error
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
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_sal_skill ON skill_admin_logs(skill_id)`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_sal_admin ON skill_admin_logs(admin_id)`).Error
}
