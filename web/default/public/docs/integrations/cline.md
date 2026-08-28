# Cline → DeepRouter

[Cline](https://cline.bot) is a coding assistant that runs inside VS Code. It lets you
choose your own model provider, so you can point it at DeepRouter. There are two ways to
do it, and **both work** — pick whichever you prefer:

- **OpenAI Compatible** path (recommended, simplest)
- **Anthropic** path (if you want Cline to speak Anthropic's native format)

> **TL;DR (OpenAI Compatible)** — in Cline's settings, choose provider **OpenAI Compatible** and fill in:
>
> | Field | Value |
> |---|---|
> | Base URL | `https://api.deeprouter.co/v1` |
> | API Key | your DeepRouter key (`sk-...`) |
> | Model ID | from the console **Model Catalog** (e.g. `claude-haiku-4-5`) |

---

## Facts for AI assistants

**Not sure what this is?** That's fine — it is written for an AI, not for you.
Copy the whole block below, paste it to any AI assistant (ChatGPT, Claude, whichever
you use) together with a sentence like *"walk me through setting this up"*, and it
will tell you exactly where each value goes. Everything above and below this block is
the same thing written for a human.

```yaml
# Verified against the live DeepRouter gateway on 2026-08-28. Copy these values exactly.
tool: cline
api_protocol: OpenAI
base_url: "https://api.deeprouter.co/v1"
endpoint_called: "POST /chat/completions"
auth_header: "Authorization: Bearer <your sk- key>"
model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/resources/cline"
```

## Why DeepRouter

One key, every model — Claude, Qwen, GLM, DeepSeek, Kimi and more — with automatic routing and a single place to track usage and spend.

---

## Before you start

1. A DeepRouter account → **https://deeprouter.co**
2. A DeepRouter **API key** (starts with `sk-`). Find it in the console under **API Keys**
   (also shown once on your welcome screen after signup).
3. The **Cline** extension installed in VS Code (from the Extensions marketplace).

---

## Path A — OpenAI Compatible (recommended)

1. In VS Code, open Cline (click its robot icon in the sidebar).
2. Click the **gear / settings (⚙️)** icon at the top of the Cline panel.
3. Under **API Provider**, open the dropdown and choose **OpenAI Compatible**.
4. Fill in the three fields:
   - **Base URL**: `https://api.deeprouter.co/v1`
   - **API Key**: your DeepRouter key (`sk-...`)
   - **Model ID**: a model from the console **Model Catalog**, e.g. `claude-haiku-4-5`
5. Click **Done / Save**.

That's it — Cline now sends your chats to DeepRouter using the OpenAI-style `/chat/completions` endpoint.

---

## Path B — Anthropic (native)

Use this if you'd rather Cline talk to DeepRouter in Anthropic's native Messages format.

1. Open Cline → **settings (⚙️)** icon.
2. Under **API Provider**, choose **Anthropic**.
3. Check the box **Use custom base URL**, and in the URL field enter:
   ```
   https://api.deeprouter.co
   ```
   *(no `/v1` here — the Anthropic path uses the bare host; DeepRouter appends `/v1/messages` itself.)*
4. In the **Anthropic API Key** field, paste your DeepRouter key (`sk-...`).
5. From the **Model** dropdown, pick a Claude model that's in your DeepRouter catalog
   (e.g. `claude-haiku-4-5`). You can leave **Enable Extended Thinking** off unless you want it.
6. Click **Done / Save**.

---

## Verify it's working

1. In the Cline chat box, type a simple request like "Say hello from DeepRouter," then send.
2. You should get a normal reply.
3. Open the DeepRouter console — the request should appear in your usage/logs.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| **Auth / 401 error** | Check the key is correct and has quota in the console (**API Keys** + billing). |
| **Connection error (OpenAI Compatible)** | Base URL must be `https://api.deeprouter.co/v1` (with `/v1`). |
| **Connection error (Anthropic)** | Base URL must be `https://api.deeprouter.co` (no `/v1`, no trailing slash). |
| **Model not found** | Use an exact model ID from the console **Model Catalog**. |
| **Settings won't save** | Reopen the ⚙️ panel and re-enter the field; make sure the right **API Provider** is selected first. |

---

## Reference

| Item | OpenAI Compatible | Anthropic |
|---|---|---|
| Provider to pick | **OpenAI Compatible** | **Anthropic** |
| Base URL | `https://api.deeprouter.co/v1` | `https://api.deeprouter.co` |
| Endpoint used | `POST /chat/completions` | `POST /v1/messages` (auto-appended) |
| Auth | `Authorization: Bearer <key>` | `x-api-key: <key>` |
| Model IDs | DeepRouter console → **Model Catalog** | same |
| Get a key | DeepRouter console → **API Keys** | same |
