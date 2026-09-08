package model

import (
	"fmt"

	"gorm.io/gorm"
)

// Quarantine of the Skill Marketplace V1 schema.
//
// Why this exists
//
// V1 lived in internal/skill/ and was removed in PR #159. That PR deliberately
// left its nine tables in the database — dropping tables from a code-removal PR
// would have been irreversible — and handed the cleanup to a hand-run script,
// scripts/migrations/2026-09-01-drop-v1-skill-tables.sql, to be executed on
// production before the V2 P1 deploy.
//
// The script was never run. V2 reuses three of V1's table names with an
// incompatible shape (V1 skills.id is CHAR(36); V2's is BIGSERIAL), and every
// CREATE TABLE in migrate.go is guarded by IF NOT EXISTS — so the colliding
// tables were silently skipped, the V2 code kept believing skills.id was a
// bigint, and the first table V1 never owned (skill_purchases) failed to build
// its foreign key:
//
//	ERROR: foreign key constraint "skill_purchases_skill_id_fkey"
//	       cannot be implemented (SQLSTATE 42804)
//
// migrateDB returned that error, main.go called FatalLog, the process exited,
// docker restarted it under `restart: unless-stopped`, and the gateway sat
// behind Caddy as a permanent 502 from 2026-09-02 onwards.
//
// Nobody is going to run the script by hand, so it runs here instead.
//
// What it does
//
// Renames — never drops — the nine V1 tables out of the way, together with
// every index and constraint they own, so the names are free for V2 and the V1
// rows stay recoverable under the _v1_bak_ prefix.
//
// Renaming the tables alone is not enough, and this is the subtle part. Three
// kinds of object keep their names when a table is renamed, and each one of
// them fails differently:
//
//   - Indexes. Index names are global in PostgreSQL — they share a namespace
//     with tables. V1 and V2 both define idx_skills_featured, so leaving V1's
//     behind makes V2's `CREATE INDEX IF NOT EXISTS idx_skills_featured`
//     silently skip and ship a table without that index.
//   - Constraints. Constraint names travel with the renamed table, and
//     addActiveVersionFK checks `SELECT 1 FROM pg_constraint WHERE conname =
//     'fk_skills_active_version'` without scoping to a table — so V1's
//     constraint sitting on _v1_bak_skills would satisfy that check forever
//     and V2's skills table would never get its foreign key. (That check is
//     scoped properly now; see migrate.go.)
//   - Sequences. A serial column's sequence is not renamed either, so V2's
//     table has to settle for user_enabled_skills_id_seq1 — leaving production
//     with a schema that differs from every other environment forever. This one
//     was found by diffing pg_dump output between a quarantined database and a
//     fresh one; it was the only difference left.
//
// All three failures are silent: the service starts, and the schema is quietly
// wrong. schema_assert.go is the backstop that catches them if they recur.
//
// Nothing here works from a list of known V1 object names, and that is
// deliberate. Six days of crash-looping had V2's own migration adding its
// indexes and its foreign key to V1's table, so production's leftover table is
// a hybrid that no hard-coded list would describe. Whatever the table actually
// owns, as the catalog reports it, is what gets renamed.
//
// This file is single-purpose and self-contained. Once every environment has
// been quarantined it can be deleted wholesale.

const legacyV1Prefix = "_v1_bak_"

// legacyV1Tables is the same nine tables as
// scripts/migrations/2026-09-01-drop-v1-skill-tables.sql, in the same order.
//
// Only the first three collide with V2 today. The other six are quarantined
// anyway because they are exactly the names a future marketplace feature would
// reach for — a V2 skill_usage_events table would reproduce this whole incident
// months from now, with nobody left who remembers V1.
var legacyV1Tables = []string{
	"skills",
	"skill_versions",
	"user_enabled_skills",
	"skill_audit_log",
	"skill_purchase_orders",
	"skill_entitlements",
	"skill_telemetry_quarantines",
	"skill_usage_events",
	"user_saved_skills",
}

