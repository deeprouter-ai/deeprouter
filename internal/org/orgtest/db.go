// Package orgtest opens throwaway databases for the Enterprise Org tests, so
// every schema and query is exercised on each engine the gateway supports
// (AGENTS.md Rule 2) and not only on SQLite, which tolerates things a real
// server rejects.
//
// 🔴 It must import neither internal/org/model nor the platform model package:
// tests inside both use it, and either import would be a cycle.
package orgtest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ForEachDialect runs fn once per available database engine, each time against
// a fresh, empty database.
//
// SQLite always runs. PostgreSQL runs when TEST_POSTGRES_DSN is set and MySQL
// when TEST_MYSQL_DSN is set; both are admin DSNs — a scratch database is
// created on that server and dropped afterwards, the same convention as
// internal/skill-marketplace/model/migrate_test.go. Examples:
//
//	TEST_POSTGRES_DSN=postgresql://root:123456@localhost:5432/postgres
//	TEST_MYSQL_DSN='root:123456@tcp(localhost:3306)/mysql?charset=utf8mb4&parseTime=True'
func ForEachDialect(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) { fn(t, OpenSQLite(t)) })
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		t.Run("postgres", func(t *testing.T) { fn(t, openPostgres(t, dsn)) })
	}
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		t.Run("mysql", func(t *testing.T) { fn(t, openMySQL(t, dsn)) })
	}
}

// gormConfig mirrors model/main.go. PrepareStmt matters: it switches
// PostgreSQL to the extended query protocol, which rejects SQL the simple
// protocol accepts.
func gormConfig() *gorm.Config {
	return &gorm.Config{PrepareStmt: true}
}

// OpenSQLite opens a fresh SQLite database file in the test's temp dir. A file
// rather than ":memory:" because a pooled in-memory database is a different
// database per connection.
func OpenSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "org.db"))
	// synchronous(OFF): a throwaway database has nothing to lose in a crash,
	// and without the fsync after every statement the suite runs several
	// times faster.
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=synchronous(OFF)"
	db, err := gorm.Open(sqlite.Open(dsn), gormConfig())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	closeOnCleanup(t, db)
	return db
}

// openPostgres creates a scratch database on the server behind adminDSN and
// opens it.
func openPostgres(t *testing.T, adminDSN string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin DSN: %v", err)
	}
	closeOnCleanup(t, admin)
	name := scratchName()
	if err := admin.Exec(fmt.Sprintf(`CREATE DATABASE %q`, name)).Error; err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}
	t.Cleanup(func() {
		admin.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  replaceDatabase(adminDSN, name),
		PreferSimpleProtocol: true, // as in model/main.go
	}), gormConfig())
	if err != nil {
		t.Fatalf("open scratch database %s: %v", name, err)
	}
	closeOnCleanup(t, db)
	return db
}

// openMySQL creates a scratch database on the server behind adminDSN and
// opens it.
func openMySQL(t *testing.T, adminDSN string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(mysql.Open(adminDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open mysql admin DSN: %v", err)
	}
	closeOnCleanup(t, admin)
	name := scratchName()
	if err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP DATABASE IF EXISTS `" + name + "`")
	})
	db, err := gorm.Open(mysql.Open(replaceDatabase(adminDSN, name)), gormConfig())
	if err != nil {
		t.Fatalf("open scratch database %s: %v", name, err)
	}
	closeOnCleanup(t, db)
	return db
}

// scratchSerial numbers the scratch databases one test process creates.
var scratchSerial atomic.Int64

// scratchName returns a database name no other test run is using. The clock
// alone does not make it so: `go test` runs the packages that use this helper
// as separate processes at the same time, and on Windows the clock moves in
// steps long enough for two of them to read the same instant.
func scratchName() string {
	return fmt.Sprintf("org_test_%d_%d_%d", time.Now().UnixNano(), os.Getpid(), scratchSerial.Add(1))
}

// replaceDatabase swaps the database name of a DSN: the path segment after
// the last "/" and before an optional "?query". Works for both the postgres
// URL form and the MySQL "user:pass@tcp(host)/db" form.
func replaceDatabase(dsn string, name string) string {
	slash := strings.LastIndex(dsn, "/")
	if slash == -1 {
		return dsn
	}
	query := ""
	if q := strings.IndexByte(dsn[slash:], '?'); q != -1 {
		query = dsn[slash+q:]
	}
	return dsn[:slash] + "/" + name + query
}

// closeOnCleanup closes the connection pool when the test ends, before the
// temp dir or the scratch database behind it is removed.
func closeOnCleanup(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
