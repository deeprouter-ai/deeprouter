# Google Gemini CLI → DeepRouter

Point Google's [Gemini CLI](https://github.com/google-gemini/gemini-cli) at DeepRouter so its
requests run through your DeepRouter account instead of going straight to Google.
Gemini CLI talks the **native Gemini API**, and DeepRouter offers a matching Gemini‑compatible
endpoint — so this is just two environment variables, no coding required.

> **TL;DR** — set two environment variables:
> ```bash
> export GOOGLE_GEMINI_BASE_URL=https://api.deeprouter.co
> export GEMINI_API_KEY=sk-...your-deeprouter-key...
> ```

---

## Facts for AI assistants

**Not sure what this is?** That's fine — it is written for an AI, not for you.
Copy the whole block below, paste it to any AI assistant (ChatGPT, Claude, whichever
you use) together with a sentence like *"walk me through setting this up"*, and it
will tell you exactly where each value goes. Everything above and below this block is
the same thing written for a human.

```yaml
# Verified against the live DeepRouter gateway on 2026-08-28. Copy these values exactly.
tool: gemini-cli
api_protocol: Gemini
base_url: "https://api.deeprouter.co"
base_url_warning: "HOST ONLY - do not add /v1beta. Gemini CLI appends it; writing it yourself gives 404 Invalid URL (POST /v1beta/v1beta/models/...). Goes in GOOGLE_GEMINI_BASE_URL."
endpoint_called: "POST /v1beta/models/<model>:generateContent"
auth_header: "x-goog-api-key: <your sk- key>   # or ?key=<your sk- key>"
model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/resources/gemini-cli"
```

## Why route Gemini CLI through DeepRouter

- **One key, every model.** Gemini, plus many other models in the catalog — reachable through
  the same endpoint, with automatic model routing and fallback.
- **Smart routing.** DeepRouter picks the right model and channel per request and fails over
  automatically when an upstream is down.
- **Billing in one place.** Your team's usage, spend, and logs all live in the DeepRouter console.

---

## One-click setup (recommended)

You do not have to edit any config file by hand. One line in a terminal does all
of it — and it configures only the tools you tick, skipping anything you do not
have installed.

1. Open **API Keys** in the DeepRouter console.
2. Under **One-click setup → Terminal tools**, tick **Gemini CLI**.
3. Copy the command for your system and paste it into a terminal:
   - macOS / Linux (also WSL and Git Bash): `curl -fsSL <the address shown> | sh`
   - Windows (PowerShell or Terminal, **not** cmd): `irm <the address shown> | iex`
4. Then run `gemini`. Open a **new** terminal first. If it asks how to sign in, pick **API key** — choosing the Google login bypasses DeepRouter entirely.

> **What travels in that command is a one-time token, not your key.** It dies
> after one use or fifteen minutes; the key itself is injected server-side when
> the script is fetched. The page also links the script source so you can read it
> before running it, and one line puts everything back:
> `curl -fsSL <base>/uninstall | sh`.

**Prefer to do it yourself?** The manual steps below configure exactly the same
things, and they are what the script writes.

---

## Before you start

