# Enterprise Org

A customer company as an organization: members, roles, departments, and the
keys and usage that belong to it. The product spec is the meta-repo's
`docs/enterprise-org-prd.md`; this file is what you need to know before
changing the code.

## Layout

| Package | Holds | May import the platform `model` package? |
|---|---|---|
| `model/` | The seven org tables, the permission primitives, the preset roles and departments, the migration | **No** — `model/main.go` imports it to migrate, so the reverse is a cycle |
| `service/` | Logic that also touches `users` / `tokens`: organizations, departments, members, service accounts, invites | Yes |
| `orgtest/` | Throwaway databases for tests, one per engine | **No** — tests in both packages above use it |

Handlers live in `controller/org.go`, routes under `/api/org` in
`router/api-router.go`. The web page is `web/default/src/features/org`.

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
- `service.CreateForOwnerTx` — create an organization, its starter departments
  (`model.PresetDepartments`: the default one plus five business units) and
  make a personal account its owner. Sign-up calls it inside the transaction
  that inserts the user (`controller/org.go`), so the account and the
  organization appear together or not at all. The departments are named once,
  in the language of the sign-up request, and are the company's own data from
  then on: switching the interface language never renames them. The default
  frontend sends the page's language with that request
  (`web/default/src/features/auth/api.ts`), because the browser's own
  `Accept-Language` need not match what the page shows.
- Departments: list, create, rename, delete. The default department can be
  renamed and never deleted; deleting any other moves its members and unused
  invite links to the default one, in one transaction.
- Members: list, change role and department (`service.UpdateMember`), create
  service accounts.
- Invite links (`/sign-up?org_invite=<code>`): create, list, revoke, a public
  preview, and `service.JoinByInviteTx`, which sign-up calls the same way it
  calls `CreateForOwnerTx`.

### Who may do what, for now

Every management function takes an `*Actor` and starts with
`actor.requireManager()`: **the owner and the admins run the organization,
nobody else may.** That one line is the whole permission model until the
permission engine (`Can()`, PRD §7.5) replaces it — which is also why
`internal/org/service/gate_test.go` lists every management function: a new one
that forgets the gate fails there.

On top of the gate, four rules about the owner and admins (PRD D10, D19, D26):

- Only the owner appoints and dismisses admins — by changing a role, or by an
  invite that grants the admin role.
- Nobody changes the owner's role and nobody is made an owner. Ownership moves
  by hand, through the platform. The owner also cannot delete their own
  account (`controller.DeleteSelf`): it is the company wallet.
- The owner and the admins sit in the default department, and nobody moves
  them out of it — not even they themselves. They run the whole organization,
  so their usage belongs to no business unit. `placeMember` decides it: being
  appointed an admin takes a member there, and an invite that makes admins can
  only point there (`CreateInvite`). Once dismissed, a former admin is an
  ordinary member and can be placed anywhere.
- A custom role may be named anything, `admin` included. "Is an admin" always
  means the platform preset (`org_roles.org_id = 0`), see `isPreset`.

### How a refusal travels

`controller.orgError` maps the service's sentinel errors to messages
(`i18n/locales/*.yaml`, keys `org.*`). Being refused for lack of permission is
a real **HTTP 403**; every other refusal is the usual 200 + `success=false`.

### Things that are easy to get wrong

- **Every lookup is scoped to the actor's organization** (`findDepartment`,
  `findMember`, `findRole`, the invite queries). An id that belongs to another
  company must read as "not found", never as "forbidden" and never succeed.
- **An invite link is not consumed.** It admits everyone who presents it until
  it expires (`InviteValidity`, 7 days) or is revoked — one link serves a team.
- **A service account is a `users` row that nothing can sign in to**: no
  password, no email, no access token, a generated username (usernames are
  unique across the platform, so the company's name for it lives in
  `display_name`). It is written directly, not through `User.Insert`, which
  would hand it the sign-up trial credit. `controller.setupLogin` — where
  every way of signing in ends — refuses it as well, in case someone gives it
  a password later.
- **The dialogs use the themed dropdown** (`web/default/src/features/org/components/org-select.tsx`),
  not `NativeSelect`: the browser draws that one's list itself, and it comes
  out white on the dark theme.
- **Someone who joins by invite gets no starter key** (PRD D16), and their
  console mode is not decided for them. The founder's is: sign-up stores them
  as a `team` persona so they land in the Advanced console (PRD D24).

## Not here yet, and where it will hook in

- **Removing a member** — with key reclaim, in one transaction (P6). Until
  then a member, a service account included, cannot be taken out.
- **`department_managers` is never written.** A member with a dept-scoped role
  (the `manager` preset) manages no department yet. The permission engine (P4)
  decides how a manager gets their departments; `DeleteDepartment` already
  clears the table's rows for a department that goes away.
- `Can()` and custom roles (P4), org key endpoints (P5/P6), the billing branch
  and log stamp (P7), alerts (P8), reports and the audit log (P4/P9).

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
