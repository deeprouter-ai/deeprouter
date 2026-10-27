# DeepRouter MCP server

DeepRouter runs a read-only [MCP](https://modelcontextprotocol.io) server at
`https://deeprouter.co/mcp`. Add it once to Claude Code, Codex or Cursor, and your AI can
ask DeepRouter itself which models your key can use, how to call each one, what it costs,
and how to set up another tool — instead of you pasting docs into the chat.

> **TL;DR** — Claude Code, one command:
> ```bash
> claude mcp add --transport http deeprouter https://deeprouter.co/mcp --header "Authorization: Bearer sk-...your-deeprouter-key..."
> ```

It only answers questions. Nothing on this server generates an image or a video, and
nothing spends money: to do the work, your AI calls the DeepRouter API with your key, the
way the other guides describe.

---

## Facts for AI assistants

**Not sure what this is?** That's fine — it is written for an AI, not for you.
Copy the whole block below, paste it to any AI assistant (ChatGPT, Claude, whichever
you use) together with a sentence like *"walk me through setting this up"*, and it
will tell you exactly where each value goes. Everything above and below this block is
the same thing written for a human.

```yaml
# DeepRouter's MCP server. Client commands follow each client's own docs (checked 2026-10-27). Copy these values exactly.
tool: mcp
mcp_url: "https://deeprouter.co/mcp"
transport: "Streamable HTTP, stateless, JSON responses"
auth_header: "Authorization: Bearer <your sk- key>"
auth_note: "Optional. Without a key the tools answer from the public catalog only, and check_key reports no_key."
tools: "list_models, get_model, get_connection_config, check_key, get_started"
read_only: "Nothing here generates anything or spends money. To do the work, call the DeepRouter API with the key."
claude_code: 'claude mcp add --transport http deeprouter https://deeprouter.co/mcp --header "Authorization: Bearer <your sk- key>"'
codex: "codex mcp add deeprouter --url https://deeprouter.co/mcp --bearer-token-env-var DEEPROUTER_API_KEY"
codex_warning: "Codex sends the value of the DEEPROUTER_API_KEY environment variable; it must be set where Codex starts."
cursor: '~/.cursor/mcp.json -> {"mcpServers": {"deeprouter": {"url": "https://deeprouter.co/mcp", "headers": {"Authorization": "Bearer <your sk- key>"}}}}'
api_reference: "https://deeprouter.co/llms.txt"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/docs/integrations/mcp.md"
```

## What your AI can ask

| Tool | What it answers |
|---|---|
| `list_models` | Which models your key can use right now: what each is for (chat, image, video, speech, transcription…), the endpoint to call and the price. |
| `get_model` | How to call one model: the full URL, the auth header, the required parameters, a minimal request body, and for video the submit, poll and download steps. Also its price, and whether your key can use it. |
| `get_connection_config` | The exact settings for another tool — Cursor, Cherry Studio, the OpenAI SDK… — taken word for word from that tool's guide here. |
| `check_key` | Whether your key works and, if not, why; its own spending limit, its expiry, and the balance that pays for it. It never shows the key itself. |
| `get_started` | What DeepRouter is, the base URL for each protocol, and how to get a key. |

Prices are what your key pays, your group's ratio included. For video and image models
priced per request it is the base price: some providers multiply it by the clip length,
the resolution or the number of images, so the AI should tell you the expected cost
before it generates anything.

---

## Prerequisites

1. A DeepRouter account → **https://deeprouter.co**
2. A DeepRouter **API key** (starts with `sk-`): console → **API Keys**.
3. One of: [Claude Code](https://claude.com/claude-code), [Codex CLI](https://github.com/openai/codex), [Cursor](https://cursor.com).

---

## Claude Code

Run this in your project folder (one line):

```bash
claude mcp add --transport http deeprouter https://deeprouter.co/mcp --header "Authorization: Bearer sk-...your-deeprouter-key..."
```

- This adds the server for the current project — Claude Code's default `local` scope,
  saved in `~/.claude.json`, never part of your repository. Add `--scope user` to have it
  in every project.
- Do **not** use `--scope project` with the key typed into the command: that writes
  `.mcp.json` into your repository, which is meant to be committed. For a shared
  `.mcp.json`, write the header as `"Authorization": "Bearer ${DEEPROUTER_API_KEY}"` and keep
  the key in that environment variable.
- **Check:** `claude mcp list` should end with `deeprouter: https://deeprouter.co/mcp (HTTP) - ✔ Connected`.
  Inside Claude Code, `/mcp` shows the same.
- **Remove:** `claude mcp remove deeprouter`.

---

## Codex CLI

Codex sends a bearer token from an environment variable, so the command names the
variable, not the key:

```bash
codex mcp add deeprouter --url https://deeprouter.co/mcp --bearer-token-env-var DEEPROUTER_API_KEY
```

That writes these lines to `~/.codex/config.toml` — you can also add them by hand:

```toml
[mcp_servers.deeprouter]
url = "https://deeprouter.co/mcp"
bearer_token_env_var = "DEEPROUTER_API_KEY"
```

`DEEPROUTER_API_KEY` has to be set in the environment Codex starts in. If you set Codex up
with DeepRouter's one-click setup, it already is. Otherwise:

- macOS / Linux: add `export DEEPROUTER_API_KEY=sk-...your-deeprouter-key...` to `~/.zshrc`
  or `~/.bashrc`, then open a new terminal.
- Windows (PowerShell): `setx DEEPROUTER_API_KEY "sk-...your-deeprouter-key..."`, then open a
  new terminal.

- **Check:** `codex mcp list` shows `deeprouter` with auth `Bearer token`; inside Codex, `/mcp`
  lists its tools.
- **Remove:** `codex mcp remove deeprouter`.

---

## Cursor

Open — or create — `~/.cursor/mcp.json` to have it in every project, or `.cursor/mcp.json`
inside one project:

```json
{
  "mcpServers": {
    "deeprouter": {
      "url": "https://deeprouter.co/mcp",
      "headers": {
        "Authorization": "Bearer sk-...your-deeprouter-key..."
      }
    }
  }
}
```

To keep the key out of the file, write `"Bearer ${env:DEEPROUTER_API_KEY}"` instead and set
that environment variable. Never commit a project `.cursor/mcp.json` that holds the key
itself.

---

## Try it

Ask your AI in plain words; it calls the tools and answers:

- *"Which DeepRouter models can my key use for video, and what do they cost?"*
- *"How do I call that model from Python?"*
- *"Is my DeepRouter key still working? How much balance is left?"*
- *"Set up Cherry Studio to use DeepRouter."*

## Without a key

Leave the header out and the server still answers: `list_models` gives the public catalog
(`scope: public`), `get_model`, `get_connection_config` and `get_started` work, and
`check_key` replies `no_key` with where to get one. Answers about *your* models and
*your* balance need the key.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| **The server fails to connect** | The URL must be exactly `https://deeprouter.co/mcp` — no `/v1`, no trailing slash. |
| **`list_models` answers `scope: public` though you added a key** | The key is not reaching the server. Codex: `DEEPROUTER_API_KEY` is not set where Codex started — set it and open a new terminal. Claude Code / Cursor: the header must read `Authorization: Bearer sk-...`, with `Bearer `. |
| **`check_key` says `invalid`** | The key was not copied in full, or it was deleted. Copy it again from the console. |
| **`check_key` says the header holds `${ANTHROPIC_AUTH_TOKEN}` itself** | The command on the keys page names that variable, and it is not set where Claude Code started. Run one-click setup from the console's API Keys page (it saves the key in the variable) and open a new terminal; or add the server again with your key in place of the variable. |
| **`check_key` says `disabled`, `expired` or `exhausted`** | Its message says what to change; do that in the console under API Keys. |
| **`check_key` says `refused`** | The gateway's own reason follows — for example, your IP is not on the key's allow-list. |
| **Opening the URL in a browser says "Method Not Allowed"** | Expected: the URL is for MCP clients, which send POST requests. |

---

## Reference

| Item | Value |
|---|---|
| MCP URL | `https://deeprouter.co/mcp` (`https://api.deeprouter.co/mcp` is the same server) |
| Transport | Streamable HTTP, stateless, JSON responses |
| Auth | `Authorization: Bearer <key>` — optional |
| Tools | `list_models`, `get_model`, `get_connection_config`, `check_key`, `get_started` — all read-only |
| API reference for AIs | `https://deeprouter.co/llms.txt` |
| Get a key | DeepRouter console → API Keys |
