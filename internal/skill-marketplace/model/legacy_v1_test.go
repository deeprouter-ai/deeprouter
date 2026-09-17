package model

// Tests for the V1 quarantine and the post-migration schema assertions.
//
// These are an internal (package model) test file rather than model_test,
// because the behaviour worth pinning down lives in unexported functions:
// Migrate is fail-soft by design and always returns nil, so calling it can no
// longer tell a test whether the migration actually worked. migrate() — the
// strict version — can.
//
// Everything here is gated on TEST_POSTGRES_DSN, same convention as
// migrate_test.go. The whole subject is PostgreSQL catalog behaviour (global
// index names, constraints following a rename, sequence ownership), none of
// which SQLite models, so a SQLite run of these would prove nothing.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// legacyV1FixtureDDL recreates the leftover V1 schema as production carries it.
//
// It is not pristine V1. Six days of crash-looping had V2's own migration
// adding idx_skills_status, idx_skills_created and fk_skills_active_version to
// V1's table before dying at skill_purchases, so the real leftover is a hybrid
// — and reproducing that hybrid is the point: a quarantine written against a
// list of "V1 names" would sail past those three and leave the names taken.
//
// user_enabled_skills is given a BIGSERIAL id, which real V1 did not have (its
// primary key is composite). It is here to exercise sequence renaming, because
// production's actual shape cannot be inspected from here and a quarantine that
// silently skips sequences leaves that database permanently unlike every other.
const legacyV1FixtureDDL = `
CREATE TABLE skills (
    id                char(36)     NOT NULL PRIMARY KEY,
    slug              varchar(128) NOT NULL UNIQUE,
    status            varchar(32)  NOT NULL DEFAULT 'draft',
    category          varchar(64)  NOT NULL,
    tags              text         NOT NULL,
    name              varchar(160) NOT NULL,
    short_description varchar(280) NOT NULL,
    description       text         NOT NULL,
    required_plan     varchar(32)  NOT NULL,
    monetization_type varchar(32)  NOT NULL,
    model_whitelist   text         NOT NULL,
    featured_flag     boolean      NOT NULL DEFAULT false,
    featured_rank     integer,
    active_version_id char(36),
    created_by        bigint       NOT NULL,
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_skills_status CHECK (status IN ('draft','published','deprecated','archived')),
    CONSTRAINT chk_skills_required_plan CHECK (required_plan IN ('free','pro','enterprise'))
);
CREATE INDEX idx_skills_featured ON skills(featured_flag, featured_rank);
CREATE INDEX idx_skills_status_category ON skills(status, category);

CREATE TABLE skill_versions (
    id             char(36)    NOT NULL PRIMARY KEY,
    skill_id       char(36)    NOT NULL,
    version_number integer     NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_skill_versions_skill FOREIGN KEY (skill_id) REFERENCES skills(id)
);

CREATE TABLE user_enabled_skills (
    id         bigserial   NOT NULL PRIMARY KEY,
    user_id    bigint      NOT NULL,
    skill_id   char(36)    NOT NULL,
    enabled_at timestamptz NOT NULL DEFAULT now()
);

-- The six V1 tables whose names V2 does not reuse. Stubs are enough: what is
-- being tested is that they get quarantined anyway, because they are exactly
-- the names a future marketplace feature would reach for.
CREATE TABLE skill_audit_log             (id char(36) NOT NULL PRIMARY KEY);
CREATE TABLE skill_purchase_orders       (id char(36) NOT NULL PRIMARY KEY);
CREATE TABLE skill_entitlements          (id char(36) NOT NULL PRIMARY KEY);
CREATE TABLE skill_telemetry_quarantines (id char(36) NOT NULL PRIMARY KEY);
CREATE TABLE skill_usage_events          (event_id char(36) NOT NULL PRIMARY KEY);
CREATE TABLE user_saved_skills           (id char(36) NOT NULL PRIMARY KEY);

INSERT INTO skills (id, slug, status, category, tags, name, short_description,
                    description, required_plan, monetization_type, model_whitelist, created_by)
VALUES ('11111111-2222-3333-4444-555555555555', 'legacy-v1-skill', 'published', 'writing',
        '["legacy"]', 'Legacy V1 Skill', 'from before the rewrite',
        'a row that must survive the quarantine', 'free', 'token_markup', '[]', 1);
`