1. A DeepRouter account → **https://deeprouter.co**
2. A DeepRouter **API key** (it starts with `sk-`). Get it from the console:
   **API Keys** page (it's also shown once on your welcome screen right after signup).
3. Gemini CLI installed:
   ```bash
   npm install -g @google/gemini-cli
   ```

---

## Step 1 — Set the two environment variables

Gemini CLI reads two settings to redirect its traffic:

- **`GOOGLE_GEMINI_BASE_URL`** — where requests go. Point it at DeepRouter's Gemini endpoint.
- **`GEMINI_API_KEY`** — your key. Put your DeepRouter key here (not a Google key).

Add both to your shell profile (`~/.zshrc`, `~/.bashrc`, or your fish config):

```bash
# Send Gemini CLI to DeepRouter. Host only - the CLI appends /v1beta itself.
export GOOGLE_GEMINI_BASE_URL=https://api.deeprouter.co
# Authenticate with your DeepRouter key
export GEMINI_API_KEY=sk-...your-deeprouter-key...
```

Then reload your shell (or open a new terminal window):

```bash
source ~/.zshrc
```

> **Why the host on its own — no `/v1beta`?** Gemini CLI speaks Google's native Gemini format,
> and DeepRouter's `…/v1beta` endpoint understands it exactly, so the CLI works unchanged. But
> **the CLI appends `/v1beta` for you.** Add it yourself and every request goes to
> `/v1beta/v1beta/models/…`, which the gateway rejects with
> `404 Invalid URL` — a message that says nothing about the real cause.
> (DeepRouter also has an OpenAI‑style endpoint, but the Gemini CLI doesn't speak that dialect —
> see the fallback note below.)

---

## Step 2 — Run it

Start Gemini CLI in any folder:

```bash
gemini
```

If it tries to walk you through a Google sign‑in, choose the **API key** option (not "log in with
Google") — you're authenticating through DeepRouter, not a Google account.

To use a specific model, set it once:

```bash
export GEMINI_MODEL=gemini-2.5-flash
```

Pick exact model IDs from the console **Model Catalog**.

---

## Verify it's working

Ask Gemini CLI something simple like "say hello." A normal reply means traffic is flowing
through DeepRouter. To confirm, open the DeepRouter console and watch your usage tick up after
a request.

You can also test the endpoint directly with curl — a `200` means you're routed correctly:

```bash
curl "https://api.deeprouter.co/v1beta/models/gemini-2.5-flash:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"parts": [{"text": "Say hello from DeepRouter."}]}]
  }'
```

---

## If the Gemini endpoint gives you trouble (OpenAI‑compatible fallback)

Gemini CLI is built and tested against Google's own servers, and Google does **not** officially
support pointing it at third‑party endpoints — so behavior can vary between CLI versions. If the
steps above don't work cleanly on your version, the most reliable path is to route through
DeepRouter's **OpenAI‑compatible** endpoint instead, using a tool that speaks that dialect:

- Use **[Codex CLI](./codex.md)** or **[OpenCode](./opencode.md)** with DeepRouter's
  `https://api.deeprouter.co/v1` endpoint — both are first‑class OpenAI‑protocol clients.
- Or run a local OpenAI‑compatible proxy (e.g. LiteLLM) in front of DeepRouter and point Gemini
  CLI at the proxy.

We'd rather tell you this honestly than have you fight a half‑working setup.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| **Connection or 404 errors** | Make sure `GOOGLE_GEMINI_BASE_URL` is exactly `https://api.deeprouter.co` — the **host only**, with no `/v1beta` and no trailing slash. `404 Invalid URL (POST /v1beta/v1beta/models/…)` means you added `/v1beta` yourself; the CLI already does. |
| **Authentication / 401** | The key is wrong or it picked your Google login. Set `GEMINI_API_KEY` to your DeepRouter `sk-...` key and choose the API‑key sign‑in option. |
| **Still going to Google** | A stale env var or an old session is winning. Run `echo $GOOGLE_GEMINI_BASE_URL` to check, then open a fresh terminal so the new values take effect. |
| **Weird parameter errors** | Your Gemini CLI version may be sending fields DeepRouter's endpoint doesn't accept. Use the OpenAI‑compatible fallback above. |
| **`model not found`** | That model isn't enabled for your account. Pick an ID from the console **Model Catalog**. |

---

## Reference

| Item | Value |
|---|---|
| Value for `GOOGLE_GEMINI_BASE_URL` | `https://api.deeprouter.co` — **host only** |
| Endpoint the CLI actually calls | `POST /v1beta/models/<model>:generateContent` (the CLI adds `/v1beta`) |
| Env var (endpoint) | `GOOGLE_GEMINI_BASE_URL` |
| Env var (key) | `GEMINI_API_KEY` (use your DeepRouter `sk-...` key) |
| Auth header | `x-goog-api-key: <key>` (sent by the CLI) |
| OpenAI‑compatible fallback base | `https://api.deeprouter.co/v1` |
| Model IDs | DeepRouter console → **Model Catalog** |
| Get a key | DeepRouter console → **API Keys** |
