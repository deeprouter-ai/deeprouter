package service

import (
	"strings"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// tagsOverlapWhere builds the SQL fragment + args for "the skill has at
// least one of these tags" (OR semantics), used by both the public and
// admin skill listing filters.
//
// Production runs on PostgreSQL only (PRD §5 preamble), where this is the
// native array-overlap operator against the tags text[] column. The unit
// test harness for this package runs on SQLite (no && operator, no array
// type), where model.Skill.Tags round-trips through the same
// pq.StringArray serialization into a TEXT column formatted like
// `{"writing","code-review"}` — so on that dialect we approximate overlap
// with a per-tag LIKE against that literal, OR'd together. Good enough to
// exercise the surrounding branch logic in tests; the exact operator
// semantics are only real against Postgres.
func tagsOverlapWhere(db *gorm.DB, column string, tags []string) (string, []interface{}) {
	if db.Dialector.Name() == "postgres" {
		return column + " && ?", []interface{}{pq.StringArray(tags)}
	}

	clauses := make([]string, len(tags))
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		clauses[i] = column + " LIKE ?"
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