// quarantineLegacyV1Schema moves the V1 tables aside if — and only if — the
// database still carries them. It is a strict no-op on a clean database, on a
// database already quarantined, and on any dialect other than PostgreSQL.
//
// Safe to run on every boot: after the first pass the detection below can no
// longer match, because the skills table is V2's and its id is a bigint.
func quarantineLegacyV1Schema(db *gorm.DB) error {
	// V1 ran on SQLite and MySQL too, but the only deployment that ever had a
	// populated V1 schema is production PostgreSQL. Restricting the blast
	// radius to PG keeps this out of every unit test and local SQLite run.
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	present, err := legacyV1SchemaPresent(db)
	if err != nil {
		return fmt.Errorf("detect legacy V1 schema: %w", err)
	}

	// One transaction for the whole pass: PostgreSQL has transactional DDL, so
	// a failure halfway through leaves the database exactly as it was rather
	// than in a half-renamed state nobody can reason about.
	return db.Transaction(func(tx *gorm.DB) error {
		if present {
			for _, table := range legacyV1Tables {
				exists, err := relationExists(tx, table)
				if err != nil {
					return fmt.Errorf("check table %s: %w", table, err)
				}
				if !exists {
					continue
				}
				if err := quarantineTable(tx, table); err != nil {
					return fmt.Errorf("quarantine %s: %w", table, err)
				}
			}
		}
		// Runs whether or not this pass renamed anything — see the doc comment.
		return freeNamesHeldByBackupTables(tx)
	})
}

