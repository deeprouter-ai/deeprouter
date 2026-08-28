# Any other tool → DeepRouter

Not every app has its own guide here — but you almost never need one. DeepRouter speaks
two standard "languages" that nearly every AI tool already understands:

- the **OpenAI-compatible API**, and
- the **Anthropic-native (Claude) API**.

So the universal rule is simple:

> **If a tool lets you set a base URL and an API key, you can point it at DeepRouter.**
> Use the OpenAI base `https://api.deeprouter.co/v1` (or the Anthropic base
> `https://api.deeprouter.co`) with your DeepRouter key (`sk-...`).

> **TL;DR**
>
> | If the tool speaks… | Set base URL to | Use key as |
> |---|---|---|
> | OpenAI format | `https://api.deeprouter.co/v1` | `Authorization: Bearer sk-...` |
> | Anthropic / Claude format | `https://api.deeprouter.co` *(no `/v1`)* | `x-api-key: sk-...` |
>
> Pick model IDs from the console **Model Catalog**.

---

## Facts for AI assistants

**Not sure what this is?** That's fine — it is written for an AI, not for you.
Copy the whole block below, paste it to any AI assistant (ChatGPT, Claude, whichever
you use) together with a sentence like *"walk me through setting this up"*, and it
will tell you exactly where each value goes. Everything above and below this block is
the same thing written for a human.

```yaml
# Which base URL depends on the protocol the tool speaks - there is no single answer.
protocols:
  - api_protocol: OpenAI
    base_url: "https://api.deeprouter.co/v1"
    endpoint_called: "POST /chat/completions"
    auth_header: "Authorization: Bearer <your sk- key>"
  - api_protocol: Anthropic
    base_url: "https://api.deeprouter.co"
    endpoint_called: "POST /v1/messages"
    auth_header: "x-api-key: <your sk- key>"
  - api_protocol: Gemini
    base_url: "https://api.deeprouter.co/v1beta"
    endpoint_called: "POST /v1beta/models/<model>:generateContent"
    auth_header: "x-goog-api-key: <your sk- key>"

# Tools that append the version segment themselves - give these the HOST ONLY.
# Adding it yourself doubles the path and the gateway answers 404 Invalid URL.
host_only_tools:
  cherry-studio: "https://api.deeprouter.co"   # appends /v1
  nextchat:      "https://api.deeprouter.co"   # appends /v1
  gemini-cli:    "https://api.deeprouter.co"   # appends /v1beta

model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide_index: "https://deeprouter.co/llms.txt"
```


## Why DeepRouter

One key, every model — Claude, Qwen, GLM, DeepSeek, Kimi and more — with automatic routing and a single place to track usage and spend.

---

## Before you start

1. A DeepRouter account → **https://deeprouter.co**
2. A DeepRouter **API key** (`sk-...`) from the console under **API Keys** (also shown
   once on your welcome screen after signup).

---

## Which one does my tool use?

A quick way to tell:

- If the tool mentions **OpenAI**, "OpenAI-compatible", `chat/completions`, or fields
  like *Base URL* + *Model* → use the **OpenAI** path.
- If it mentions **Anthropic** or **Claude**, or talks about `messages` / `x-api-key`
  → use the **Anthropic** path.
- If you're not sure, try **OpenAI** first — it's the more common one.

### The two settings to change

Whatever the tool calls its fields (Base URL, API Base, Endpoint, Host…), set:

| | OpenAI path | Anthropic path |
|---|---|---|
| Base URL | `https://api.deeprouter.co/v1` | `https://api.deeprouter.co` (no `/v1`, no trailing slash) |
| API Key | your `sk-...` key | your `sk-...` key |
| Model | from console **Model Catalog** | a Claude model from **Model Catalog** |

That's it — no code changes beyond those two values.

---

## Verify with a curl smoke test

Before fiddling with the tool, you can prove your key + base URL work straight from a
terminal. Replace `sk-...` with your key.

**OpenAI-compatible:**

```bash
curl https://api.deeprouter.co/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role": "user", "content": "Say hello from DeepRouter."}]
  }'
```

**Anthropic-native:**

```bash
curl https://api.deeprouter.co/v1/messages \
  -H "x-api-key: sk-..." \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Say hello from DeepRouter."}]
  }'
```

If either returns a normal reply, your account and key are good — any remaining problem
is in how the tool is configured.

---

## Verify it's working (in the tool)

1. Configure the base URL + key in the tool, then send a simple test message.
2. Open the DeepRouter console — the request should appear in your usage/logs.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| **Auth / 401 error** | Check the key (`sk-...`) and that it has quota in the console (**API Keys** + billing). For the OpenAI path the header is `Authorization: Bearer`; for the Anthropic path it's `x-api-key`. |
| **404 / connection error (OpenAI path)** | Base URL must be `https://api.deeprouter.co/v1` (with `/v1`). Some tools want the full `…/v1/chat/completions` — check whether the field is "base URL" or "full endpoint". |
| **404 / connection error (Anthropic path)** | Base URL must be `https://api.deeprouter.co` (no `/v1`, no trailing slash). |
| **Model not found** | Use an exact model ID from the console **Model Catalog** (e.g. `claude-haiku-4-5`). |
| **Curl works but the tool doesn't** | The tool is sending to the wrong URL or with the wrong auth header — re-check the two settings against the table above. |

---

## Reference

| Item | OpenAI-compatible | Anthropic-native |
|---|---|---|
| Base URL | `https://api.deeprouter.co/v1` | `https://api.deeprouter.co` |
| Endpoint | `POST /chat/completions` | `POST /v1/messages` |
| Auth header | `Authorization: Bearer <key>` | `x-api-key: <key>` (or `Authorization: Bearer <key>`) |
| Env vars (if the tool reads them) | `OPENAI_BASE_URL`, `OPENAI_API_KEY` | `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` |
| Model IDs | console → **Model Catalog** | same |
| Get a key | console → **API Keys** | same |
