package service

// White-box tests for isUniqueViolation.
// CreateSkill's happy-path and duplicate-slug path cannot be tested with the
// SQLite in-memory DB (used in admin_skill_test.go) because db.Create() emits
// a tags column that does not exist in the simplified SQLite schema. These
// tests therefore validate the detection function directly.

import (
	"errors"
	"testing"
)

func TestIsUniqueViolation_PostgresError(t *testing.T) {
	err := errors.New(`ERROR: duplicate key value violates unique constraint "skills_slug_key" (SQLSTATE 23505)`)
	if !isUniqueViolation(err) {
		t.Fatal("expected true for PostgreSQL unique constraint error")
	}
}

func TestIsUniqueViolation_SQLiteError(t *testing.T) {
	err := errors.New("UNIQUE constraint failed: skills.slug")
	if !isUniqueViolation(err) {
		t.Fatal("expected true for SQLite unique constraint error")
	}
}

func TestIsUniqueViolation_OtherError(t *testing.T) {
	if isUniqueViolation(errors.New("connection refused")) {
		t.Fatal("expected false for unrelated error")
	}
}

func TestIsUniqueViolation_Nil(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("expected false for nil")
	}
}