// crashLoopDriftDDL is what V2's migration managed to apply to V1's table over
// six days of restarts before failing: two indexes whose names V1 never used,
// and the circular foreign key, which succeeded because V1's own columns are
// consistently CHAR(36).
const crashLoopDriftDDL = `
CREATE INDEX IF NOT EXISTS idx_skills_status ON skills(status);
CREATE INDEX IF NOT EXISTS idx_skills_created ON skills(created_at DESC);
ALTER TABLE skills ADD CONSTRAINT fk_skills_active_version
  FOREIGN KEY (active_version_id) REFERENCES skill_versions(id);
`

func TestQuarantineLegacyV1Schema_MovesV1AsideAndBuildsV2(t *testing.T) {
	db := newLegacyV1Database(t)

	require.NoError(t, migrate(db), "migrate must succeed against a V1 leftover database")

	t.Run("V2 tables exist with V2 shapes", func(t *testing.T) {
		for _, table := range expectedTables {
			require.True(t, db.Migrator().HasTable(table), "table %q should exist", table)
		}
		require.Equal(t, "bigint", columnTypeOf(t, db, "skills", "id"),
			"skills.id must be V2's bigint, not V1's char(36)")
		require.Equal(t, "ARRAY", columnTypeOf(t, db, "skills", "tags"),
			"skills.tags must be V2's text[], not V1's text")
	})

	t.Run("the V1 table is preserved under the backup prefix", func(t *testing.T) {
		require.True(t, db.Migrator().HasTable("_v1_bak_skills"))
		var slug string
		require.NoError(t, db.Raw(`SELECT slug FROM _v1_bak_skills`).Scan(&slug).Error)
		require.Equal(t, "legacy-v1-skill", slug, "V1 rows must survive; the rename is the whole safety net")
	})

	t.Run("every V1 table is quarantined, not just the three that collide", func(t *testing.T) {
		// The six non-colliding ones are the names a future marketplace
		// feature would reach for. Leaving them would reproduce this incident
		// later, with nobody left who remembers V1.
		for _, table := range legacyV1Tables {
			require.True(t, db.Migrator().HasTable(legacyV1Prefix+table),
				"%q should have been renamed to %q", table, legacyV1Prefix+table)
		}
	})

	t.Run("the foreign key lands on the V2 table, not the backup", func(t *testing.T) {
		// Constraint names follow a renamed table, and addActiveVersionFK used
		// to check the name without scoping to a relation — so V1's copy on
		// _v1_bak_skills would have satisfied that check forever.
		on, err := constraintOnTable(db, "skills", "fk_skills_active_version")
		require.NoError(t, err)
		require.True(t, on, "fk_skills_active_version must be on skills")
	})

	t.Run("the featured index lands on the V2 table with V2's definition", func(t *testing.T) {
		// Index names are global, so V1's idx_skills_featured would otherwise
		// make V2's CREATE INDEX IF NOT EXISTS a silent no-op.
		on, err := indexOnTable(db, "skills", "idx_skills_featured")
		require.NoError(t, err)
		require.True(t, on, "idx_skills_featured must be on skills")

		var def string
		require.NoError(t, db.Raw(`
			SELECT indexdef FROM pg_indexes
			WHERE schemaname = current_schema() AND indexname = 'idx_skills_featured'
		`).Scan(&def).Error)
		require.Contains(t, def, "published",
			"the index on skills must be V2's partial index, not V1's unfiltered one")
	})

	t.Run("the quarantined database is actually writable, not just correctly shaped", func(t *testing.T) {
		// Matching pg_dump byte for byte says the schema is right; it does not
		// say a row can go in. Everything this incident touched shows up on the
		// write path — the bigserial id that V1's char(36) was shadowing, and
		// tags, which was its own production bug when it came back as TEXT
		// instead of TEXT[]. So actually write one.
		require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)

		skill := &Skill{
			Slug:             "post-quarantine-skill",
			Name:             "Post Quarantine Skill",
			Description:      "written on a database that used to hold V1",
			Category:         "code",
			Tags:             []string{"alpha", "beta"},
			Status:           "draft",
			MonetizationType: "free",
			CreatedBy:        1,
		}
		require.NoError(t, db.Create(skill).Error, "a skill must be insertable after quarantine")
		require.NotZero(t, skill.ID, "the bigserial id must be assigned by the sequence")

		var got Skill
		require.NoError(t, db.First(&got, skill.ID).Error)
		require.Equal(t, []string{"alpha", "beta"}, []string(got.Tags),
			"tags must round-trip through the text[] column")
	})

	t.Run("V2 keeps the canonical sequence name", func(t *testing.T) {
		// A sequence left behind under the old name pushes V2's onto a "_seq1"
		// suffix, which is how production ends up permanently unlike every
		// other environment. Found by diffing pg_dump, not by reading code.
		var def string
		require.NoError(t, db.Raw(`
			SELECT column_default FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'user_enabled_skills' AND column_name = 'id'
		`).Scan(&def).Error)
		require.Contains(t, def, "user_enabled_skills_id_seq'",
			"V2 must own the unsuffixed sequence name; got %q", def)
	})
}

