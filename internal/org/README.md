# Enterprise Org

A customer company as an organization: members, roles, departments, and the
keys and usage that belong to it. The product spec is the meta-repo's
`docs/enterprise-org-prd.md`; this file is what you need to know before
changing the code.

## Layout

| Package | Holds | May import the platform `model` package? |
|---|---|---|
| `model/` | The seven org tables and the migration; the permission vocabulary — primitives, inherent powers, preset roles, role packs, audit actions — and the permission engine `Can` | **No** — `model/main.go` imports it to migrate, so the reverse is a cycle |
| `service/` | Logic that also touches `users` / `tokens`: organizations, departments, members, service accounts, invites, custom roles, organization keys, the audit log | Yes |
| `orgtest/` | Throwaway databases for tests, one per engine | **No** — tests in both packages above use it |

Handlers live in `controller/org.go` and `controller/org_keys.go`, routes under
`/api/org` in `router/api-router.go`. The web pages are
`web/default/src/features/org`.

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

## Who may do what

One function decides: `model.Can(subject, action, target)` (`model/can.go`). It
asks the two questions of PRD §2 — does the role grant the action, and is the
target within the role's reach — and nothing else. It is pure: no database, no
request. `model/can_test.go` is its truth table, every primitive against every
preset role inside and outside its scope, written out by hand from the PRD.

- **`action`** is a primitive (`key.assign`, `member.invite`, … — the 13 in
  `model.Primitives`) or the name of an inherent power (`model.Power…`).
  Anything else is refused, for the owner too.
- **A primitive** is granted by the role, and reaches as far as the role's
  scope: the whole organization (`org`), or the departments its holder manages
  (`dept`). A write brings the read of the same resource.
- **An inherent power** — roles, departments, service accounts, appointing
  admins — comes with being the owner or an admin. No role lists one, no custom
  role can hold one, and the three marked `OwnerOnly` are the owner's alone.
- **Every member reads what is their own**: the keys assigned to them and their
  own usage, whatever their role.
- **`target`** is the department and the member the thing belongs to. The zero
  target is the organization as a whole, which only an `org`-scoped role — or
  an inherent power — acts on.

Service functions never decide for themselves. Each takes an `*Actor` — loaded
fresh from the database on every request, never cached, which is what makes a
changed role take effect without anyone signing in again — and asks through
one of three methods (`service/actor.go`):

| Method | For | Answers |
|---|---|---|
| `actor.allow(action, target)` | a read of one thing | `nil` or `ErrForbidden` |
| `actor.reach(primitive)` | a list | everything, or the departments to cut it to; `ErrForbidden` when neither |
| `actor.permit(action, target)` | a write | a permit to record the change with, or `ErrForbidden` |

What each endpoint asks for:

| Endpoint | Needs |
|---|---|
| `GET /departments`, `GET /members` | `member.read`; a `dept`-scoped role is sent its departments only |
| `GET /roles`, `GET /permissions` | `member.read` anywhere — roles belong to no department |
| `POST/PUT/DELETE /departments`, moving a member | power `department.manage` |
| `POST/PUT/DELETE /roles`, `POST /role-packs/:key/adopt`, giving a member a role or departments to manage | power `role.manage`; plus `admin.appoint` when the admin role is involved |
| `POST /service-accounts` | power `service_account.manage` (PRD D27) |
| `POST /invites`, `GET /invites`, `DELETE /invites/:id` | `member.invite` reaching the invite's department — see below |
| `GET /audit-logs` | `audit.read` across the whole organization |
| `GET /keys` | `key.read`; a `dept`-scoped role is sent the keys held in its departments only |
| `GET /key-templates` | `key.read` anywhere |
| `GET /key-holders` | `key.create` anywhere; answers the members a key can be made out to, each with their role only where the caller may read that member (D34) |
| `GET /key-models?holder_id=` | `key.create` or `key.update` over that member; answers the models a key of theirs can be limited to by hand |
| `POST /keys` | `key.create` over the holder, plus `key.assign` unless the key is parked under the owner (D30) |
| `PUT /keys/:id` | `key.update` over the holder |
| `POST /keys/:id/rotate` | `key.rotate` over the holder |
| `POST /keys/:id/freeze`, `…/unfreeze` | `key.freeze` over the holder |
| `DELETE /keys/:id` | `key.delete` over the holder |

