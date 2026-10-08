# Enterprise Org

A customer company as an organization: members, roles, departments, and the
keys and usage that belong to it. The product spec is the meta-repo's
`docs/enterprise-org-prd.md`; this file is what you need to know before
changing the code.

## Layout

| Package | Holds | May import the platform `model` package? |
|---|---|---|
| `model/` | The seven org tables and the migration; the permission vocabulary — primitives, inherent powers, preset roles, role packs, audit actions — and the permission engine `Can`; the alert rules and the settings they run on | **No** — `model/main.go` imports it to migrate, so the reverse is a cycle |
| `service/` | Logic that also touches `users` / `tokens`: organizations, departments, members, service accounts, invites, custom roles, organization keys, the audit log, warnings and alerts, the usage report | Yes |
| `orgtest/` | Throwaway databases for tests, one per engine | **No** — tests in both packages above use it |

Handlers live in `controller/org.go`, `controller/org_keys.go`,
`controller/org_alerts.go` and `controller/org_usage.go`, routes under `/api/org` in
`router/api-router.go`. The web pages are `web/default/src/features/org`. What an organization key does on the relay
path — whose balance pays, how its usage is stamped — is under "The company
wallet" below.

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
- **Every member reads what is their own**: the keys assigned to them, their
  own usage and the alerts raised on those keys, whatever their role
  (`ownReads` in `can.go`). Which of those alerts — the warnings, not the
  anomalies — is a rule of the alert list, on top of the engine.
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
| `GET /self/keys?purpose=` | being a member; answers the caller's own keys that can serve a purpose of the console (D16) |
| `GET /departments`, `GET /members` | `member.read`; a `dept`-scoped role is sent its departments only |
| `GET /roles`, `GET /permissions` | `member.read` anywhere — roles belong to no department |
| `POST/PUT/DELETE /departments`, moving a member | power `department.manage` |
| `POST/PUT/DELETE /roles`, `POST /role-packs/:key/adopt`, giving a member a role or departments to manage | power `role.manage`; plus `admin.appoint` when the admin role is involved |
| `POST /service-accounts` | power `service_account.manage` (PRD D27) |
| `DELETE /members/:id` | `member.remove` over the member — never the owner, never oneself, an admin only by the owner |
| `POST /invites`, `GET /invites`, `DELETE /invites/:id` | `member.invite` reaching the invite's department — see below |
| `GET /audit-logs` | `audit.read` across the whole organization; `?actor=`, `?target_type=`, `?start_timestamp=` and `?end_timestamp=` narrow it |
| `GET /usage?group_by=` | being a member — `usage.read` decides how much is answered: everything, what was spent in the departments the role reaches, or only what the caller used themselves. `&department_id=` takes `usage.read` over that department |
| `GET /keys` | `key.read`; a `dept`-scoped role is sent the keys held in its departments only |
| `GET /key-templates` | `key.read` anywhere |
| `GET /key-holders` | `key.create` anywhere; answers the members a key can be made out to, each with their role only where the caller may read that member (D34) |
| `GET /key-models?holder_id=` | `key.create` or `key.update` over that member; answers the models a key of theirs can be limited to by hand |
| `POST /keys` | `key.create` over the holder, plus `key.assign` unless the key is parked under the owner (D30) |
| `PUT /keys/:id` | `key.update` over the holder |
| `POST /keys/:id/rotate` | `key.rotate` over the holder |
| `POST /keys/:id/freeze`, `…/unfreeze` | `key.freeze` over the holder |
| `DELETE /keys/:id` | `key.delete` over the holder |
| `GET /key-assignees` | `key.assign` anywhere; answers the members a key can be handed to — the owner is not one — each with their role only where the caller may read that member |
| `POST /keys/:id/assign` | `key.assign` over the holder **and** over the member the key goes to |
| `POST /keys/:id/reclaim` | `key.assign` over the holder |
| `GET /alerts` | being a member; `alert.read` decides how much: all alerts, those raised on keys held in a `dept`-scoped role's departments, or — without it — the warnings on the caller's own keys (D47) |
| `POST /alerts/:id/handle` | power `alert.handle` |
| `GET /alert-settings`, `PUT /alert-settings` | power `org.settings` |

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
- **Nobody removes the owner, nobody removes themselves, and only the owner
  removes an admin** (`RemoveMember`). The last of the three is what keeps a
  `dept`-scoped `member.remove` that manages the default department — where
  the owner and the admins sit (D26) — from reaching them: removing an admin
  dismisses one, and that is the owner's alone (D10).
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
  1. in the answer to the request that *made* it — `CreateKey`, `RotateKey`, or
     `AssignKey` handing the key to the account — and only when the holder is a
     service account, which has no other way to receive it (`KeyGrant.Value`);
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
- 🔴 **A key's value never outlives a change of holder** (`service/assign.go`).
  Whoever held a key has its value in their tools; a value that went on working
  under the next holder would be spent by one person and billed to another. So
  the one function that moves a key, `handOver`, gives it a new value in the
  same transaction — when a key is assigned, when it is taken back, and when its
  holder is removed. Taking back rotates too, although the key is frozen then:
  otherwise the next person to unfreeze a parked key would wake the copy of
  somebody who has left.