func TestQuarantineLegacyV1Schema_RenamesConstraintsAndIndexesNotJustTables(t *testing.T) {
	// The three object classes that survive ALTER TABLE ... RENAME. Asserting
	// the table moved says nothing about them, and each one left behind breaks
	// V2 in its own silent way.
	db := newLegacyV1Database(t)
	require.NoError(t, migrate(db))

	t.Run("V1 constraints carry the backup prefix", func(t *testing.T) {
		for _, name := range []string{"chk_skills_status", "chk_skills_required_plan"} {
			on, err := constraintOnTable(db, "_v1_bak_skills", legacyV1Prefix+name)
			require.NoError(t, err)
			require.True(t, on, "%q should have been renamed on the backup table", name)

			// And the original name must be free, or a future V2 constraint of
			// that name would collide with a table nobody is looking at.
			var stillTaken int64
			require.NoError(t, db.Raw(
				`SELECT count(*) FROM pg_constraint WHERE conname = ?`, name).Scan(&stillTaken).Error)
			require.Zero(t, stillTaken, "the name %q must be free after quarantine", name)
		}
	})

	t.Run("V1 indexes carry the backup prefix", func(t *testing.T) {
		on, err := indexOnTable(db, "_v1_bak_skills", legacyV1Prefix+"idx_skills_status_category")
		require.NoError(t, err)
		require.True(t, on, "a V1-only index should have been renamed onto the backup table")
	})

	t.Run("the V1 sequence carries the backup prefix", func(t *testing.T) {
		exists, err := relationExists(db, legacyV1Prefix+"user_enabled_skills_id_seq")
		require.NoError(t, err)
		require.True(t, exists, "the V1 sequence should have been renamed, not left holding the name")
	})
}

func TestAddActiveVersionFK_IsScopedToTheSkillsTable(t *testing.T) {
	// The regression this pins down: constraint names are unique per table, not
	// per schema, so `WHERE conname = 'fk_skills_active_version'` alone treats a
	// constraint sitting on some unrelated relation as proof that skills has
	// one. Quarantining V1 puts exactly such a constraint on _v1_bak_skills, so
	// without the conrelid scope the skills table silently never gets its
	// foreign key back — and nothing else in the system would ever say so.
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)

	// A decoy carrying the name, on a table that is not skills.
	require.NoError(t, db.Exec(`CREATE TABLE decoy_parent (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE decoy (id BIGSERIAL PRIMARY KEY, parent_id BIGINT)`).Error)
	require.NoError(t, db.Exec(`
		ALTER TABLE decoy ADD CONSTRAINT fk_skills_active_version
		FOREIGN KEY (parent_id) REFERENCES decoy_parent(id)`).Error)

	require.NoError(t, migrate(db))

	on, err := constraintOnTable(db, "skills", "fk_skills_active_version")
	require.NoError(t, err)
	require.True(t, on,
		"skills must get its own foreign key even when the name is taken elsewhere")
}

func TestQuarantineLegacyV1Schema_SurvivesAnAlreadyTakenBackupName(t *testing.T) {
	// Someone may already have run scripts/migrations/2026-09-01-drop-v1-skill-
	// tables.sql by hand on some environment, or a previous attempt may have
	// gone half way. Neither may turn into a hard failure here.
	db := newLegacyV1Database(t)
	require.NoError(t, db.Exec(`CREATE TABLE _v1_bak_skills (id bigint)`).Error)

	require.NoError(t, migrate(db), "a taken backup name must not fail the migration")

	// The pre-existing table keeps the plain name; V1's goes to the next free one.
	exists, err := relationExists(db, "_v1_bak_skills_2")
	require.NoError(t, err)
	require.True(t, exists, "V1's table should have landed on the next free name")

	var slug string
	require.NoError(t, db.Raw(`SELECT slug FROM _v1_bak_skills_2`).Scan(&slug).Error)
	require.Equal(t, "legacy-v1-skill", slug, "and it must be the real V1 table that moved there")

	require.Equal(t, "bigint", columnTypeOf(t, db, "skills", "id"), "V2 still gets built")
}

