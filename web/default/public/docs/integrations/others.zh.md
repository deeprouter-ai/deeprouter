# 任意其他工具 → DeepRouter

不是每个应用都在这里有专门的教程——但你几乎从来都不需要。DeepRouter 会说
两种几乎所有 AI 工具都已经听得懂的标准“语言”：

- **OpenAI 兼容 API**，以及
- **Anthropic 原生（Claude）API**。

所以通用规则很简单：

> **只要一个工具允许你设置接入地址（Base URL）和 API Key，你就能把它接到 DeepRouter。**
> 使用 OpenAI 接入地址 `https://api.deeprouter.co/v1`（或者 Anthropic 接入地址
> `https://api.deeprouter.co`），配上你的 DeepRouter 密钥（`sk-...`）。

> **一句话总结**
>
> | 如果工具说的是… | 把接入地址（Base URL）设为 | 密钥这样用 |
> |---|---|---|
> | OpenAI 格式 | `https://api.deeprouter.co/v1` | `Authorization: Bearer sk-...` |
> | Anthropic / Claude 格式 | `https://api.deeprouter.co` *（不带 `/v1`）* | `x-api-key: sk-...` |
>
> 从控制台的 **Model Catalog（模型目录）** 里挑选模型 ID。

---

## 给 AI 助手看的事实块

**看不懂？没关系** —— 这段不是写给你的，是写给 AI 的。
把下面整段复制下来，连同一句「教我怎么配」发给任意 AI 助手（ChatGPT、Claude、
你手边用哪个都行），它就会一步步告诉你每个值该填在哪里。这一段之外的内容，
就是同一件事写给人看的版本。

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


## 为什么用 DeepRouter

一把密钥，所有模型——Claude、Qwen、GLM、DeepSeek、Kimi 等等——自动路由，还能在一个地方统一查看用量和花费。

---

## 开始之前

1. 一个 DeepRouter 账号 → **https://deeprouter.co**
2. 一把 DeepRouter **API Key**（`sk-...`），在控制台的 **API Keys** 里获取（注册后的欢迎页上也会
   显示一次）。

---

## 我的工具用的是哪一种？

快速判断方法：

- 如果工具提到 **OpenAI**、“OpenAI 兼容”、`chat/completions`，或者像
  *Base URL* + *Model* 这样的字段 → 走 **OpenAI** 路径。
- 如果它提到 **Anthropic** 或 **Claude**，或者说到 `messages` / `x-api-key`
  → 走 **Anthropic** 路径。
- 如果你不确定，先试 **OpenAI**——它更常见。

### 需要改的两项设置

不管工具把这些字段叫什么（Base URL、API Base、Endpoint、Host……），都这样设：

| | OpenAI 路径 | Anthropic 路径 |
|---|---|---|
| Base URL | `https://api.deeprouter.co/v1` | `https://api.deeprouter.co`（不带 `/v1`，结尾不带斜杠） |
| API Key | 你的 `sk-...` 密钥 | 你的 `sk-...` 密钥 |
| Model | 从控制台 **Model Catalog** 里选 | 从 **Model Catalog** 里选一个 Claude 模型 |

就这样——除了这两个值，不需要改任何代码。

---

## 用一条 curl 命令做冒烟测试

在折腾工具之前，你可以直接在终端里证明你的密钥 + 接入地址是好用的。把 `sk-...` 换成
你自己的密钥。

**OpenAI 兼容：**

```bash
curl https://api.deeprouter.co/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role": "user", "content": "Say hello from DeepRouter."}]
  }'
```

**Anthropic 原生：**

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

如果其中任意一条返回了正常的回复，说明你的账号和密钥都没问题——剩下的任何问题
都出在工具的配置上。

---

## 确认它在工作（在工具里）

1. 在工具里配置好接入地址（Base URL）+ 密钥，然后发一条简单的测试消息。
2. 打开 DeepRouter 控制台——这条请求应该会出现在你的用量/日志里。

---

## 排查问题

| 现象 | 解决办法 |
|---|---|
| **鉴权 / 401 错误** | 检查密钥（`sk-...`），并确认它在控制台里还有额度（**API Keys** + 账单）。OpenAI 路径的请求头是 `Authorization: Bearer`；Anthropic 路径的是 `x-api-key`。 |
| **404 / 连接错误（OpenAI 路径）** | 接入地址必须是 `https://api.deeprouter.co/v1`（带 `/v1`）。有些工具要的是完整的 `…/v1/chat/completions`——看看那个字段是“base URL”还是“完整 endpoint”。 |
| **404 / 连接错误（Anthropic 路径）** | 接入地址必须是 `https://api.deeprouter.co`（不带 `/v1`，结尾不带斜杠）。 |
| **找不到模型** | 使用控制台 **Model Catalog** 里精确的模型 ID（例如 `claude-haiku-4-5`）。 |
| **curl 能用但工具不行** | 工具把请求发到了错误的 URL，或者用了错误的鉴权请求头——对照上面的表格重新检查这两项设置。 |

---

## 参考

| 项目 | OpenAI 兼容 | Anthropic 原生 |
|---|---|---|
| Base URL | `https://api.deeprouter.co/v1` | `https://api.deeprouter.co` |
| Endpoint | `POST /chat/completions` | `POST /v1/messages` |
| 鉴权请求头 | `Authorization: Bearer <key>` | `x-api-key: <key>`（或 `Authorization: Bearer <key>`） |
| 环境变量（如果工具会读取） | `OPENAI_BASE_URL`、`OPENAI_API_KEY` | `ANTHROPIC_BASE_URL`、`ANTHROPIC_AUTH_TOKEN` |
| 模型 ID | 控制台 → **Model Catalog** | 同上 |
| 获取密钥 | 控制台 → **API Keys** | 同上 |
