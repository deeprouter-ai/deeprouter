# ADR 0007 — `deeprouter-auto` re-picks inside the token's model whitelist

- **Status**: Accepted
- **Date**: 2026-08-26
- **Affects**: `middleware/smart_router.go` (resolution path of `deeprouter-auto`)

## Context

Measured on production (`deeprouter.co`, 2026-08-26): every `deeprouter-auto`
request from a token with `model_limits` enabled returned
`403 该令牌无权访问模型 <X>` — where `<X>` was a model the user never typed
(`claude-haiku-4-5-20251001`, `deepseek-v4-flash`). Three different prompts
resolved to two different models, so smart-router was alive and deciding; the
block was the token whitelist.

Root cause is an asymmetry between the two virtual-model paths in
`middleware/distributor.go`:

- **Simple-mode** (`deeprouter`, `deeprouter-coding`): token issuance writes
  the resolve targets into `Token.ModelLimits`, so the later whitelist check
  passes by construction.
- **`deeprouter-auto`**: smart-router picks from the **tenant's** catalog
  (`/internal/router-catalog` filters by `ab.Group != user.Group`,
  `controller/internal_catalog.go`), which is wider than any one token's
  whitelist — measured 77 tenant models vs 29 token models, intersection 25.
  The whitelist check then runs on the **resolved** name and 403s.

smart-router's own `filterByCatalog` is correct; it just answers a different
question ("can the tenant use it"), not "can this token use it". The gateway
cannot push the token whitelist into the catalog: the cache is per-tenant and
the 30s poller has no notion of "the current token".

## Decision

**Option C — re-filter after resolution, inside the gateway.** In
`resolveAutoModel`, after the sidecar answers (or the default fallback is
chosen), re-pick using exactly the check Distribute enforces later
(`FormatMatchingModelName` + `MatchModelLimit`):

1. keep the primary if the whitelist allows it;
2. otherwise the first allowed entry of the fallback chain;
3. otherwise the **cheapest chat-usable whitelisted model the request's group
   can serve** (per-call-priced models excluded — same set the router catalog
   skips; ties broken by name for determinism);
4. if nothing at all survives, return the pick unchanged and let Distribute's
   own 403 fire — the token can use nothing, and masking that only moves the
   error.

The stored fallback chain (`ContextKeySmartRouterFallback`) is filtered too:
`controller/relay_cross_model.go` retries through it, and a non-whitelisted
entry there would hit the same 403 on retry.

The adjustment is visible: `X-DeepRouter-Routed-Reason` gains a
`+token_whitelist` / `+token_whitelist_degraded` suffix, and
`X-DeepRouter-Routed-Model` always carries the final pick.

## Alternatives rejected

- **A — catalog by token scope**: explodes the per-tenant cache keyspace and
  the poller cannot know the current token. Also widens the cross-process
  contract for a gateway-local concern (Interface Segregation).
- **B — skip the whitelist for auto-resolved models**: changes a security
  boundary. The whitelist is the operator's statement of what a leaked or
  scoped-down token may spend money on; routing must live inside it, not
  around it.

C fixes both failure modes at once (sidecar down AND pick-outside-whitelist)
without touching the boundary: the final model always passes the same
`MatchModelLimit` call Distribute runs.

## Consequences

- Tokens with a whitelist now get smart routing **within** their whitelist;
  the routing quality degrades gracefully as the whitelist narrows.
- The degradation lookup (`GetGroupEnabledModels`) hits the DB only on the
  rare path where neither primary nor chain passes.
- The whitelist re-check lives in `middleware/smart_router.go` (fork-owned),
  not in upstream `distributor.go` — the rebase surface is unchanged
  (ADR 0006).
- A token whose whitelist intersects nothing the group serves still 403s;
  that is a token misconfiguration and hiding it would be worse.