func TestQuarantineLegacyV1Schema_RollsBackEntirelyOnFailure(t *testing.T) {
	// The quarantine renames nine tables. A failure partway through must not
	// leave a database where some names have moved and some have not — that
	// state is worse than either end of it, and nobody could diagnose it
	// without the access this team does not have.
	//
	// The failure is forced by exhausting freeRelationName's suffixes for the
	// LAST table in the list, so the eight before it have already been renamed
	// inside the transaction when the error surfaces.
	db := newLegacyV1Database(t)
	last := legacyV1Tables[len(legacyV1Tables)-1]
	require.NoError(t, db.Exec(fmt.Sprintf(`
		DO $$ BEGIN
		  FOR i IN 1..50 LOOP
		    EXECUTE format('CREATE TABLE %%I (id bigint)',
		      CASE WHEN i = 1 THEN '%s%s' ELSE '%s%s_' || i END);
		  END LOOP;
		END $$`, legacyV1Prefix, last, legacyV1Prefix, last)).Error)

	require.Error(t, migrate(db), "the migration must report the failure")

	// Nothing moved: skills is still V1's, under its original name.
	require.Equal(t, "character", columnTypeOf(t, db, "skills", "id"),
		"a failed quarantine must leave every table where it was")
	exists, err := relationExists(db, legacyV1Prefix+"skill_versions")
	require.NoError(t, err)
	require.False(t, exists,
		"no table may stay renamed when a later one in the same transaction failed")
}

func TestAssertSchema_NoOpOnNonPostgres(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, assertSchema(db),
		"the assertions read PostgreSQL catalogs and must not fire on other dialects")
}

func TestQuoteIdent(t *testing.T) {
	// Everything reaching quoteIdent is a constant or a catalog-read name, but
	// the DDL above cannot use bind parameters for identifiers, so the escaping
	// is the only thing standing between a catalog name and the statement text.
	require.Equal(t, `"skills"`, quoteIdent("skills"))
	require.Equal(t, `"_v1_bak_skills"`, quoteIdent("_v1_bak_skills"))
	require.Equal(t, `"weird""name"`, quoteIdent(`weird"name`),
		"an embedded double quote must be doubled, not dropped")
}

func TestQuarantineLegacyV1Schema_NoOpOnCleanDatabase(t *testing.T) {
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)

	require.NoError(t, migrate(db))

	var backups int64
	require.NoError(t, db.Raw(`
		SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname LIKE '\_v1\_bak\_%'
	`).Scan(&backups).Error)
	require.Zero(t, backups, "a database that never saw V1 must come out untouched")
}

func TestQuarantineLegacyV1Schema_IsIdempotent(t *testing.T) {
	db := newLegacyV1Database(t)

	require.NoError(t, migrate(db), "first run")
	before := relationNames(t, db)

	// Every boot runs this again. The second pass must find nothing to do —
	// after the first, skills is V2's and its id is a bigint, so the detection
	// can no longer match.
	require.NoError(t, migrate(db), "second run")
	require.Equal(t, before, relationNames(t, db), "a second run must not move anything")
}

func TestQuarantineLegacyV1Schema_LeavesAV2TableAloneEvenWithAV1LookingColumn(t *testing.T) {
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, migrate(db), "build a real V2 schema first")

	// A later PR is perfectly entitled to add a column with this name. If the
	// detection rested on the V1-only column alone, this is the moment it would
	// rename a live V2 table — full of real skills — out of existence.
	require.NoError(t, db.Exec(`ALTER TABLE skills ADD COLUMN required_plan varchar(32)`).Error)

	require.NoError(t, migrate(db))

	require.False(t, db.Migrator().HasTable("_v1_bak_skills"),
		"the V1-only column alone must never be enough to quarantine a table")
	require.Equal(t, "bigint", columnTypeOf(t, db, "skills", "id"),
		"the live V2 table must still be in place")
}

