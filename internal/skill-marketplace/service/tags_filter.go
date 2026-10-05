package service

import (
	"strings"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// normalizeTags is the canonical stored form of Admin-typed tags: trimmed,
// lowercase, blanks and duplicates dropped, first-seen order kept. Never
// returns nil, so an empty list is stored as {} rather than NULL.
func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// tagsOverlapWhere builds the SQL fragment + args for "the skill has at
// least one of these tags" (OR semantics), used by both the public and
// admin skill listing filters.
//
// Case-insensitive on purpose: tags are free text (Admin types them into a
// plain input, no suggestions, unlike the old category ComboboxInput), but
// the fixed filter-button list sends its canonical lowercase value
// (constants.ts SKILL_LABELS / marketplace/index.tsx LABELS). An Admin who
// types "Writing" must still match a click on the "Writing" filter button —
// found the hard way: the && operator is exact-match, so a skill tagged
// "Writing" never matched a filter request for "writing" and looked like
// the filter was broken (2026-09-30, real browser click-through).
//
// Production runs on PostgreSQL only (PRD §5 preamble); the unit test
// harness for this package runs on SQLite (no && operator, no array type),
// where model.Skill.Tags round-trips through the same pq.StringArray
// serialization into a TEXT column formatted like `{"Writing","code"}` — so
// on that dialect we approximate overlap with a per-tag case-insensitive
// LIKE against that literal, OR'd together. Good enough to exercise the
// surrounding branch logic in tests; the exact operator semantics are only
// real against Postgres.
func tagsOverlapWhere(db *gorm.DB, column string, tags []string) (string, []interface{}) {
	if db.Dialector.Name() == "postgres" {
		lowered := make([]string, len(tags))
		for i, t := range tags {
			lowered[i] = strings.ToLower(t)
		}
		return "EXISTS (SELECT 1 FROM unnest(" + column + ") t WHERE LOWER(t) = ANY(?))",
			[]interface{}{pq.StringArray(lowered)}
	}

	clauses := make([]string, len(tags))
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		clauses[i] = "LOWER(" + column + ") LIKE LOWER(?)"
		args[i] = `%"` + tag + `"%`
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// tagsSearchWhere builds the SQL fragment + arg for "any tag contains this
// search term" (case-insensitive substring), meant to be OR'd alongside the
// existing name/description search condition. Same dialect split as
// tagsOverlapWhere: Postgres unnests the real array, SQLite approximates
// against the serialized text column.
func tagsSearchWhere(db *gorm.DB, column, pattern string) (string, interface{}) {
	if db.Dialector.Name() == "postgres" {
		return "EXISTS (SELECT 1 FROM unnest(" + column + ") t WHERE LOWER(t) LIKE LOWER(?))", pattern
	}
	return "LOWER(" + column + ") LIKE LOWER(?)", pattern
}
