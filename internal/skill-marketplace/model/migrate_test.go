package model_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Migrate() is PostgreSQL-only raw DDL (TEXT[], JSONB, partial indexes) and
// was never once run against a real Postgres server before this test existed
// — every other test in this package uses SQLite, which tolerates things a
// real PG driver rejects. Gated on TEST_POSTGRES_DSN (an admin/superuser DSN
// pointed at the `postgres` maintenance database, e.g.
// postgresql://root:123456@localhost:5432/postgres) so it only runs when a
// real server is available; skipped otherwise, same convention as
// controller/token_test.go's TEST_POSTGRES_DSN-gated tests.
func TestMigrate_RunsAgainstRealPostgres(t *testing.T) {
	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the real-Postgres migration test")
	}

	testDSN := createScratchDatabase(t, adminDSN)

	// PrepareStmt: true matches model/main.go's real connection config
	// exactly — it's *why* the bug this test exists for only shows up
	// against a real server: PrepareStmt forces the extended query
	// protocol, which is what rejects multiple commands in one Exec.
	// Without it, the same buggy SQL runs (simple protocol tolerates
	// multi-statement), and this test would pass while production breaks.
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err, "open scratch database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Migrate()'s DDL references users(id) (created_by/admin_id FKs). In the
	// real app this table always exists first — upstream's own migration
	// creates it long before internal/skill-marketplace's Migrate() runs —
	// so a minimal stand-in is the correct fixture here, not a gap.
	require.NoError(t, db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`).Error)

	// This call alone reproduces the bug this test exists for: three of
	// Migrate()'s six DDL steps packed multiple `CREATE INDEX ...;` commands
	// into a single Exec, which PostgreSQL's extended query protocol
	// rejects outright (SQLSTATE 42601, "cannot insert multiple commands
	// into a prepared statement") — caught in the P5 real-service
	// walkthrough (2026-09-04); Migrate() had never run against real PG
	// before that.
	require.NoError(t, model.Migrate(db), "first Migrate() run")

	for _, table := range []string{
		"skills",
		"skill_versions",
		"user_enabled_skills",
		"skill_purchases",
		"skill_admin_logs",
	} {
		require.True(t, db.Migrator().HasTable(table), "table %q should exist after Migrate()", table)
	}

	// Every app boot calls Migrate() again against the same database — the
	// IF NOT EXISTS / guarded-DO-block design only holds up if a second run
	// is a genuine no-op.
	require.NoError(t, model.Migrate(db), "second Migrate() run must be idempotent")
}

// createScratchDatabase opens the admin DSN's own (maintenance) database,
// creates a uniquely-named database for this test run, and registers
// cleanup to drop it — so this test never touches whatever database the
// admin DSN names, and never leaves debris on a shared server. Returns a
// DSN pointed at the new scratch database.
func createScratchDatabase(t *testing.T, adminDSN string) string {
	t.Helper()

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	require.NoError(t, err, "open admin DSN")
	adminSQLDB, err := adminDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminSQLDB.Close() })

	dbName := fmt.Sprintf("skill_marketplace_migrate_test_%d", time.Now().UnixNano())
	require.NoError(t, adminDB.Exec(fmt.Sprintf(`CREATE DATABASE %q`, dbName)).Error,
		"create scratch database %q", dbName)

	t.Cleanup(func() {
		adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, dbName))
	})

	return replaceDBName(adminDSN, dbName)
}

// replaceDBName swaps a postgres DSN's trailing /<dbname> path segment
// (everything after the last '/' and before an optional '?query').
func replaceDBName(dsn, newName string) string {
	idx := strings.LastIndex(dsn, "/")
	if idx == -1 {
		return dsn
	}
	base := dsn[:idx]
	query := ""
	if q := strings.IndexByte(dsn[idx:], '?'); q != -1 {
		query = dsn[idx:][q:]
	}
	return base + "/" + newName + query
}