func TestQuarantineLegacyV1Schema_NeedsBothSignalsNotJustTheIdType(t *testing.T) {
	// The mirror of the required_plan test above, and the half that was missing:
	// a character-typed id on its own must not be enough either. Without this,
	// deleting the second signal from legacyV1SchemaPresent would not turn a
	// single test red, and a check described as two-signal would quietly be one.
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)

	// Some other project's skills table: char id, but nothing of V1 about it.
	require.NoError(t, db.Exec(`CREATE TABLE skills (id char(36) PRIMARY KEY, note text)`).Error)

	// The migration cannot build V2 on top of a stranger's table and says so —
	// CREATE TABLE IF NOT EXISTS accepts it, then the first index on a column
	// that is not there fails. Erroring out is the right end to this: the
	// alternative is quietly renaming away a table this code does not
	// understand.
	require.Error(t, migrate(db))

	require.False(t, db.Migrator().HasTable("_v1_bak_skills"),
		"a character id alone must not be read as V1")
	require.Equal(t, "character", columnTypeOf(t, db, "skills", "id"),
		"the unrecognised table must be left exactly as it was")
	var note int64
	require.NoError(t, db.Raw(`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'skills' AND column_name = 'note'
	`).Scan(&note).Error)
	require.EqualValues(t, 1, note, "and its own columns must still be there")
}

func TestQuarantineLegacyV1Schema_RecoversFromTheHandRunScript(t *testing.T) {
	// The state left by someone running
	// scripts/migrations/2026-09-01-drop-v1-skill-tables.sql by hand, which is
	// what that script was written for and so a likely state rather than an
	// exotic one. It renames tables and nothing else, and it has no
	// transaction — it aborts on the first table that does not exist, which is
	// how it behaved when this incident was being reproduced.
	//
	// Detection then sees no V1 skills table and correctly does nothing, so
	// without freeNamesHeldByBackupTables the V2 tables get built while
	// _v1_bak_skills still owns idx_skills_featured, idx_skills_status,
	// idx_skills_created and fk_skills_active_version — and every one of those
	// CREATE ... IF NOT EXISTS statements skips in silence.
	db := newLegacyV1Database(t)

	// Tables only, exactly as the script does it, and stopping partway as it
	// does when a table is missing.
	for _, table := range []string{"skills", "skill_versions", "user_enabled_skills"} {
		require.NoError(t, db.Exec(fmt.Sprintf(
			`ALTER TABLE %q RENAME TO %q`, table, legacyV1Prefix+table)).Error)
	}

	require.NoError(t, migrate(db), "the migration must recover from a half-done manual cleanup")

	require.Equal(t, "bigint", columnTypeOf(t, db, "skills", "id"))

	for _, index := range expectedIndexes["skills"] {
		on, err := indexOnTable(db, "skills", index)
		require.NoError(t, err)
		require.True(t, on, "%s must end up on the V2 table, not stranded on the backup", index)
	}
	on, err := constraintOnTable(db, "skills", "fk_skills_active_version")
	require.NoError(t, err)
	require.True(t, on, "the foreign key must end up on the V2 table")

	// The tables the script never reached are knowingly left alone: treating
	// any table with a V1 name as residue would rename away a future V2
	// skill_usage_events full of real rows. A future feature reaching for one
	// of those names collides loudly at its own CREATE, which is diagnosable.
	stillThere, err := relationExists(db, "skill_audit_log")
	require.NoError(t, err)
	require.True(t, stillThere)
}

func TestQuarantineLegacyV1Schema_NoOpOnNonPostgres(t *testing.T) {
	// Guard against a future contributor dropping the dialect check: this DDL
	// is PostgreSQL catalog surgery and must never fire on SQLite or MySQL.
	// Nothing here needs a server, so it runs on every CI machine.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, quarantineLegacyV1Schema(db),
		"quarantine must be a no-op on a non-postgres dialect")
}

func TestMigrate_IsFailSoftButMigrateIsNot(t *testing.T) {
	// No users table, so migrateSkills' FK to users(id) cannot be satisfied and
	// the migration genuinely fails.
	db := newPostgresDatabase(t)

	require.Error(t, migrate(db),
		"the strict migration must report the failure")
	require.NoError(t, Migrate(db),
		"the public entry point must swallow it: a marketplace migration failure "+
			"took the whole gateway offline for six days in September 2026")
}

