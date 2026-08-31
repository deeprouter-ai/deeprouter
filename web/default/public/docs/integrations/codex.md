# OpenAI Codex CLI → DeepRouter

Point OpenAI's [Codex CLI](https://developers.openai.com/codex) at DeepRouter so its
requests run through your DeepRouter account instead of going straight to OpenAI.
Codex speaks the **OpenAI protocol**, and DeepRouter ships an OpenAI‑compatible endpoint,
so this is a config‑file change — no coding required.

> **TL;DR** — add a provider to `~/.codex/config.toml` and set one API key.
> ```toml
> # ~/.codex/config.toml
> model = "claude-haiku-4-5"
> model_provider = "deeprouter"
>
> [model_providers.deeprouter]
> name = "DeepRouter"
> base_url = "https://api.deeprouter.co/v1"
> env_key = "DEEPROUTER_API_KEY"
> wire_api = "responses"
> ```
> ```bash
> export DEEPROUTER_API_KEY=sk-...your-deeprouter-key...
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
tool: codex
api_protocol: OpenAI
base_url: "https://api.deeprouter.co/v1"
base_url_warning: 'Goes in base_url inside ~/.codex/config.toml, together with wire_api = "responses" - the value "chat" was removed and stops Codex loading the file at all.'
endpoint_called: "POST /v1/responses"
auth_header: "Authorization: Bearer <your sk- key>"
model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/resources/codex"
```

## Why route Codex through DeepRouter

- **One key, every model.** GPT‑family, Claude, and many open models — all reachable through
  the same OpenAI‑shaped endpoint, with automatic model routing and fallback.
- **Smart routing.** DeepRouter picks the right model and channel per request and fails over
  automatically when an upstream is down.
- **Billing in one place.** Your team's usage, spend, and logs all live in the DeepRouter console.

---

## One-click setup (recommended)

You do not have to edit any config file by hand. One line in a terminal does all
of it — and it configures only the tools you tick, skipping anything you do not
have installed.

1. Open **API Keys** in the DeepRouter console.
2. Under **One-click setup → Terminal tools**, tick **Codex CLI**.
3. Copy the command for your system and paste it into a terminal:
   - macOS / Linux (also WSL and Git Bash): `curl -fsSL <the address shown> | sh`
   - Windows (PowerShell or Terminal, **not** cmd): `irm <the address shown> | iex`
4. Then run `codex`. Open a **new** terminal first. If Codex still asks you to log in to ChatGPT, your machine already had a Codex config, so run `codex --profile deeprouter` instead.

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
3. Codex CLI installed:
   ```bash
   npm install -g @openai/codex
   ```

---

## Step 1 — Open (or create) the Codex config file

The file lives at:

```
~/.codex/config.toml
```

On a Mac that's `/Users/you/.codex/config.toml`. If the `.codex` folder or the file
isn't there yet, create them. (The leading dot means it's hidden — in Finder press
`Cmd‑Shift‑.` to show hidden files, or just edit it from your terminal.)

> **Important:** Codex only reads provider settings from this **user‑level** file in your
> home folder. A `config.toml` inside a project folder is ignored for providers.

---

## Step 2 — Add DeepRouter as a provider

Paste this into `~/.codex/config.toml`:

```toml
# Which model to use by default (pick any ID from the DeepRouter Model Catalog)
model = "claude-haiku-4-5"
# Use the DeepRouter provider defined below
model_provider = "deeprouter"

[model_providers.deeprouter]
name = "DeepRouter"
# DeepRouter's OpenAI-compatible endpoint (note: ends in /v1, no trailing slash)
base_url = "https://api.deeprouter.co/v1"
# Name of the environment variable that holds your key (set in Step 3)
env_key = "DEEPROUTER_API_KEY"
# Current Codex builds only accept "responses"; "chat" stops the file loading
wire_api = "responses"
```

What each line does, in plain terms:

- **`model`** — the model Codex asks for. Copy an exact ID from the console **Model Catalog**.
- **`model_provider`** — tells Codex to use the `deeprouter` block instead of the built‑in OpenAI one.
- **`base_url`** — where requests go. Codex appends the endpoint itself; with `wire_api = "responses"` it calls `POST /v1/responses`.
- **`env_key`** — Codex reads your key from this environment variable, so the secret never sits in the file.
- **`wire_api`** — the API "dialect." Use `responses`. DeepRouter's `/v1` endpoint serves the Responses API as well as Chat Completions, and current Codex builds only accept `responses`.

> 🔴 **Do not write `wire_api = "chat"`.** It was removed in Codex v0.149.1, and it does not
> merely fall back — Codex refuses to load `config.toml` at all:
> `Error loading config.toml: 'wire_api = "chat"' is no longer supported.` Every setting in the
> file is then ignored, so Codex looks like it never saw your DeepRouter config. Use
> `responses`; DeepRouter serves that endpoint (`POST /v1/responses`, verified on the live
> gateway).

---

## Step 3 — Put your key in the environment

Add your DeepRouter key to your shell profile (`~/.zshrc`, `~/.bashrc`, or your fish config):

```bash
export DEEPROUTER_API_KEY=sk-...your-deeprouter-key...
```

Then reload your shell (or just open a new terminal window):

```bash
source ~/.zshrc
```

---

## Verify it's working

Start Codex in any project folder:

```bash
cd your-project
codex
```

Ask it something simple like "say hello." A normal reply means traffic is flowing through DeepRouter.

You can also confirm the endpoint directly with curl — a `200` response means you're routed correctly:

```bash
curl https://api.deeprouter.co/v1/chat/completions \
  -H "Authorization: Bearer $DEEPROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role": "user", "content": "Say hello from DeepRouter."}]
  }'
```

To double‑check which account is billed, open the DeepRouter console and watch your usage
tick up after a request.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| **Connection or 404 errors** | Make sure `base_url` is exactly `https://api.deeprouter.co/v1` — with `/v1`, and **no trailing slash**. |
| **Authentication / 401** | The key is wrong or empty. Check `DEEPROUTER_API_KEY` is set (`echo $DEEPROUTER_API_KEY`) and that `env_key` in the file matches that exact name. |
| **Provider settings ignored** | You edited a project‑local `config.toml`. Provider blocks only work in `~/.codex/config.toml` in your home folder. |
| **Still going to OpenAI** | An old `model_provider` is in effect, or a different config wins. Confirm `model_provider = "deeprouter"` and restart Codex in a fresh terminal. |
| **`Error loading config.toml: 'wire_api = "chat"' is no longer supported`** | Exactly what it says — and Codex then ignores the whole file. Change that line to `wire_api = "responses"`. Do not downgrade Codex. |
| **`model not found`** | That model isn't enabled for your account. Pick an ID from the console **Model Catalog**. |

---

## Reference

| Item | Value |
|---|---|
| Config file | `~/.codex/config.toml` (user‑level only) |
| OpenAI‑compatible base URL | `https://api.deeprouter.co/v1` |
| Endpoint | `POST /chat/completions` (appended by Codex) |
| `wire_api` | `responses` |
| Auth | `Authorization: Bearer <key>` (Codex sends your `env_key` value) |
| Model IDs | DeepRouter console → **Model Catalog** |
| Get a key | DeepRouter console → **API Keys** |
