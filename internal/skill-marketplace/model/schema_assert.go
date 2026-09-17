package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Post-migration schema assertions.
//
// Every CREATE in migrate.go is guarded by IF NOT EXISTS, which means a table
// that already exists under the right name is accepted without anyone ever
// looking at its shape. That is what let a V1 skills table (id CHAR(36)) pass
// for a V2 one (id BIGSERIAL) for six days: the migration reported success at
// every step until a foreign key finally refused to point at the wrong type.
//
// These assertions close that gap generically. They do not know anything about
// V1 — they check that what ended up in the database is what migrate.go set out
// to build, so any future cause of the same class gets named on the spot
// instead of surfacing as a puzzling error somewhere downstream.
//
// 🔴 Read this before relying on it:
//
// The result reaches a log line and nothing else. This deployment has no log
// access for the people who would act on it, no diagnostic endpoint, and no
// post-deploy health check, and Migrate is fail-soft — so an assertion failure
// does not turn the deploy red, does not page anyone, and does not change what
// any endpoint returns. For missing columns it hardly matters: the public
// /api/skills handler passes err.Error() straight through, so a shape problem
// that breaks queries is visible from outside anyway. For a missing index or a
// missing foreign key — the two failures this incident actually produced —
// there is no external symptom at all, and nobody will hear the assertion fire.
//
// That was a deliberate call (2026-09-08): the assertion is written for whoever
// eventually does have log access, not as a safety net for the team shipping
// it. Anyone adding a diagnostic surface or a deploy health check later should
// wire this in as its first input.

// expectedColumn is one column whose type carries load. Type strings are
// information_schema.data_type values ("bigint", "ARRAY", "text").
type expectedColumn struct {
	table  string
	column string
	typ    string
}

// The invariants worth asserting are the ones that have actually broken, not
// every column in the schema — an assertion that fails on a legitimate future
// column addition would be noise, and noise nobody can hear is worse than
// nothing.
var (
	expectedTables = []string{
		"skills",
		"skill_versions",
		"user_enabled_skills",
		"skill_purchases",
		"skill_admin_logs",
	}

	expectedColumns = []expectedColumn{
		// The V1/V2 divergence that caused the outage: V1's ids are CHAR(36).
		{"skills", "id", "bigint"},
		{"skill_versions", "id", "bigint"},
		{"skill_purchases", "skill_id", "bigint"},
		// Written through pq.StringArray; a plain TEXT column here was a P1
		// production bug in its own right (fixed in #163).
		{"skills", "tags", "ARRAY"},
		// PRD §13 reference listings (addReferenceListingColumns) — added via
		// ADD COLUMN IF NOT EXISTS, which accepts a pre-existing column of the
		// wrong type exactly the way CREATE TABLE IF NOT EXISTS accepted V1's.
		{"skills", "listing_type", "character varying"},
		{"skills", "source_url", "character varying"},
	}

	// Indexes and constraints that a name collision can silently swallow,
	// because CREATE INDEX IF NOT EXISTS and the pg_constraint existence check
	// both accept a name owned by some other table as proof of existence.
	expectedIndexes = map[string][]string{
		"skills": {"idx_skills_status", "idx_skills_featured", "idx_skills_created"},
	}

	expectedConstraints = map[string][]string{
		"skills": {
			"fk_skills_active_version",
			"skills_listing_type_check",
			"skills_reference_source_check",
			"skills_reference_free_check",
		},
	}
)

// assertSchema reports every deviation at once rather than stopping at the
// first: whoever reads this log line is unlikely to get a second run out of the
// database, so one message has to carry the whole picture.
func assertSchema(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	var problems []string

	for _, table := range expectedTables {
		ok, err := relationExists(db, table)
		if err != nil {
			return fmt.Errorf("assert table %s: %w", table, err)
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("table %s is missing", table))
		}
	}

	for _, want := range expectedColumns {
		ok, err := columnExists(db, want.table, want.column, want.typ)
		if err != nil {
			return fmt.Errorf("assert column %s.%s: %w", want.table, want.column, err)
		}
		if !ok {
			actual, err := columnType(db, want.table, want.column)
			if err != nil {
				return fmt.Errorf("assert column %s.%s: %w", want.table, want.column, err)
			}
			if actual == "" {
				problems = append(problems, fmt.Sprintf("column %s.%s is missing", want.table, want.column))
			} else {
				problems = append(problems, fmt.Sprintf(
					"column %s.%s is %s, want %s", want.table, want.column, actual, want.typ))
			}
		}
	}

	for table, indexes := range expectedIndexes {
		for _, index := range indexes {
			ok, err := indexOnTable(db, table, index)
			if err != nil {
				return fmt.Errorf("assert index %s: %w", index, err)
			}
			if !ok {
				// Naming the likely cause here is worth the words: the index
				// almost certainly exists, just attached to something else.
				problems = append(problems, fmt.Sprintf(
					"index %s is not on %s (a relation elsewhere may hold the name)", index, table))
			}
		}
	}

	for table, constraints := range expectedConstraints {
		for _, constraint := range constraints {
			ok, err := constraintOnTable(db, table, constraint)
			if err != nil {
				return fmt.Errorf("assert constraint %s: %w", constraint, err)
			}
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"constraint %s is not on %s (a relation elsewhere may hold the name)", constraint, table))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("marketplace schema is not what the migration built: %s",
			strings.Join(problems, "; "))
	}
	return nil
}

func columnType(db *gorm.DB, table, column string) (string, error) {
	var types []string
	err := db.Raw(`
		SELECT data_type FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?
	`, table, column).Scan(&types).Error
	if err != nil || len(types) == 0 {
		return "", err
	}
	return types[0], nil
}

// indexOnTable is scoped to the table on purpose. Asking only whether the name
// exists is exactly the mistake this file is here to catch.
func indexOnTable(db *gorm.DB, table, index string) (bool, error) {
	var count int64
	err := db.Raw(`
		SELECT count(*) FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename = ? AND indexname = ?
	`, table, index).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func constraintOnTable(db *gorm.DB, table, constraint string) (bool, error) {
	var count int64
	err := db.Raw(`
		SELECT count(*) FROM pg_constraint
		WHERE conrelid = to_regclass(?) AND conname = ?
	`, table, constraint).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