`service/gate_test.go` tries every one of these as every kind of member, against
a table that says by hand who gets through. A new management function belongs
in that table; the test counts them.

### Rules on top of the engine

- **Only the owner appoints and dismisses admins** (PRD D10) — by changing a
  role, or by an invite that grants the admin role.
- **Nobody changes the owner's role and nobody is made an owner.** Ownership
  moves by hand, through the platform (D19). The owner also cannot delete their
  own account (`controller.DeleteSelf`): it is the company wallet.
- **The owner and the admins sit in the default department** (D26), and nobody
  moves them out of it. `placeMember` decides it: being appointed an admin
  takes a member there, and an invite that makes admins can only point there.
- **Nobody gives what they do not have** (PRD §2). Whoever invites without
  being the owner or an admin invites Staff, and only into departments their
  role reaches. `mayIssue` is that rule — and it also decides which invite
  links a member **sees** and may **revoke**: a link is a way in, so showing one
  to somebody who could not have made it would hand them that way in. An admin
  therefore does not see the owner's admin links.
- **"Is an admin" always means the platform preset** (`org_roles.org_id = 0`),
  see `isPreset`. A custom role cannot take a preset's name (`checkRoleName`),
  and one written into the table by hand under that name still grants nothing
  beyond its primitives.

### Custom roles and role packs

- A custom role is a name, a scope (`org` or `dept`) and at least one
  primitive. It is stored with the reads its writes bring
  (`model.WithImpliedReads`), and `/api/org/self` reports permissions the same
  way, so a client looks a primitive up in a list and applies no rule of its
  own.
- `audit.read` needs `org` scope: the audit log belongs to the organization as
  a whole and records carry no department.
- The five presets cannot be changed or deleted. Change one in
  `model.PresetRoles`, nowhere else; it is synced into `org_roles` on boot.
- **Deleting a role in use** (D29) sends its holders, and the invite links
  that pointed at it, back to Staff — the role that grants nothing.
- A role pack (`model.RolePacks`) is copied into a custom role on adoption and
  is the organization's own from then on. The page sends the name it shows the
  pack under, so the role comes out in the language the admin is reading.
- `GET /permissions` and `RoleView.Powers` exist so that the roles page holds
  **no list of its own**: every row and every tick of its matrix is the
  backend's answer.

### Which departments a manager manages

A member whose role has `dept` scope always manages **the department they
belong to** (D28), so moving them moves what they manage. `department_managers`
holds only the *further* departments an owner or admin added for them, and
holds rows only for members with such a role: any other role clears them
(`manageDepartments`, `UpdateRole`, `DeleteRole`), and so does deleting the
department.

## Organization keys

An organization key is an ordinary `tokens` row with `org_id` set. The gateway
serves it like any other key; what is different is who may touch it.

- **It is managed under `/api/org/keys` and nowhere else.** The personal key
  endpoints only ever asked "is it your key", and a key handed to a member *is*
  theirs by that test — so each of them that changes, deletes or reveals a key
  starts with `controller.refuseOrgKeys` and answers an organization key with
  403. A batch that names one is refused whole. A new personal endpoint that
  writes a key or returns its value needs the same first line.
- 🔴 **A key's value leaves the server in exactly two ways** (PRD D15):
  1. in the answer to the request that *made* it — `CreateKey` or `RotateKey` —
     and only when the holder is a service account, which has no other way to
     receive it (`KeyGrant.Value`);
  2. through one-click setup, to the holder's own machine
     (`internal/connect.RedeemScript`), which writes a `key.deliver` record
     first and hands out nothing when that fails.

  `KeyView` — every list, every other answer — carries the masked form. The
  audit log keeps masked values too, and says `value_shown` when way 1 happened.