// freeNamesHeldByBackupTables makes sure a table already sitting under the
// backup prefix does not still own objects that V2 needs by name.
//
// The case this exists for is someone having run
// scripts/migrations/2026-09-01-drop-v1-skill-tables.sql by hand — which is
// exactly what that script was written for, so it is a likely state, not an
// exotic one. The script renames tables and nothing else. That leaves
// _v1_bak_skills holding idx_skills_featured, idx_skills_status,
// idx_skills_created and fk_skills_active_version under their original names,
// while the detection above sees no V1 skills table and correctly does
// nothing — so V2's CREATE INDEX IF NOT EXISTS statements skip in silence and
// the marketplace comes up missing three indexes and a foreign key, with no
// error anywhere.
//
// Caught by the schema assertions, which is what they are for.
//
// Normalising the invariant — a backup table owns only prefixed objects — is
// idempotent: after the quarantine above, or after a previous run of this, the
// names already carry the prefix and every rename here is skipped.
func freeNamesHeldByBackupTables(tx *gorm.DB) error {
	var tables []string
	if err := tx.Raw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema()
		  AND c.relkind = 'r'
		  AND c.relname LIKE ?
		ORDER BY c.relname
	`, legacyV1Prefix+"%").Scan(&tables).Error; err != nil {
		return fmt.Errorf("list backup tables: %w", err)
	}
	for _, table := range tables {
		if err := renameConstraints(tx, table); err != nil {
			return fmt.Errorf("free constraint names on %s: %w", table, err)
		}
		if err := renameIndexes(tx, table); err != nil {
			return fmt.Errorf("free index names on %s: %w", table, err)
		}
		if err := renameSequences(tx, table); err != nil {
			return fmt.Errorf("free sequence names on %s: %w", table, err)
		}
	}
	return nil
}

// legacyV1SchemaPresent requires two independent signals before anything is
// renamed. One signal is not enough: a later PR could legitimately add a column
// named required_plan to V2's skills, and a single-signal check would then
// rename a live V2 table full of real data out of existence.
//
// The id type is the load-bearing half. A V2 skills table always has a bigint
// primary key; a column can be added to a table, but its primary key does not
// change type. For both signals to hold at once, the table has to actually be
// V1's.
func legacyV1SchemaPresent(db *gorm.DB) (bool, error) {
	idIsCharacter, err := columnExists(db, "skills", "id", "character", "character varying")
	if err != nil {
		return false, err
	}
	if !idIsCharacter {
		return false, nil
	}
	// required_plan is V1-only: V2 has no notion of plan tiers on a skill.
	return columnExists(db, "skills", "required_plan")
}

// columnExists reports whether table.column exists, optionally constrained to a
// set of information_schema data types. An empty types list matches any type.
func columnExists(db *gorm.DB, table, column string, types ...string) (bool, error) {
	query := db.Table("information_schema.columns").
		Where("table_schema = current_schema()").
		Where("table_name = ?", table).
		Where("column_name = ?", column)
	if len(types) > 0 {
		query = query.Where("data_type IN ?", types)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// relationExists covers tables and indexes alike — both live in pg_class, which
// is exactly why an index left behind by a renamed table can block a CREATE
// INDEX for a completely different table.
func relationExists(db *gorm.DB, name string) (bool, error) {
	var count int64
	err := db.Raw(`
		SELECT count(*) FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname = ?
	`, name).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// quarantineTable frees every name the table owns, then the table's own name.
//
// Order matters in two places. Constraints go first because renaming a
// constraint also renames the index backing it (primary keys and unique
// constraints), which keeps the index pass from doing that work twice under a
// name that has already moved. Sequences go before the table rename, while the
// catalog can still find them by the table's current name.
func quarantineTable(tx *gorm.DB, table string) error {
	if err := renameConstraints(tx, table); err != nil {
		return err
	}
	if err := renameIndexes(tx, table); err != nil {
		return err
	}
	if err := renameSequences(tx, table); err != nil {
		return err
	}
	target, err := freeRelationName(tx, legacyV1Prefix+table)
	if err != nil {
		return err
	}
	return tx.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, quoteIdent(table), quoteIdent(target))).Error
}

// renameConstraints prefixes every constraint on the table. Foreign keys and
// CHECK constraints have no index of their own, so ALTER INDEX cannot reach
// them — this pass is the only thing that frees fk_skills_active_version.
func renameConstraints(tx *gorm.DB, table string) error {
	var names []string
	if err := tx.Raw(`
		SELECT conname FROM pg_constraint
		WHERE conrelid = to_regclass(?)
		ORDER BY conname
	`, table).Scan(&names).Error; err != nil {
		return err
	}
	for _, name := range names {
		if hasLegacyPrefix(name) {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE %s RENAME CONSTRAINT %s TO %s`,
			quoteIdent(table), quoteIdent(name), quoteIdent(legacyV1Prefix+name))
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// renameIndexes prefixes whatever indexes are left after the constraint pass —
// the plain CREATE INDEX ones, including the idx_skills_featured that would
// otherwise shadow V2's index of the same name.
func renameIndexes(tx *gorm.DB, table string) error {
	var names []string
	if err := tx.Raw(`
		SELECT indexname FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename = ?
		ORDER BY indexname
	`, table).Scan(&names).Error; err != nil {
		return err
	}
	for _, name := range names {
		if hasLegacyPrefix(name) {
			continue
		}
		target, err := freeRelationName(tx, legacyV1Prefix+name)
		if err != nil {
			return err
		}
		stmt := fmt.Sprintf(`ALTER INDEX %s RENAME TO %s`, quoteIdent(name), quoteIdent(target))
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// renameSequences prefixes the sequences owned by the table's serial columns.
// ALTER TABLE ... RENAME leaves them behind under the old name, which is enough
// to push V2's own sequence onto a "_seq1" suffix and make production's schema
// permanently different from a fresh install.
//
// The ownership link is a pg_depend edge from the sequence to the column
// (deptype 'a' — auto — for serial columns, 'i' for identity columns), not
// anything derivable from the name.
func renameSequences(tx *gorm.DB, table string) error {
	var names []string
	if err := tx.Raw(`
		SELECT s.relname
		FROM pg_class s
		JOIN pg_namespace sn ON sn.oid = s.relnamespace
		JOIN pg_depend d     ON d.objid = s.oid
		                    AND d.classid    = 'pg_class'::regclass
		                    AND d.refclassid = 'pg_class'::regclass
		                    AND d.deptype IN ('a', 'i')
		JOIN pg_class t      ON t.oid = d.refobjid
		JOIN pg_namespace tn ON tn.oid = t.relnamespace
		WHERE s.relkind = 'S'
		  AND sn.nspname = current_schema()
		  AND tn.nspname = current_schema()
		  AND t.relname = ?
		ORDER BY s.relname
	`, table).Scan(&names).Error; err != nil {
		return err
	}
	for _, name := range names {
		if hasLegacyPrefix(name) {
			continue
		}
		target, err := freeRelationName(tx, legacyV1Prefix+name)
		if err != nil {
			return err
		}
		stmt := fmt.Sprintf(`ALTER SEQUENCE %s RENAME TO %s`, quoteIdent(name), quoteIdent(target))
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// freeRelationName returns candidate, or candidate_2, candidate_3, ... if the
// name is taken. A previous partial run — or someone having run the SQL script
// by hand on one environment — must not turn into a hard failure here.
func freeRelationName(tx *gorm.DB, candidate string) (string, error) {
	name := candidate
	for i := 2; ; i++ {
		taken, err := relationExists(tx, name)
		if err != nil {
			return "", err
		}
		if !taken {
			return name, nil
		}
		if i > 50 {
			return "", fmt.Errorf("no free name for %s after 50 attempts", candidate)
		}
		name = fmt.Sprintf("%s_%d", candidate, i)
	}
}

func hasLegacyPrefix(name string) bool {
	return len(name) >= len(legacyV1Prefix) && name[:len(legacyV1Prefix)] == legacyV1Prefix
}

// quoteIdent double-quotes an identifier. Every value reaching it is a constant
// from legacyV1Tables or a name read back from the catalog, never user input,
// but the DDL statements above cannot use bind parameters for identifiers so
// the escaping is written out rather than assumed.
func quoteIdent(name string) string {
	quoted := make([]rune, 0, len(name)+2)
	quoted = append(quoted, '"')
	for _, r := range name {
		if r == '"' {
			quoted = append(quoted, '"')
		}
		quoted = append(quoted, r)
	}
	quoted = append(quoted, '"')
	return string(quoted)
}