- **Handing a key over takes `key.assign` on both ends** (`AssignKey`): over
  whoever holds it now, and over whoever is to hold it. A manager therefore
  moves keys between the people of their departments and nowhere else, and
  does not reach a key parked under the owner, who sits in the default
  department (decided 2026-10-07, D35). Getting a key into a department is done
  by a role that reaches the whole organization — creating it for someone
  there, or assigning a parked one. `GET /key-assignees` answers the second end
  for the form; the first needs no list of its own — it is the keys
  `GET /keys` already shows the actor.
- **Taking a key back is: frozen, a new value, under the owner** (`ReclaimKey`).
  There is no "unassigned" state: a key under the owner *is* the key nobody has
  been handed, which is why `KeyView.HolderIsOwner` exists and why "assigning"
  a key to the owner takes it back instead. It asks for reach over the holder
  only, and never how many keys the owner holds — a key must be able to come
  back whatever the place it returns to looks like. The owner's own key count
  can exceed `MaxUserTokens` this way; creating a new parked key is then
  refused until some are assigned or deleted.
- **A frozen key that is handed over stays frozen unless the actor may unfreeze
  keys.** `AssignKey` switches a frozen key on for its new holder only when the
  actor holds `key.freeze` there and nothing else stops the key from working
  (`spent`). A manager, who holds `key.assign` alone, hands a frozen key over
  frozen: moving a key is not a way around a freeze.
- **A member makes no key of their own** (D16). The personal `AddToken` starts
  with `controller.refuseOrgMember`, which asks about the *caller* — not about
  a key, which is what sets it apart from `refuseOrgKeys` — and reads a deleted
  account's row too, because a removed member may still hold a session. Where
  the console used to make a key for a purpose, it asks `GET /self/keys?purpose=`
  instead (`ListOwnKeysFor`): the caller's own working keys whose whitelist has
  a model in common with what the purpose is made of (`allowancesMeet` — both
  are lists of names and `prefix*` rules), each with the purpose's named models
  it may call. An empty answer is where the page says "ask an administrator".
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
  database, so every function that changes a key — the three that move one
  included — ends with `forgetKeyValue` on the value the key *had*:
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

## Leaving the organization

There is one way out: `RemoveMember`, for a person and for a service account
alike. In one transaction it

1. takes back every organization key the member holds — frozen, a new value,
   under the owner, a `key.reclaim` record each (see above);
2. clears the departments that were added for them to manage;
3. deletes the account, and writes the `member.remove` record, which carries
   their name, role and department and the ids of the keys.

