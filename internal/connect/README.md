# `internal/connect` — one-click CLI setup

Issues the one-time tokens behind the key page's **一键配置** block, and redeems
them for a ready-to-run install script.

Spec: [`docs/tasks/one-click-cli-setup-prd.md`](../../docs/tasks/one-click-cli-setup-prd.md)
§2.1 (page), §4.1 (token), §5 (security).

## Why a token instead of the key

The command is *meant* to be copied. It lands in clipboards, terminal
scrollback, screen recordings and screenshots, and gets pasted into group chats
when someone asks for help. So what travels through all of that is a token that
is worth nothing once used:

- **single use** — spent on redemption, whether or not the caller then succeeds
- **30 minutes** — `GrantTTL` (was 15; widened 2026-09-12 with the downloadable
  installers, PRD §11 D4 — download, find the file, double-click is a slower path
  than pasting a command)
- **bound to one key** that the issuing session already proved belongs to the caller

The API key itself is read from the database at redemption and injected into the
script body. It never appears in a URL.

## Surface

| Route | Auth | Purpose |
|---|---|---|
| `GET /api/connect/tools` | none | Which tools this build can configure |
| `POST /api/connect/token` | session (`UserAuth`) | Mint a token for one of the caller's own keys |
| `GET /i/:token` | **the token itself** | Redeem for the script |
| `GET /d/:token/deeprouter-setup.cmd` · `…/deeprouter-setup.sh` | none (see below) | Download the double-click installer (PRD §11) |
| `GET /d/:token/deeprouter-setup-<token>-<host>.pkg` | none (see below) | Download the signed macOS installer (PRD §11.7) — dark unless `DEEPROUTER_PKG_FILE` points at a built artifact; assets in [`macos-pkg/`](./macos-pkg/) |

The two halves authenticate differently on purpose. Issuing needs a session
because that is the step deciding *whose* key this is. Redeeming cannot have one
— it runs in the user's terminal via `curl | sh` — so there the token is the
credential.

`/i/:token` sits at the root rather than under `/api` because it has to fit in a
command someone retypes from a screenshot. It is registered before the web
router so the SPA fallback does not swallow it.

### The installer download is a text generator, not a redemption

🔴 `/d/…` **must not** go through `Redeem()`. That call claims the grant and
destroys it, so a download routed through it would hand the user a file that
reports "expired" the instant they double-click it — the token has to survive
until the file runs. So this endpoint touches the grant store not at all: it
shape-checks the token (which is what keeps an arbitrary path parameter out of
the generated file) and writes it into a wrapper. An unknown or spent token still
downloads; `/i/:token` is where expiry gets explained, in a script that prints why.

The file is a wrapper and nothing more: 🔴 it carries the token and **never the
key** (PRD §11.4), and it fetches the same script the one-line command fetches,
so every fix to `templates/` reaches the download path for free. The filename is
in the path as well as in `Content-Disposition` because a client that ignores
that header still has to save something Windows will run when double-clicked.
Which platform's file to serve comes from the **requested filename**, not the
User-Agent every other handler here sniffs — the page shows one button per OS, so
a human already chose, and setting up a Windows machine from a Mac is a real case.

## The authorization boundary is one line

```go
key, err := model.GetTokenByIds(req.KeyID, userID)   // scoped by BOTH
```

A key id belonging to somebody else is simply not found. This is the only
privilege-escalation path the feature has, and it is covered three ways in
`handler_test.go` — issuing for another user's key, a grant forged to name
another user's key id, and a check that neither response contains the other
key's value.

## Storage

`Grant` holds the key's **row id**, not the key. So a grant sitting in Redis is
not itself worth stealing, and a key regenerated in the meantime is not served
stale.

Two stores, picked at call time by `common.RedisEnabled`:

- **Redis** — `SET` with TTL; the claim is `GET` then `DEL`, using DEL's
  returned count to decide the winner. That is atomic without needing `GETDEL`,
  so it works on Redis older than 6.2.
- **in-memory** — a mutex-guarded map, swept on write. Losing grants on restart
  is fine: they are short-lived and worthless after use.

## ⚠️ The script is a placeholder

`script.go` renders a **real, runnable** script, but not the real *installer*.
Detecting each tool, merging its config, verifying and uninstalling is
[P2](../../../docs/adlc/tasks/one-click-setup-script-task.md). What is here
prints the server, the chosen tools and a pointer to the guides.

It is deliberately not a stub that errors: PRD §5.4 promises a two-step form
(`curl -o` → read it → `sh`) that actually works, and a user who gets a broken
script assumes they did something wrong.

## 🔴 No default base URL, ever

There is no fallback address anywhere in the template. More than one independent
deployment exists, with separate databases and keys that do not work across
them; a wrong address fails as a run of 401s that never mentions the address
(PRD §0.1 F2). The value comes from the issuing instance's own
`system_setting.ServerAddress`, and `TestConnectScript_HasNoHardcodedBaseURL`
fails if a hostname is ever baked in.
