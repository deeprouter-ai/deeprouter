# Tenant Onboarding (V0)

> How to create a tenant in DeepRouter and hand off an API key to a consumer product (Airbotix Kids, JR Academy, etc.).
>
> V0 uses the NewAPI admin UI directly — we don't yet have a dedicated multi-tenant onboarding flow. That comes in Phase 2 of [`PLAN.md`](../PLAN.md) once we wire policy+billing into the relay path.

---

## Concept recap

A **tenant** in DeepRouter is just a **User** in NewAPI's data model. We added 4 Airbotix-specific columns to `users` ([`model/user.go`](../model/user.go)):

| Column | Type | Purpose |
|---|---|---|
| `kids_mode` | `bool` | When `true`, the relay applies all hard kid-safe constraints (PRD §6.4-pre) |
| `policy_profile` | enum `passthrough` / `adult` / `kid-safe` | Behavioural profile (overridden by `kids_mode=true`) |
| `billing_webhook_url` | string | Where to POST per-request billing events |
| `custom_pricing_id` | string | Reference to a custom pricing table (V1+) |

---

## Step-by-step: create the `airbotix-kids` tenant

### 0. Boot DeepRouter

```bash
cd ~/Documents/sites/deeprouter-ai/deeprouter
docker compose up -d
open http://localhost:3000
```

### 1. Register the first admin

The very first registered account becomes the **root admin**. Use a real email (you'll need it for password resets).

```
http://localhost:3000 → Register → email + password
```

### 2. Create the tenant user

```
Admin UI → Users → "Add new user"
  username:   airbotix-kids
  email:      ops+airbotix-kids@kidsinai.org
  password:   <generate strong>
  role:       1 (common user)
  quota:      0 (no internal NewAPI quota; we use the policy layer)
```

Save.

### 3. **Phase 1 task pending — set Airbotix-specific fields**

> ⚠️ As of 2026-05-12 the admin UI does **not yet** expose the 4 Airbotix fields. Engineer task to add them: [`docs/tasks/phase-1-admin-ui.md`](./tasks/phase-1-admin-ui.md).
>
> Until that's done, set the fields directly via SQL:

```bash
docker compose exec postgres psql -U root -d new-api -c "
UPDATE users
SET kids_mode = true,
    policy_profile = 'kid-safe',
    billing_webhook_url = 'https://api.kidsinai.org/internal/deeprouter/billing'
WHERE username = 'airbotix-kids';
"
```

Verify:
```bash
docker compose exec postgres psql -U root -d new-api -c "
SELECT id, username, kids_mode, policy_profile, billing_webhook_url
FROM users WHERE username = 'airbotix-kids';
"
```

### 4. Configure upstream providers (Channels)

Each provider (Anthropic, OpenAI, DeepSeek, ...) is a **Channel**. Add at least one OpenAI channel for first tests:

```
Admin UI → Channels → "Add channel"
  type:    OpenAI
  name:    openai-prod-1
  key:     sk-... (your real OpenAI key with under-18 zdr-eligible billing)
  models:  gpt-4o-mini, gpt-image-1
  group:   default
```

Save.

### 5. Create the API key (Token) for the tenant

The API key is what `kidsinai/platform-backend` will send in its `Authorization: Bearer …` header.

```
Admin UI → Tokens → "Add token" (while logged in as the airbotix-kids user!)

  - Sign out
  - Sign in as airbotix-kids
  - Tokens → Add
    name:        platform-backend-prod
    quota:       unlimited (or per-month cap if you want)
    expiry:      no expiry
```

Copy the generated `sk-...` value. **This is what goes into `kidsinai/platform-backend/.env`'s `DEEPROUTER_API_KEY`.**

### 6. Smoke test the tenant

From the `airbotix-kids` perspective:

```bash
TOKEN=sk-...   # from step 5

curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

Should return a standard OpenAI chat completion response.

> ⚠️ Once Phase 2 relay wiring lands, this request will also:
> - Refuse `gpt-3.5-turbo` (not on kid whitelist)
> - Strip identifying metadata from the outgoing payload
> - Inject `store: false` for OpenAI
> - Inject child-safe system prompt
> - POST a billing event to `billing_webhook_url`

---

## Repeat for additional tenants

Repeat steps 2-5 for each consumer:

| Tenant username | kids_mode | policy_profile | billing_webhook_url |
|---|---|---|---|
| `airbotix-kids` | `true` | `kid-safe` | `https://api.kidsinai.org/internal/deeprouter/billing` |
| `jr-academy` | `false` | `adult` | `https://api.jiangren.com.au/_/deeprouter/billing` |
| `external-x-test` | `false` | `passthrough` | `https://test.example.com/billing` |

---

## Production handoff

When deploying to `api.deeprouter.ai`:
1. Repeat the same flow against the production instance
2. Save the production token in your secrets manager (Doppler / 1Password / aws-sm)
3. Hand off the token via secure channel (1Password share, encrypted email) — never plain Slack/WeChat
4. Document the rotation cadence (recommend: 90-day rotation, V1+ automate)

---

## Common errors

| Symptom | Cause | Fix |
|---|---|---|
| 401 invalid token | Token regenerated or expired | Recreate token, update consumer's `.env` |
| 404 model not found | Model not on any active channel | Add model to a channel's `models` field |
| 200 but webhook never fires | `billing_webhook_url` empty OR Phase 2 wiring not done | Check column value; check `git log` to see if billing dispatch is wired in relay |
| `kids_mode` constraints not applied | Phase 2 wiring not done | See [`PLAN.md`](../PLAN.md) Phase 2 |