- **Creating a key for someone is also handing it out** (D30): it takes
  `key.assign` as well as `key.create` over that member. A key made out to
  nobody is parked under the owner and takes `key.create` alone.
  `mayCreateKeyFor` is that rule; `GET /key-holders` answers it for the create
  form, so the form works for a role that cannot read the member list.
- **The holder list tells a member's role only to whoever may read that
  member** (D34). The create form finds a person in a long list by name, by
  department and by role, so `KeyHolderView` carries all three. The name and
  the department are what the form cannot do without. Who holds which role is
  the member list's to tell: `role_id` and `role` stay empty unless the actor
  holds `member.read` over that member, and the page then offers no filter by
  role. The list is not searched or paged on the server — the page is sent all
  of it, already cut to whom the actor may make a key out to, and searches and
  filters it there (measured locally at 10,000 members: 28 ms and 1.4 MB).
- **What a key may call is one of three things** (PRD §5): every model, the
  models of a policy template, or a list picked by hand. All three end up in
  the same place — `tokens.model_limits`, which is what the gateway enforces —
  and `resolveAllowance` is the one function that fills it. A key takes a
  template or a list, never both; `policy_template` is empty for a hand-picked
  list. A change replaces the two together (`KeyPatch`): whichever is not in
  the patch counts as empty.
- **A policy template is resolved once, when it is applied** (D14).
  `model.PolicyTemplates` says which purposes a template is made of; the
  controller resolves each purpose through the same two sources a personal
  key's purpose goes through (`controller.orgModelCatalogue`, which is the
  service's `ModelCatalogue`). The result is written into `model_limits` and
  the template's key into `policy_template`, so applying it again later picks
  up models the catalogue has gained. A template that is unknown, or that
  resolves to nothing, is refused — it must never fall through to "every
  model".
- **A hand-picked model has to be one the key's holder can be served** (D33):
  enabled in the holder's group and priced — the list `GET /key-models` offers
  the form, read from the live catalogue for the *holder*, not for whoever
  fills in the form. Anything else is refused, and the refusal names it
  (`KeyModelsUnavailableError`): a mistyped name, a model of another group, and
  a rule such as `claude-*` or `*` — a list cannot say "everything". The one
  exception is what the key is limited to already: those entries may stay
  through a change, whatever the catalogue says today. A channel that is down
  for a minute takes its models out of the catalogue, and an edit that only
  adds a model must not fail over one nobody touched; the same goes for the
  rules a personal purpose left on a founder's first key. So a change is only
  ever refused for what it adds.
- **A change takes effect on the next request.** The gateway caches a key for
  `SYNC_FREQUENCY` seconds and answers from that cache without looking at the
  database, so every function that changes a key ends with `forgetKeyValue`:
  drop the entry now and once more two seconds later, for a request that read
  the old row just before the change and writes it back just after the first
  drop. A new function that writes an organization key must end the same way,
  or "frozen" means "frozen within a minute".
- **`UpdateKey` writes only the fields the patch names.** The key is spending
  its quota while somebody has the form open; writing back the number the form
  started with would refund it.
- **Rotating keeps the row.** Holder, settings, used quota and usage history
  stay; only the value changes, and the old one is dead at once. Deleting is a
  soft delete for the same reason: the usage log still points at the id.
- **Unfreezing is `key.freeze` too**, recorded as `key.unfreeze`. It refuses a
  key that is past its expiry or has no quota left: change that first.

## The audit log

Every management write goes into `org_audit_logs` (D17): who, what, on which
target, the values before and after, the request's address, when.

- **A write is allowed by `actor.permit(…)` and recorded through the permit it
  returns** — `permit.record(tx, action, targetType, id, before, after)`. A
  function that asks for a permit and then forgets the record does not compile
  (Go rejects the unused variable), which is the point: the log cannot fall
  behind by oversight.
- **Pass the transaction of the change itself.** The change and its record
  commit or roll back together.
- `action` is one of `model.Audit…`: a primitive's name where the act is one,
  a name of its own under an inherent power (`department.rename`) or for one
  of two opposite acts a single primitive covers (`key.unfreeze`).
- The detail carries **names as well as ids**, so a record still reads right
  after the department or role it mentions is renamed or gone.
