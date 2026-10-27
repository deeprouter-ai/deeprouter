# `internal/mcp` — the MCP endpoint for AI tools

`POST /mcp` speaks the [Model Context Protocol](https://modelcontextprotocol.io)
(Streamable HTTP, stateless, JSON answers). An AI tool adds it once — Claude Code,
Codex, Cursor — and can then ask DeepRouter itself which models its key can use,
how to call each one and how to set up a client, instead of a person pasting docs.

Spec: meta-repo `docs/deeprouter-mcp-prd.md` (D4: MCP only tells the AI how to use
the API; it never generates). Card: `docs/adlc/tasks/deeprouter-mcp-server-and-tools-task.md`.

## The five tools — all read-only

| Tool | Answers | Comes from |
|---|---|---|
| `list_models` | the models this key can use: purpose, endpoint, one-line price; the public catalog when no key is sent | the `/v1/models` handler + the `/api/pricing` handler |
| `get_model` | how to call one model: URL, auth, required params, a minimal body, the submit → poll → download steps for video; price; whether the key can use it | the same two handlers + `recipes` in `catalog.go` |
| `get_connection_config` | a client's exact settings; `mcp` for adding this server to a client | the published guides' "Facts for AI assistants" block, verbatim |
| `check_key` | does the key work, and if not why; its limit, expiry, group, whitelist, who pays | the key's record + `middleware.TokenAuth` |
| `get_started` | what DeepRouter is, base URLs, sign-up, which tool for what | static |

Nothing here reaches the relay, so no tool can spend money. No answer contains the
caller's key or an upstream channel key.

## The answers are the API's own

The tools do not re-implement the gateway. A key is put through
`middleware.TokenAuth` itself, and the model list and catalog come from running the
`ListModels` and `GetPricing` handlers on a *shadow* gin context (`caller.go`): built
on the real engine, with the request's headers, writing into a buffer instead of the
response. So an IP-restricted key, a key in a group it may not use, a kids account
or an organization without a wallet is treated exactly as `/v1/models` treats it,
and no upstream file had to change.

- The key is looked at only when a tool needs it, once per request: `initialize`,
  `tools/list` and notifications never touch the database.
- A bad key is a **status**, not an HTTP 401: `no_key`, `invalid`, `disabled`,
  `expired`, `exhausted`, `refused` (with the gateway's own reason), `error`. An MCP
  client that gets a 401 starts an OAuth flow we do not offer; a sentence the AI can
  pass on is what the user needs. A header still holding `${ANTHROPIC_AUTH_TOKEN}` — the
  keys page's command, run where that variable was never set — is `no_key` with the
  variable named, not `invalid`.
- Prices are what the key pays: the model's ratio × 2 (USD per 1M tokens) or its
  per-request price, times the group ratio — `default` is 1.2 today. A visitor sees
  the cheapest group selling the model, marked "from". A model the catalog leaves
  out — switched off on the Models page while a channel still serves it, so
  `/v1/models` lists it and the relay bills it — is priced from the settings the
  relay bills by (`pricingFromSettings`), not reported as unpriced.
- `get_connection_config` reads `web/default/dist/docs/integrations/*.md` from the
  embedded build, the same files `/docs/integrations/<client>.md` serves.

## Known gaps

- **Per-request media prices are base prices.** Some video and image providers are
  billed by duration, resolution or image count (`relay/channel/task/*`
  `EstimateBilling` — MiniMax-H3 is per output second), and the price table does not
  record which. `get_model` says so in `price_note` instead of inventing a per-clip
  cost.
- **`docs_url` is `unknown`.** Provider documentation links live only in the
  console's import presets (`web/default/src/features/models/lib/model-presets.ts`).
- **The URLs are deeprouter.co's**, like `discovery.GuideURL` and the guides
  themselves; a second deployment on another domain answers with them too.
- **`/v1/models` lists no endpoint types until the pricing catalog has been built**
  once after start-up (`model.GetModelSupportEndpointTypes` reads what
  `updatePricing` fills). The tools build the catalog first, so their answers are
  complete from the first call.

## Tests

`go test ./internal/mcp/` — the end-to-end tests connect with the SDK's own client
over HTTP to a real engine on a throwaway database (SQLite; PostgreSQL too when
`TEST_POSTGRES_DSN` is set), next to the real `/v1/models`.
`router/mcp_router_test.go` checks the route as `SetRouter` registers it.