func TestAssertSchema_NamesWhatIsWrong(t *testing.T) {
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, migrate(db))
	require.NoError(t, assertSchema(db), "a freshly migrated schema must pass")

	t.Run("missing table", func(t *testing.T) {
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`DROP TABLE skill_admin_logs`).Error)
		require.ErrorContains(t, assertSchema(db), "skill_admin_logs")
	})

	t.Run("missing column", func(t *testing.T) {
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`ALTER TABLE skills DROP COLUMN tags`).Error)
		require.ErrorContains(t, assertSchema(db), "skills.tags")
	})

	t.Run("wrong column type", func(t *testing.T) {
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`ALTER TABLE skills DROP COLUMN tags`).Error)
		require.NoError(t, db.Exec(`ALTER TABLE skills ADD COLUMN tags text`).Error)
		err := assertSchema(db)
		require.ErrorContains(t, err, "skills.tags")
		require.ErrorContains(t, err, "want ARRAY")
	})

	t.Run("missing index", func(t *testing.T) {
		// The failure with no external symptom whatsoever — queries still
		// return correct rows, just slower. Nothing but this assertion would
		// ever notice.
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`DROP INDEX idx_skills_featured`).Error)
		require.ErrorContains(t, assertSchema(db), "idx_skills_featured")
	})

	t.Run("missing constraint", func(t *testing.T) {
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`ALTER TABLE skills DROP CONSTRAINT fk_skills_active_version`).Error)
		require.ErrorContains(t, assertSchema(db), "fk_skills_active_version")
	})

	t.Run("every problem is reported, not just the first", func(t *testing.T) {
		// Whoever reads this in a log is unlikely to get a second run out of
		// the database, so one message has to carry the whole picture.
		db := newMigratedDatabase(t)
		require.NoError(t, db.Exec(`DROP INDEX idx_skills_featured`).Error)
		require.NoError(t, db.Exec(`ALTER TABLE skills DROP CONSTRAINT fk_skills_active_version`).Error)
		err := assertSchema(db)
		require.ErrorContains(t, err, "idx_skills_featured")
		require.ErrorContains(t, err, "fk_skills_active_version")
	})
}

// --- fixtures ---------------------------------------------------------------

func newPostgresDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the V1 quarantine tests")
	}

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	require.NoError(t, err, "open admin DSN")
	adminSQLDB, err := adminDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminSQLDB.Close() })

	name := fmt.Sprintf("skill_marketplace_v1_quarantine_%d", time.Now().UnixNano())
	require.NoError(t, adminDB.Exec(fmt.Sprintf(`CREATE DATABASE %q`, name)).Error)
	t.Cleanup(func() {
		adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})

	// PrepareStmt matches model/main.go's real connection config: it forces the
	// extended query protocol, which is what rejects multi-command Execs. A
	// migration test that does not set it can pass while production breaks.
	db, err := gorm.Open(postgres.Open(swapDatabaseName(adminDSN, name)), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

// newLegacyV1Database is a scratch database carrying the leftover V1 schema
// plus the drift six days of crash-looping added to it.
func newLegacyV1Database(t *testing.T) *gorm.DB {
	t.Helper()
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	for _, stmt := range splitStatements(legacyV1FixtureDDL) {
		require.NoError(t, db.Exec(stmt).Error, "V1 fixture: %s", stmt)
	}
	for _, stmt := range splitStatements(crashLoopDriftDDL) {
		require.NoError(t, db.Exec(stmt).Error, "crash-loop drift: %s", stmt)
	}
	return db
}

func newMigratedDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db := newPostgresDatabase(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)
	require.NoError(t, migrate(db))
	return db
}

func columnTypeOf(t *testing.T, db *gorm.DB, table, column string) string {
	t.Helper()
	got, err := columnType(db, table, column)
	require.NoError(t, err)
	return got
}

func relationNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw(`
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relkind IN ('r','i','S')
		ORDER BY c.relname
	`).Scan(&names).Error)
	return names
}

// splitStatements exists because PrepareStmt forces the extended query
// protocol, which refuses more than one command per Exec — the same SQLSTATE
// 42601 that broke production once already.
func splitStatements(ddl string) []string {
	var out []string
	for _, stmt := range strings.Split(ddl, ";") {
		if trimmed := strings.TrimSpace(stmt); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func swapDatabaseName(dsn, name string) string {
	idx := strings.LastIndex(dsn, "/")
	if idx == -1 {
		return dsn
	}
	base, query := dsn[:idx], ""
	if q := strings.IndexByte(dsn[idx:], '?'); q != -1 {
		query = dsn[idx:][q:]
	}
	return base + "/" + name + query
}
