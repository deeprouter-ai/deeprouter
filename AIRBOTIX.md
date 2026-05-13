# Airbotix / Kids in AI — DeepRouter Fork Notes

> This file is **NOT from upstream** (`QuantumNous/new-api`). It captures DeepRouter-specific intent and customisation plan, separately from upstream's `CLAUDE.md` / `AGENTS.md` (which we keep clean for rebase).

## What this fork is

This is the production code repository for **DeepRouter** — an OpenAI-compatible multi-tenant LLM gateway. Forked from `QuantumNous/new-api` (32K stars, AGPL v3, very actively maintained).

DeepRouter is an independent product (not part of Airbotix). See [`docs/PRD.md`](./docs/PRD.md) for the full engineering PRD and [`docs/DESIGN.md`](./docs/DESIGN.md) for the UI design system. The business plan (`DeepRouter-BP.md`) lives outside this repo at `~/Documents/sites/jr-academy-ai/deeprouter-brand/`.

## License inheritance

**AGPL v3** (forced by upstream). Our public fork is intentional — we follow the Supabase / Plausible / Cal.com model: open source core + hosted SaaS + enterprise support contracts.

## What we customise (planned, not yet implemented)

We aim to **minimise core changes** to keep upstream cherry-picking sustainable. All Airbotix-specific code goes in dedicated directories:

| Path | Purpose |
|---|---|
| `internal/policy/` (new) | Per-tenant policy middleware: kid-safe system prompt injection, input/output filtering, content classifiers |
| `internal/billing/` (new) | Billing webhook dispatcher: POST to tenant-configured URL after each request |
| `internal/kids/` (new) | `kids_mode` enforcement layer: metadata strip, OpenAI ZDR injection, model whitelist |
| `web/default/` (upstream) | Admin UI — we only add fields to existing user form (policy_profile, billing_webhook_url, kids_mode, custom_pricing_id) |

**Database changes**: extend NewAPI's existing `users` table with 4 columns. No new tables, no schema rewrite.

## Local development

→ See [`DEV.md`](./DEV.md) for the 5-minute local quickstart.

## Development plan

→ See [`PLAN.md`](./PLAN.md) — phase-by-phase Week 0 → V0 launch with acceptance criteria, open decisions, and risk register. **This is the living plan; update it weekly.**

## V0 12-week plan

Week-by-week breakdown lives in [`docs/PRD.md`](./docs/PRD.md) §8. P0 deliverable: **OpenAI-compatible `/v1` endpoint working by Week 6** (it blocks `kidsinai/opencode` team).

## Tenants (V0)

| tenant_id | Source | Settings |
|---|---|---|
| `airbotix-kids` | Kids in AI platform | `kids_mode: true`, strict policy, Stars billing webhook |
| `jr-academy` | JR Academy (Lightman's other co.) | adult ed policy, JR's own billing metering |
| `external-x` | future SaaS customers | V2+ |

## Critical V0 features (must hit)

1. OpenAI-compatible `/v1/chat/completions`, `/v1/messages`, image/embeddings — all with cross-protocol conversion
2. `kids_mode` hard constraints (see DeepRouter PRD §6.4-pre)
3. Multi-key Provider Pool with token bucket (Anthropic Tier RPM workaround — DeepRouter PRD §5.5, §6.5)
4. Billing webhook with HMAC signature + retry + dead letter queue
5. Atomic per-tenant quota check

## Upstream sync

```bash
git remote -v          # origin = our fork, upstream = QuantumNous/new-api
git fetch upstream
git cherry-pick <commit>      # for individual bugfix
# OR merge: git merge upstream/main  (when divergence is small)
```

If divergence > 30% triggers D-DR9 (independent fork decision) — see PRD.

## Sister docs

- [`docs/PRD.md`](./docs/PRD.md) — engineering PRD (real plan) **[in-repo]**
- [`docs/DESIGN.md`](./docs/DESIGN.md) — UI / visual design system (Lovable-inspired) **[in-repo]**
- `~/Documents/sites/jr-academy-ai/deeprouter-brand/DeepRouter-BP.md` — business plan **[external, fundraising; "MVP backend exists" is aspirational]**
- `~/Documents/sites/kidsinai/planning/PROJECT.md` — master plan across all Lightman ventures **[external]**
