# DeepRouter MCP 服务

DeepRouter 在 `https://deeprouter.co/mcp` 提供一个只读的 [MCP](https://modelcontextprotocol.io) 服务。
把它加进 Claude Code、Codex 或 Cursor 一次，你的 AI 就能直接问 DeepRouter：这把密钥能用哪些模型、
每个模型怎么调、多少钱、另一个工具该怎么配——不用你再把文档贴进对话。

> **一句话版** —— Claude Code 一条命令：
> ```bash
> claude mcp add --transport http deeprouter https://deeprouter.co/mcp --header "Authorization: Bearer sk-...your-deeprouter-key..."
> ```

它只回答问题。这个服务不会生成任何图片或视频，也不会花钱：真正干活时，你的 AI 拿你的密钥
去调 DeepRouter 的 API，就像其他接入指南里写的那样。

---

## 给 AI 助手看的事实块

**看不懂？没关系** —— 这段不是写给你的，是写给 AI 的。
把下面整段复制下来，连同一句「教我怎么配」发给任意 AI 助手（ChatGPT、Claude、
你手边用哪个都行），它就会一步步告诉你每个值该填在哪里。这一段之外的内容，
就是同一件事写给人看的版本。

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

## 你的 AI 能问什么

| 工具 | 回答什么 |
|---|---|
| `list_models` | 你这把密钥现在能用哪些模型：每个是干什么的（聊天、图片、视频、配音、转写……）、该调哪个接口、多少钱。 |
| `get_model` | 一个模型怎么调：完整地址、认证请求头、必填参数、最小请求体；视频还给出「提交、轮询、下载」三步。另外是它的价格，以及你的密钥能不能用它。 |
| `get_connection_config` | 另一个工具（Cursor、Cherry Studio、OpenAI SDK……）该怎么填，逐字取自这里对应的接入指南。 |
| `check_key` | 你的密钥能不能用、不能用的原因；它自己的额度上限、到期时间，以及替它付钱的那份余额。不会显示密钥本身。 |
| `get_started` | DeepRouter 是什么、各协议的接入地址、怎么拿到密钥。 |

报的价是你这把密钥实际要付的，已经乘上你所在分组的倍率。按次计费的视频、图片模型报的是
基础价：有些厂商会再按片长、分辨率或张数相乘，所以 AI 应该在生成之前先告诉你预计花多少。

---

## 准备工作

1. 一个 DeepRouter 账号 → **https://deeprouter.co**
2. 一把 DeepRouter **API Key**（以 `sk-` 开头）：控制台 → **API Keys**。
3. 下面任意一个工具：[Claude Code](https://claude.com/claude-code)、[Codex CLI](https://github.com/openai/codex)、[Cursor](https://cursor.com)。

---

## Claude Code

在你的项目目录里运行（一行）：

```bash
claude mcp add --transport http deeprouter https://deeprouter.co/mcp --header "Authorization: Bearer sk-...your-deeprouter-key..."
```

- 这样加的服务只对当前项目生效——这是 Claude Code 默认的 `local` 范围，存在 `~/.claude.json`，
  不会进你的代码仓库。想在所有项目里都能用，加上 `--scope user`。
- **不要**在命令里直接写着密钥又用 `--scope project`：那会在仓库里写一个 `.mcp.json`，而这个文件
  本来就是要提交的。如果确实要一份共享的 `.mcp.json`，把请求头写成
  `"Authorization": "Bearer ${DEEPROUTER_API_KEY}"`，密钥放在这个环境变量里。
- **检查：** 运行 `claude mcp list`，最后一行应该是 `deeprouter: https://deeprouter.co/mcp (HTTP) - ✔ Connected`。
  在 Claude Code 里输入 `/mcp` 也能看到。
- **移除：** `claude mcp remove deeprouter`。

---

## Codex CLI

Codex 从环境变量里读取 Bearer 令牌，所以命令里写的是变量名，不是密钥：

```bash
codex mcp add deeprouter --url https://deeprouter.co/mcp --bearer-token-env-var DEEPROUTER_API_KEY
```

它会往 `~/.codex/config.toml` 写入下面几行——你也可以手动加：

```toml
[mcp_servers.deeprouter]
url = "https://deeprouter.co/mcp"
bearer_token_env_var = "DEEPROUTER_API_KEY"
```

启动 Codex 的那个环境里必须有 `DEEPROUTER_API_KEY` 这个变量。用 DeepRouter 一键配置装过 Codex 的，
这个变量已经设好了。没有的话：

- macOS / Linux：把 `export DEEPROUTER_API_KEY=sk-...your-deeprouter-key...` 加到 `~/.zshrc` 或
  `~/.bashrc`，然后开一个新终端。
- Windows（PowerShell）：运行 `setx DEEPROUTER_API_KEY "sk-...your-deeprouter-key..."`，然后开一个新终端。

- **检查：** `codex mcp list` 里能看到 `deeprouter`，认证方式是 `Bearer token`；在 Codex 里输入 `/mcp`
  能看到它的工具。
- **移除：** `codex mcp remove deeprouter`。

---

## Cursor

打开（没有就新建）`~/.cursor/mcp.json`，对所有项目生效；或者项目里的 `.cursor/mcp.json`，只对这个项目生效：

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

不想把密钥写进文件，就改写成 `"Bearer ${env:DEEPROUTER_API_KEY}"`，再设置这个环境变量。项目里的
`.cursor/mcp.json` 如果写着密钥本身，千万别提交。

---

## 试一试

直接用大白话问你的 AI，它会自己调这些工具来回答：

- *「我这把 DeepRouter 密钥能用哪些视频模型？各要多少钱？」*
- *「用 Python 怎么调刚才那个模型？」*
- *「我的 DeepRouter 密钥还能用吗？余额还剩多少？」*
- *「帮我把 Cherry Studio 接到 DeepRouter 上。」*

## 不带密钥

不加请求头，这个服务照样回答：`list_models` 给出公开价目（`scope: public`），`get_model`、
`get_connection_config`、`get_started` 都能用，`check_key` 会回答 `no_key` 并告诉你去哪拿密钥。
跟**你的**模型、**你的**余额有关的回答，需要带上密钥。

---

## 排查问题

| 现象 | 解决办法 |
|---|---|
| **服务连不上** | 地址必须正好是 `https://deeprouter.co/mcp`——不带 `/v1`，末尾不带斜杠。 |
| **加了密钥，`list_models` 却回答 `scope: public`** | 密钥没送到服务端。Codex：启动它的环境里没有 `DEEPROUTER_API_KEY`——设好后开一个新终端。Claude Code / Cursor：请求头必须是 `Authorization: Bearer sk-...`，`Bearer ` 不能少。 |
| **`check_key` 回答 `invalid`** | 密钥没复制完整，或者已被删除。到控制台重新复制。 |
| **`check_key` 说请求头里是 `${ANTHROPIC_AUTH_TOKEN}` 这几个字本身** | 密钥页那条命令写的是这个变量名，而启动 Claude Code 的环境里没有它。到控制台的 API Keys 页跑一次一键配置（它会把密钥存进这个变量），再开一个新终端；或者重新添加时把变量换成你的密钥。 |
| **`check_key` 回答 `disabled`、`expired` 或 `exhausted`** | 它的说明里写了该改什么；到控制台的 API Keys 里改。 |
| **`check_key` 回答 `refused`** | 后面跟着网关自己的原因——比如你的 IP 不在这把密钥的允许列表里。 |
| **用浏览器打开这个地址显示 "Method Not Allowed"** | 正常：这个地址是给 MCP 客户端用的，它们发的是 POST 请求。 |

---

## 参考信息

| 项目 | 值 |
|---|---|
| MCP 地址 | `https://deeprouter.co/mcp`（`https://api.deeprouter.co/mcp` 是同一个服务） |
| 传输方式 | Streamable HTTP，无状态，返回 JSON |
| 认证 | `Authorization: Bearer <key>`——可选 |
| 工具 | `list_models`、`get_model`、`get_connection_config`、`check_key`、`get_started`——全部只读 |
| 给 AI 的 API 说明 | `https://deeprouter.co/llms.txt` |
| 获取密钥 | DeepRouter 控制台 → API Keys |
