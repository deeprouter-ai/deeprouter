# OpenAI Codex CLI → DeepRouter

把 OpenAI 的 [Codex CLI](https://developers.openai.com/codex) 指向 DeepRouter，
这样它的请求就会经过你的 DeepRouter 账户，而不是直接发给 OpenAI。
Codex 使用 **OpenAI 协议**，而 DeepRouter 提供了一个 OpenAI 兼容的接口，
所以这只是改一个配置文件的事——不用写任何代码。

> **一句话版** — 在 `~/.codex/config.toml` 里加一个 provider，并设置一个 API Key 就行。
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

## 给 AI 助手看的事实块

**看不懂？没关系** —— 这段不是写给你的，是写给 AI 的。
把下面整段复制下来，连同一句「教我怎么配」发给任意 AI 助手（ChatGPT、Claude、
你手边用哪个都行），它就会一步步告诉你每个值该填在哪里。这一段之外的内容，
就是同一件事写给人看的版本。

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

## 为什么让 Codex 走 DeepRouter

- **一把钥匙，所有模型。** GPT 系列、Claude，还有许多开源模型——全都能通过
  同一个 OpenAI 风格的接口访问，并自带自动模型路由和故障转移。
- **智能路由。** DeepRouter 会为每个请求挑选合适的模型和通道，当某个上游出问题时
  自动切换。
- **账单集中管理。** 你团队的用量、花费和日志都集中在 DeepRouter 控制台里。

---

## 一键配置（推荐）

你不用手动去改任何配置文件。终端里粘一行命令就全配好了 —— 而且它只配你勾选的工具，
没装的会自动跳过。

1. 打开 DeepRouter 控制台的 **API Keys（调用密钥）** 页。
2. 在 **一键配置 → 终端工具** 里勾上 **Codex CLI**。
3. 复制对应你系统的那条命令，粘进终端：
   - macOS / Linux（WSL 和 Git Bash 也一样）：`curl -fsSL <页面上给的地址> | sh`
   - Windows（PowerShell 或 Terminal，**不能用 cmd**）：`irm <页面上给的地址> | iex`
4. 然后运行 `codex`。**先开一个新终端**。如果 Codex 还是让你登录 ChatGPT，说明这台机器本来就有 Codex 配置，改用 `codex --profile deeprouter`。

> **那条命令里带的是一次性令牌，不是你的密钥。** 它用一次就失效、十五分钟过期；真正的
> 密钥是脚本被下载时由服务端注入的。页面上还给了脚本源码链接，你可以先读再跑；想撤销
> 也是一行：`curl -fsSL <地址前缀>/uninstall | sh`。

**更想自己动手？** 下面的手动步骤配的是同样的东西 —— 脚本写进去的就是它们。

---

## 开始之前

1. 一个 DeepRouter 账户 → **https://deeprouter.co**
2. 一把 DeepRouter **API Key**（以 `sk-` 开头）。在控制台获取：
   **API Keys** 页面（注册后欢迎页上也会显示一次）。
3. 已安装 Codex CLI：
   ```bash
   npm install -g @openai/codex
   ```

---

## 第 1 步 — 打开（或新建）Codex 配置文件

文件位置：

```
~/.codex/config.toml
```

在 Mac 上就是 `/Users/you/.codex/config.toml`。如果 `.codex` 文件夹或这个文件
还不存在，就自己建一个。（开头那个点表示它是隐藏文件——在 Finder 里按
`Cmd‑Shift‑.` 可以显示隐藏文件，或者直接在终端里编辑它。）

> **重要：** Codex 只会从你 home 目录下的这个**用户级**文件里读取 provider 设置。
> 项目文件夹里的 `config.toml` 不会被用来读取 provider。

---

## 第 2 步 — 把 DeepRouter 添加为一个 provider

把下面这段粘贴到 `~/.codex/config.toml` 里：

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
# 现在的 Codex 只认 "responses";写 "chat" 会导致整个文件加载失败
wire_api = "responses"
```

每一行是干什么的，说人话：

- **`model`** — Codex 要请求的模型。从控制台 **Model Catalog（模型目录）** 里复制一个完整的 ID。
- **`model_provider`** — 告诉 Codex 用 `deeprouter` 这一段配置，而不是内置的 OpenAI。
- **`base_url`** — 请求发往哪里。端点由 Codex 自己拼；`wire_api = "responses"` 时它调用的是 `POST /v1/responses`。
- **`env_key`** — Codex 从这个环境变量里读取你的 Key，这样密钥就不会直接写在文件里。
- **`wire_api`** — API 的“方言”。填 `responses`。DeepRouter 的 `/v1` 接口同时提供 Responses API 和 Chat Completions，而现在的 Codex 只认 `responses`。

> 🔴 **千万不要写 `wire_api = "chat"`。** 这个值在 Codex v0.149.1 里已被移除，而且它不是
> 「降级处理」—— Codex 会**直接拒绝加载整个 `config.toml`**：
> `Error loading config.toml: 'wire_api = "chat"' is no longer supported.`
> 于是文件里所有设置一起失效，表现出来就像 Codex 压根没看到你的 DeepRouter 配置。
> 请填 `responses` —— DeepRouter 支持这个端点（`POST /v1/responses`，已对线上网关验证）。

---

## 第 3 步 — 把 Key 放进环境变量

把你的 DeepRouter Key 加到 shell 配置文件里（`~/.zshrc`、`~/.bashrc`，或你的 fish 配置）：

```bash
export DEEPROUTER_API_KEY=sk-...your-deeprouter-key...
```

然后重新加载 shell（或者干脆开一个新终端窗口）：

```bash
source ~/.zshrc
```

---

## 验证是否生效

在任意项目文件夹里启动 Codex：

```bash
cd your-project
codex
```

随便问它一句，比如“say hello”。能正常回复就说明流量已经走 DeepRouter 了。

你也可以直接用 curl 确认接口——返回 `200` 就说明路由正确：

```bash
curl https://api.deeprouter.co/v1/chat/completions \
  -H "Authorization: Bearer $DEEPROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role": "user", "content": "Say hello from DeepRouter."}]
  }'
