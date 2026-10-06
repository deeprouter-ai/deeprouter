# Enterprise Org

A customer company as an organization: members, roles, departments, and the
keys and usage that belong to it. The product spec is the meta-repo's
`docs/enterprise-org-prd.md`; this file is what you need to know before
changing the code.

## Layout

| Package | Holds | May import the platform `model` package? |
|---|---|---|
| `model/` | The seven org tables, the permission primitives, the five preset roles, the migration | **No** — `model/main.go` imports it to migrate, so the reverse is a cycle |
| `service/` | Logic that also touches `users` / `tokens` | Yes |
| `orgtest/` | Throwaway databases for tests, one per engine | **No** — tests in both packages above use it |

Handlers live in `controller/org.go`, routes under `/api/org` in
`router/api-router.go`.

## Four rules that are not negotiable

1. **An org role is never a platform role.** Every member, the owner included,
   keeps `users.role = 1`. Org roles live in `users.role_id` → `org_roles`.
   Nothing in this package writes `users.role`, and nothing under `/api/org`
   sits behind `AdminAuth` / `RootAuth`. Mixing the two would let a customer
   read every channel and user on the platform.
2. **A personal account must not notice any of this.** `org_id = 0` rows behave
   exactly as they did before the feature existed: the org columns default to
   zero, are `omitempty` in JSON, and are not writable through the personal
   endpoints. Keep it that way.
3. **Permission checks stay on the management plane.** The relay path may gain
   a billing branch and a log stamp, never a permission query.
4. **The org tables are optional to the gateway.** `model.Migrate` reports a
   failure and returns nil, so a broken org migration cannot stop the gateway
   from booting. The org columns on `users` / `tokens` / `logs` are different:
   they are fields of the platform structs, migrate with the core
   `AutoMigrate`, and failing there is fatal.

## What exists today

- Tables and columns of PRD §7.2, through GORM only (AGENTS.md Rule 2).
  Exercised on SQLite and PostgreSQL; the MySQL pass below has not been run
  yet (2026-10-06).
- The five preset roles, defined once in `model.PresetRoles` and synced into
  `org_roles` (`org_id = 0`) on every boot. Change a preset there, nowhere else.
- `service.CreateForOwnerTx` — create an organization and make a personal
  account its owner. Sign-up calls it inside the transaction that inserts the
  user (`controller/org.go`), so the account and the organization appear
  together or not at all.
- `GET /api/org/self` — the caller's organization, role and permissions, or
  `null` for a personal account.

Not here yet, and where it will hook in: departments are not created with the
organization (the owner's `department_id` is 0 until they are — extend
`CreateForOwnerTx`), and there is no `Can()` permission check, no org key
endpoints, no billing branch.

## Testing

`orgtest.ForEachDialect` runs a test on SQLite always, and on a real server
when its DSN is set — a scratch database is created and dropped per test:

```bash
go test ./internal/org/...
go test ./controller/ ./router/ -run TestOrg

# the same suites against a real server
export TEST_POSTGRES_DSN=postgresql://root:123456@localhost:5432/postgres
export TEST_MYSQL_DSN='root:123456@tcp(localhost:3306)/mysql?charset=utf8mb4&parseTime=True'
```

SQLite accepts things a real server rejects (an over-long `char(32)`, for one),
so run the PostgreSQL pass before trusting a schema change — production is
PostgreSQL. Tests outside this directory are named `TestOrg…`; both CI
workflows select them by that prefix.