- **The account is deleted, not detached** (decided 2026-10-07, D37). It is a
  soft delete, like every deleted account on the platform: the row stays —
  with its `org_id`, so `keyHolders` and the audit log can go on naming them —
  and so do its **username and email, which nobody can sign up with again**
  (`model.CheckUserExistOrDeleted` looks at deleted rows; that is the
  platform's rule, not ours). Whoever is removed by mistake needs a new
  username.
- **What they spent stays.** The usage log is never touched, and
  `tokens.used_quota` stays on the keys.
- **Their session may live on, and opens nothing.** The session cookie is not
  re-checked against the database (`middleware.authHelper`), so a removed
  member can still send requests for as long as it lasts. `GetMembership`
  answers nil for an account that no longer exists, which makes every
  `/api/org/*` call a 403; `refuseOrgMember` closes key creation; and their
  keys, the only thing that reaches the gateway, are already dead.
- **No member deletes their own account** (D36): `controller.DeleteSelf`
  refuses anyone with an `org_id`. Leaving has somebody else's name on it in
  the audit log, and takes the keys back on the way — deleting the account
  alone used to leave them under a holder who no longer existed.
- `MemberView.KeyCount` is what the page shows before it asks for
  confirmation: how many keys removing this member would take back.

## The company wallet

An organization holds no money of its own: the company wallet is **the
owner's account balance** (PRD §7.3). A request made with an organization key
is paid from it and recorded under whoever made the request — the member's
`user_id` and key on the usage log, the member's `used_quota` — with two
stamps on the log line: the organization, and the department the member was
in at that moment. Topping up, payments and redemption codes are untouched:
the owner tops up their own account.

- 🔴 **This is the one place the relay path reads the organization tables, and
  it asks no permission question** (rule 3 above). `SpendOf`
  (`service/wallet.go` here) answers three things for an organization key in
  one query — the holder's department, the organization's owner, and whether
  the owner's account can be charged — and `middleware.attachOrgSpend` puts
  them on the request. `TokenAuth` calls it only when `token.OrgId != 0`; a
  personal key never gets there.
- **Nothing is cached, on purpose.** A member who changes department is
  stamped with the new one from the next request, and an owner the platform
  disables stops the organization's spending at once. It costs an
  organization key one query per request — and saves it one, because an
  organization key skips the "has a subscription?" query a personal key
  makes. A request made with a personal key runs exactly the queries it did.
- **The branch is one method.** `RelayInfo.WalletUserId()`
  (`relay/common/org_spend.go`) answers whose balance pays: the owner for an
  organization key, `UserId` for anything else. The places that take money
  from a balance or put it back call it where they used to say
  `relayInfo.UserId`. A zero `OrgSpend` changes nothing, which is what keeps a
  personal account's billing as it was.
- **An organization key never spends a subscription** — not its holder's and
  not the owner's (`NewBillingSession` forces `wallet_only`).
- 🔴 **A refusal never says what the company has left.** The platform's own
  "not enough balance" message quotes the balance. For an organization key
  `orgWalletError` replaces it (same error code, so clients that react to it
  still do): the wallet is the owner's to see, not the member's whose call was
  refused. It and the helpers named below live in the platform's
  `service/org_wallet.go` — the billing package at the repository root, not
  this directory's `service/`.
- **A wallet that cannot be charged refuses the key outright.** `SpendOf`
  answers `ErrNoWallet` when the holder is no longer a member, or when the
  owner's account is disabled or deleted; the request is a 403 before anything
  is spent. Disabling the owner's account is therefore how the platform
  suspends a whole organization. The same goes for missing organization
  tables (rule 4): an organization key is refused, a personal key is served.
- **A task remembers who paid.** A video is paid when it is submitted and
  settled or refunded minutes later by the poller, with no request to ask.
  The platform's `model.TaskPrivateData` keeps the request's `OrgSpend`, and
  `taskWalletUserId` reads it back, so the refund reaches the company
  wallet — not the member, who never paid — and its log line carries the same
  stamp.
- **The low-balance reminder goes to the owner and the admins**
  (`notifyOrgWalletLow`, with `WalletWatchers` from here), each through their own
  notification settings and in their own language, and never to the member
  whose request crossed the line. "Low" is the owner's own warning threshold,
  or the platform's default. It runs after settlement, off the request path.
  Auto top-up likewise looks at the owner's settings.
- **Two entrances are closed rather than billed wrongly**
  (`controller/org_wallet.go`):
  - the **playground endpoint** (`POST /pg/chat/completions`) refuses every
    organization account, the owner's too. It sends requests with no key, on
    the caller's own balance: a member's spending there would never reach the
    company's reports, and the owner's would leave the wallet with nothing to
    show for it. The playground page is gone from the console already. ⚠️ The
    **key self-check page** (`/keys/test`, `web/default/src/features/keys/test`)
    is still built on this endpoint, so the console offers it to no
    organization account — not on the keys page, not at the end of the setup
    guide, not from the welcome page — and sends one that opens it back to its
    keys;
  - the **Midjourney proxy** (`/mj/*`) refuses organization keys. Its task
    table has nowhere to record who paid, so a failed task would be refunded
    into the holder's own account.
- **The console hides a member's own balance.** `GET /api/user/self` carries
  `org_id` for a member (and nothing new for a personal account), so the
  console knows from the first page that the balance in that answer is not
  what the member's keys spend. `features/org/hooks/use-wallet-view.ts` is
  the one decision — `own` for a personal account and for the owner,
  `company` for every other member, `pending` until a member is known to be
  the owner or not — and every place that shows a balance, offers a top-up or
  nudges about credit asks it. The welcome page after sign-up goes further:
  a member gets an introduction of their own there (`MemberIntro` in
  `web/default/src/features/welcome`) — who pays, where keys come from, and
  the steps that are true for someone who is handed their keys and never
  sees a key's value — in place of the personal one. This is presentation
  only: nothing on the
  backend stops a member from topping up their own, unused, account.

### Where the branch sits in upstream files

Every one of these is a line or two in a file that comes from upstream. A
rebase that drops one silently breaks billing for organization keys, and the
tests named beside it are what would notice.

| File | What is there | Guarded by |
|---|---|---|
| `middleware/auth.go` (`TokenAuth`) | `attachOrgSpend` for a key with `org_id` | `router` `TestOrgWallet_ThroughTheRealGateway` |
| `relay/common/relay_info.go` | the `OrgSpend` field, filled in `genBaseRelayInfo` | `service` `TestOrgWallet_TheRequestCarriesWhoPays` |
| `service/billing_session.go` | `wallet_only` for organization keys; `WalletUserId()` in `tryWallet` | `service` `TestOrgWallet_*` |
| `service/billing.go` | `orgWalletError` around the pre-consume error; `noteOrgKeyRefused` before it, which raises the warning for a key refused for its quota | `service` `…AnEmptyCompanyWalletRefuses…`, `…AKeyRefusedForLackOfQuota…` |
| `service/quota.go` | `WalletUserId()` in `PostConsumeQuota` and `PreWssConsumeQuota`; the reminder's branch | `service` `TestOrgWallet_*` |
| `service/text_quota.go` | `MaybeAutoTopup` on `WalletUserId()` | **nothing** — it needs Redis and Stripe to observe |
| `service/task_billing.go` | `taskWalletUserId`; `OrgSpend` on the two task log lines | `service` `…ATaskIs…` |
| `model/task.go`, `controller/relay.go` (`RelayTask`) | the task keeps `OrgSpend` | `router` (the video task) |
| `model/log.go` | `stampOrgFromRequest`, `stampOrg` | `service`, `router` |
| `controller/relay.go` (`RelayMidjourney`), `controller/playground.go` | the two refusals | `router` |
| `controller/user.go` (`GetSelf`) | `org_id` for a member | `controller` `TestOrgSelf_TheProfile…` |
| `model/log.go` (the `Log` struct) | the index `idx_logs_org_created` on `org_id` and `created_at`, which the alert scan and the reports read by | `internal/org/service` `TestPlatformTables_CarryTheOrgColumnsAndIndexes` |
| `main.go` | `service.StartOrgAlertTask()` — without it no warning or alert is ever raised | **nothing** — a test that started the task would have it running under every other test |

## Warnings and alerts

Two kinds of thing an organization is told about its keys (PRD §4). Both are
rows of `org_alerts`, and both are **only ever told**: nothing here stops a
request (PRD D13).

- a **warning** — a key has used a share of what it was given: of its quota,
  or of the requests it may make in a month (`rule` `quota`, `monthly`);
- an **anomaly** — a key is being used unlike before: it spent far more
  within a day than it usually does, it spent a lot outside working hours, or
  it was used from an address it had not been used from (`spike`, `offhours`,
  `new_ip`).

What to know before changing any of it:

- 🔴 **It is a background task, not a hook on the request.**
  `service.StartOrgAlertTask` — the platform's `service/org_alerts.go`,
  started from `main.go`, on the master node only — runs a pass a minute:
  `ScanAllowances` every time, `ScanAnomalies` every ten minutes, then the
  sending. Nothing on the relay path knows it exists. The PRD's first sketch
  hung the warning off the settlement of a request. It was not built that
  way: a key's spending reaches its row seconds after the request
  (`BATCH_UPDATE_ENABLED`), so a check made at settlement reads the numbers
  from before it; and requests are settled in several places — chat, audio,
  realtime, video tasks, refunds — that a scan of the keys covers without a
  line in any of them. The price is that a warning is up to a minute late.
- **A key is warned once per level and cycle** (`ScanAllowances`,
  `withoutRepeatedWarnings`). The levels are the organization's
  (`AlertSettings.WarnAt`; 80 and 100 by default). A key is told about the
  highest level it has reached, and never again about that one or a lower one
  in the same cycle. The cycle of the monthly limit is the calendar month.
  The cycle of the quota is **what the key was given in all** — `used_quota +
  remain_quota`, a sum that spending and refunds leave alone and that changes
  when someone gives the key more. That sum is also what the share is taken
  of, the way the personal keys page draws its bar.
- **The monthly limit counts requests, not money.** It is the gateway's own
  counter (`internal/quota`, in Redis or in memory), read through
  `quota.MonthlyUsed`. The scan is handed a function for it (`MonthlyUsage`),
  so this package needs no Redis.
- **Which keys a pass looks at**: the ones used since the pass before
  (`tokens.accessed_time`, which every settlement writes), and every
  organization key on the first pass after a start — that is what catches up
  after downtime. A row changed without the key being used is looked at when
  the key is next used.
- **A request in flight counts with what it reserved.** The gateway takes a
  request's estimated cost off the key up front and puts the difference back
  at settlement, so a pass that falls in between sees a share slightly ahead
  of the truth, and may warn one request early.
- **A key can stop short of 100%, and that is warned about where it happens**
  (PRD D45). With less than ten dollars left the gateway refuses a request
  when what is left does not cover its estimated cost
  (`pre_consume_token_quota_failed`), so a client that asks for a large
  `max_tokens` is turned away with a few percent of the quota unused — and no
  scan would ever know: nothing was spent. `service.PreConsumeBilling` tells
  `noteOrgKeyRefused` about every refusal, and for an organization key
  refused for its quota that calls `WarnRefused`, off the request's goroutine.
  It raises the quota warning at level 100 with `Needed` in the detail — the
  same rule and cycle as the scan's, so the two never tell a key's people
  twice — and the next pass sends it, worded as a refusal. The one line on
  the request path raises an alert after the request is over; it still stops
  nothing.
- **The anomaly rules read the usage log** (`ScanAnomalies`). "A day" is the
  last 24 hours and "usual" is the 7 days before them, per key **under its
  current holder** — lines written by whoever held the key before are
  somebody else's habits. A key says nothing until it has been with its
  holder for 7 days, counted from when it was made or from its last
  `key.assign` / `key.reclaim` record in the audit log; rotating it does not
  start that over. The same key and rule speak once in 24 hours, whatever
  became of the alert.
  - `spike`: the day's spending is at least the floor and more than
    `SpikeMultiple` times the daily average. A key that usually spends
    nothing is a spike at the floor.
  - `offhours`: off until the organization sets working hours. Then: spent
    outside them at least the floor and at least `OffHoursPercent` of the
    daily average. Usage is summed in 15-minute buckets, and each bucket is
    read on the organization's own clock (`WorkHours.Covers`).
  - `new_ip`: an address not among those the key was used from in the 30
    days before. The first address an idle key is used from counts.
- **An organization key's usage always records the address it came from**
  (`model.Log.stampOrgFromRequest`). A personal key records it only when its
  user switched that on; without it `new_ip` has nothing to read.
- **The usage log is indexed by `(org_id, created_at)`**, for these scans
  and for the reports. An anomaly pass reads an active organization's last 8
  days of lines once, and its last 30 for the keys that have an address to
  judge. That is the query to replace with a rollup when an organization's
  log grows into millions of lines a month (PRD §8 leaves pre-aggregation out
  of v1).
- **Who is told** (`DigestsFor`): the owner and the admins about everything;
  a key's holder about the warnings on that key and not about anomalies — if
  the key leaked, its holder is not who to ask; a service account and a
  disabled or removed holder nothing. Each through their own channel and in
  their own language, like the wallet reminder. `service/org_alerts.go` does
  the wording; names and addresses are escaped where the channel reads HTML.
- **Alerts leave together.** The platform lets a user have
  `NOTIFY_LIMIT_COUNT` notifications of one type per
  `NOTIFICATION_LIMIT_DURATION_MINUTE` and drops the rest without a word, so
  a notification per alert would lose some exactly when several keys act up.
  An alert waits in `org_alerts` with `notified_time = 0`, and an
  organization is sent at most one notification per gap (`orgAlertDigestGap`:
  six minutes at the defaults) that lists whatever is waiting. The
  notification type is the task's own (`org_alert`), so these never compete
  with the low-balance reminder. An alert that could not go out within a day
  stays in the list and is not sent.
- **A notification ends with a link to the alert list** (`orgAlertListLink`:
  the alerts section of the reports page), whoever it goes to. The list shows
  each member what is theirs to see, so a holder told about their own key
  lands on the warnings about their keys.
- **The alert list** (`ListAlerts`) answers every member, and how much is
  part of the query. A role that reads alerts gets all of them, or — limited
  to departments — the ones stamped with its departments: the holder's
  department when the alert was raised, so, like a usage line, an alert stays
  with that department when its holder moves. Everyone else gets **the
  warnings on the keys they held** (D47): `user_id` on the alert is them and
  `rule` is one of `WarningRules`. That is what they are notified of, and no
  more — an anomaly on their key is not theirs to read, for the reason it is
  not sent to them. The alert stays with whoever held the key when it was
  raised; handing the key on does not hand on its history. Marking an alert
  handled or a false alarm (`HandleAlert`) is the owner's and the admins',
  and is audited; it changes nothing about the key and nothing about the
  sending.
- **The settings** (`organizations.alert_settings`, JSON; `AlertSettings`)
  are the warning levels, the two multiples, the floor and the working hours.
  Empty means `DefaultAlertSettings`. A change is audited with both versions.

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
- **Reading it** (`ListAuditLogs`) is by page, newest first, narrowed by an
  `AuditFilter`: part of the name of whoever did it — display name or
  username, in any case, a removed member's included, with `%` and `_` taken
  as characters — the kind of thing it was done to, and a period. The total
  it answers with counts what the filter lets through.
- **Every record in the list names its target** (`AuditLogView.Target`). A
  record about a member holds what changed and not who they are, so members
  are named from the accounts, as they are called today; an alert is named by
  its key; anything else by the name its record carries, the later one first.
  An invite link has none, and the page calls it by what it is.

## Usage reports

`ReportUsage` sums what the organization's keys spent over a period, by
department, member, key or model (PRD §6). It has no table of its own: it
reads the usage log, where every line of an organization key says who used
which key on which model for how much, and carries the organization and the
department of that moment (see "The company wallet").

- **How much the caller is shown is part of the query, not a filter on the
  answer.** A role that reads usage across the organization gets everything; a
  department-scoped one gets `department_id IN (the departments it reaches)`;
  everyone else gets `user_id = themselves`, because the engine lets every
  member see their own usage. The answer says which of the three it was
  (`scope`). So the endpoint has no gate of its own to forget — it cannot be
  asked for more than it filters by.
- **Naming a department is asking about that department.** It takes
  `usage.read` over it, and outside the caller's reach it is `ErrForbidden` —
  a 403, never an empty report. That holds for a member's own department too:
  a department's usage is everybody's in it. A department of another
  organization has no usage here, and whoever may name any department gets an
  empty report for it.
- **Usage counts where it was spent.** A line belongs to the department its
  user sat in at that moment, so moving someone neither brings their past
  usage to their new manager nor takes it from the old one, and one member
  can have usage in two departments.
- **Money is what left the company wallet.** A refund line (`type` 6: a task
  that failed, or was settled for less than was reserved) is taken off, and is
  not a request. The platform's own figures — `SumUsedQuota`,
  `users.used_quota`, the dashboard — add what was consumed and never take a
  refund off, so they can read higher than the report for the same period.
- **Rows are called what they are called today**, by id: a department or a
  key renamed since shows under its new name, once. What is gone is still
  named and marked (`gone`) — a dissolved department, a deleted key, a
  removed member — which is why the lookups are `Unscoped`. A service account
  is marked as one.
- **A key's row says who used it** (`used_by`), the biggest spender first: a
  key that changed hands has two names, and three keys called "Claude Code"
  are told apart by theirs. The names come from the same lines as the
  numbers, so a manager is never told about someone outside their
  departments.
- **The period** is two Unix timestamps, both included; either can be left
  out. A period that ends before it starts, or a grouping that is none of the
  four, is `ErrInvalidUsageQuery` — invalid parameters, not a 403.
- **Nothing is paged.** A report has one row per department, member, key or
  model that spent anything, sorted by what it spent; the page shows the first
  hundred and the rest on request.
- **`departments` in the answer** are the departments the caller may narrow
  the report to — all that exist, or the ones they manage. It is there so that
  a role holding nothing but `usage.read` needs no other endpoint to fill the
  filter.

### The page

"Reports & alerts" (`/org/reports`, `web/default/src/features/org/reports.tsx`)
is one page with three sections — usage, alerts, the audit log
(`lib/reports.ts`). Usage and the audit log take their primitive,
`usage.read` and `audit.read`. Alerts are every member's section (D47): what
`alert.read` decides is how much of the list they are sent. So every member
of an organization has the page — the sidebar entry, and the route's gate
`requireOrgMember` — with the tabs that are theirs. The section is in the
address (`?section=alerts`), which is what a notification links to.

- **Staff get the alerts tab alone**, with the warnings on their own keys and
  a sentence that says so (`seesOwnAlertsOnly`). Their own calls are on the
  usage logs page, as before: the usage endpoint still answers them — with
  their own usage, which is what "staff see only their own" rests on — but
  the page gives them no usage tab.
- **Marking an alert and the settings form are the owner's and the admins'**
  (`canManageOrg`), like everything else an inherent power covers. The form
  checks every field itself, because the backend refuses a bad request with
  one message for all of them (`org.alert_settings_invalid`).
- **Two things the form converts**: the floor is typed in the console's
  currency and stored in quota units; the hours of a working day are clock
  times on the form and minutes after midnight on the wire, where a day that
  ends at 00:00 is minute 1440.
- **An audit record is put into words on the page** (`lib/audit.ts`): the
  action, and what changed — only what differs between "before" and "after",
  with a list shown as what was added and what was taken out. A field the
  page has no words for shows under its own name; none is dropped.

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
  accounts, remove a member or a service account.
- Invite links (`/sign-up?org_invite=<code>`): create, list, revoke, a public
  preview, and `service.JoinByInviteTx`, which sign-up calls the same way it
  calls `CreateForOwnerTx`.
- Roles: the five presets, custom roles (create, change, delete), role packs,
  the permission catalogue.
- Organization keys: list, create (for a person, a service account, or
  parked under the owner), change, rotate, freeze, unfreeze, delete; the two
  policy templates, and a list of models picked by hand; the members a key
  can be made out to, for the form to search and filter; handing a key to
  another member and taking it back; a member's own keys by purpose, for the
  console.
- The company wallet: an organization key spends the owner's balance, its
  usage is stamped with the organization and the member's department, and the
  owner and the admins hear when the wallet runs low.
- The personal endpoints closed to members: creating a key, deleting one's own
  account.
- The audit log: written by every management write; read by page, narrowed
  by who did it, by what kind of thing and by when.
- Warnings and alerts: raised and sent by a background task, and raised on
  the spot for a key refused for lack of quota; the alert list — every
  member's, as far as it is theirs — marking an alert, and the settings, over
  four endpoints.
- Usage reports: one endpoint, four groupings, a period, and a department to
  narrow to; money and requests, no token counts (PRD D44).
- The console's "Reports & alerts" page: the usage report, the alert list
  with its marks and its settings form, and the audit log.

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
  itself is only exercised against a real Redis. The controller tests'
  `forEachOrgDialect` switches the flag off too and — unlike the other
  globals it sets — does not put it back: a handler can leave a goroutine
  behind that reads it after the test is over (a sign-up rewarding its inviter
  does), and writing it then is a data race for the detector CI runs with.
- **A soft-deleted member row keeps its `org_id`, role and department.** Any
  query on `users` that is meant to see members has to go through GORM's
  default scope (which hides deleted rows) — `Unscoped()` belongs only where a
  name is looked up for something that outlived the account.
- **Someone who joins by invite gets no starter key** (PRD D16). Their console
  mode is not stored for them at sign-up, but it has a default (PRD D42):
  until they choose, a member works in the Advanced console. The console
  decides that in one place — `consoleModeFor` in
  `web/default/src/features/simple/lib/mode.ts`, which treats an account with
  no stored choice and an `org_id` in its profile as Advanced — and the
  welcome page offers a member `team`. The founder's mode is stored: sign-up
  saves them as a `team` persona so they land in the Advanced console (PRD
  D24).

## Not here yet, and where it will hook in

- **Coming back.** An invite link works at sign-up only, and a removed
  member's username and email stay taken — so there is no way to bring the
  same account back, and none to move an existing personal account into an
  organization. Not in v1.
- **A report over time, a chart, an export.** `ReportUsage` groups by one
  column and answers totals for a period; the page shows a table. The chart —
  spend by department and by model over days, weeks, months or years — is the
  next card of the PRD (P10): a second query over the same lines, bucketed by
  `created_at`. A CSV for the finance role would be another handler over the
  same query.
- **Usage from before the stamp.** Lines written before P7 carry no
  organization and are in no report; none exist in production.
- **A summary table.** Reports read the usage log directly, through the
  `(org_id, created_at)` index (PRD §8). `quota_data` — the platform's hourly
  aggregate — has no key or department on it; adding them is the step to take
  when a company's log grows past what one grouped scan answers quickly.
- **Midjourney for organization keys.** Refused until its task table can say
  who paid for a task.
- **Stopping a member from topping up their own account.** The console
  offers them no way to; the payment endpoints do not ask.

## Testing

`orgtest.ForEachDialect` runs a test on SQLite always, and on a real server
when its DSN is set — a scratch database is created and dropped per test:

```bash
go test ./internal/org/...
go test ./controller/ ./router/ ./service/ -run TestOrg

# the same suites against a real server
export TEST_POSTGRES_DSN=postgresql://root:123456@localhost:5432/postgres
export TEST_MYSQL_DSN='root:123456@tcp(localhost:3306)/mysql?charset=utf8mb4&parseTime=True'
```

SQLite accepts things a real server rejects (an over-long `char(32)`, for one),
so run the PostgreSQL pass before trusting a schema change — production is
PostgreSQL. Tests outside this directory are named `TestOrg…`; both CI
workflows select them by that prefix.