```

想再确认是哪个账户在扣费，可以打开 DeepRouter 控制台，发一次请求后看着用量往上涨。

---

## 排查问题

| 现象 | 解决办法 |
|---|---|
| **连接错误或 404** | 确认 `base_url` 正好是 `https://api.deeprouter.co/v1`——要带 `/v1`，而且**结尾不要有斜杠**。 |
| **认证失败 / 401** | Key 错了或者是空的。检查 `DEEPROUTER_API_KEY` 已设置（`echo $DEEPROUTER_API_KEY`），并且文件里的 `env_key` 和这个名字完全一致。 |
| **provider 设置被忽略** | 你改的是项目本地的 `config.toml`。provider 配置只在你 home 目录下的 `~/.codex/config.toml` 里才生效。 |
| **还是发去了 OpenAI** | 旧的 `model_provider` 在起作用，或者别处的配置盖过了它。确认 `model_provider = "deeprouter"`，并在新终端里重启 Codex。 |
| **`Error loading config.toml: 'wire_api = "chat"' is no longer supported`** | 字面意思，而且 Codex 会连整个文件一起忽略。把那一行改成 `wire_api = "responses"`。**不要降级 Codex。** |
| **`model not found`** | 这个模型在你的账户里没有开通。从控制台 **Model Catalog（模型目录）** 里挑一个 ID。 |

---

## 参考速查

| 项目 | 值 |
|---|---|
| 配置文件 | `~/.codex/config.toml`（仅用户级） |
| OpenAI 兼容的 Base URL | `https://api.deeprouter.co/v1` |
| 接口 | `POST /chat/completions`（由 Codex 自动拼接） |
| `wire_api` | `responses` |
| 鉴权 | `Authorization: Bearer <key>`（Codex 发送你 `env_key` 对应的值） |
| 模型 ID | DeepRouter 控制台 → **Model Catalog（模型目录）** |
| 获取 Key | DeepRouter 控制台 → **API Keys** |