- 🔴 **Never put a credential in a record.** An invite's code is left out on
  purpose (`inviteRecord`): whoever may read the log is not thereby entitled
  to let people in. A key's value is left out the same way — a rotation keeps
  the two masked values, nothing more.
- `RecordAudit` is exported for the few recorded events that are nobody's
  management action. Today there are two: someone joining through an invite
  link (`member.join`), recorded because a link admits anyone who holds it;
  and a holder taking their key's value through one-click setup
  (`key.deliver`, via `RecordKeyDelivery`), recorded because that is the one
  moment a person's key leaves the server.
- The log is append-only. Nothing in this package updates or deletes a record,
  and there is no endpoint that does.

## What exists today

- Tables and columns of PRD §7.2, through GORM only (AGENTS.md Rule 2).
  Exercised on SQLite and PostgreSQL; the MySQL pass below has not been run
  yet (2026-10-06).
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
- Members: list, change role, department and managed departments
  (`service.UpdateMember`, all of a request or none of it), create service
  accounts.
- Invite links (`/sign-up?org_invite=<code>`): create, list, revoke, a public
  preview, and `service.JoinByInviteTx`, which sign-up calls the same way it
  calls `CreateForOwnerTx`.
- Roles: the five presets, custom roles (create, change, delete), role packs,
  the permission catalogue.
- Organization keys: list, create (for a person, a service account, or
  parked under the owner), change, rotate, freeze, unfreeze, delete; the two
  policy templates, and a list of models picked by hand; the members a key
  can be made out to, for the form to search and filter. Handing a key to
  someone else and taking it back are P6.
- The audit log: written by every management write, read by page.

### How a refusal travels

`controller.orgError` maps the service's sentinel errors to messages
(`i18n/locales/*.yaml`, keys `org.*`). Being refused for lack of permission is
a real **HTTP 403**; every other refusal is the usual 200 + `success=false`.
`controller.TestOrgMessages_AreTranslatedInEveryLocale` fails when a message
is missing from a locale file.

### Things that are easy to get wrong

- **Every lookup is scoped to the actor's organization** (`findDepartment`,
  `findMember`, `findRole`, the invite queries). An id that belongs to another
  company must read as "not found", never as "forbidden" and never succeed.
  Inside the organization it is the other way round: a thing out of the
  actor's reach is **403**, not "not found".
- **A list is cut on the server.** A manager's member list is a `WHERE` on
  their departments (`actor.reach`), not a full list the page filters.
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
- **Tests run without Redis, and say so.** `common.RedisEnabled` is `true`
  until start-up decides otherwise, so a test that reaches the token cache
  with it left on dereferences a nil client. `forEachDialect` switches it off;
  `dropCachedKey` is the seam the key tests watch instead. The cache path
  itself is only exercised against a real Redis.
- **Someone who joins by invite gets no starter key** (PRD D16), and their
  console mode is not decided for them. The founder's is: sign-up stores them
  as a `team` persona so they land in the Advanced console (PRD D24).

## Not here yet, and where it will hook in

- **Removing a member** — with key reclaim, in one transaction (P6). Until
  then a member, a service account included, cannot be taken out. It must
  also clear the member's `department_managers` rows, and must not let a
  `dept`-scoped `member.remove` reach the owner or an admin who happens to sit
  in a department it manages.
- **Assigning and reclaiming keys** (P6): moving a key between holders under
  `key.assign`. Reclaiming freezes the key and parks it under the owner;
  assigning a key that was reclaimed gives it a new value first (PRD §3). Each
  of them writes the key, so each must end with `forgetKeyValue`. Until P6 a
  member of an organization
  can still create personal keys of their own (`controller.AddToken`), and the
  Simple console does not find organization keys, which carry no
  `simple_purpose`.
- **The billing branch and log stamp** (P7) — until then an organization key
  spends its holder's own balance — and **alerts** (P8) — the powers
  `org.settings` and `alert.handle` are defined and have no endpoint yet —
  and **reports and the audit log page** (P9; the endpoint is here).

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
