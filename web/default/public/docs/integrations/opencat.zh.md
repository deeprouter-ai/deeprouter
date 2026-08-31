# OpenCat → DeepRouter

[OpenCat](https://opencat.app) 是一款适用于 iPhone、iPad 和 Mac 的精致 AI 聊天应用。
它允许你添加自己的"自定义服务商（custom provider）"——这正是把它指向 DeepRouter 的方式。
你只需添加一个服务商，粘贴一个网址和你的密钥，选一个模型，就完成了。无需写代码，
也不用打开终端。

> **一句话版** —— 进入 **设置（Settings）→ API 服务商（API Providers）→ 添加服务商（Add Provider）**，填入：
>
> | 字段 | 填写内容 |
> |---|---|
> | 名称 | 随便起个你喜欢的名字，例如 `DeepRouter` |
> | API Host | `https://api.deeprouter.co/v1` |
> | API Key | 你的 DeepRouter 密钥（`sk-...`） |
> | 服务商类型 / 协议 | OpenAI |
> | 模型 | 从控制台的 **模型目录（Model Catalog）** 里添加一个（例如 `claude-haiku-4-5`） |

---

## 给 AI 助手看的事实块

**看不懂？没关系** —— 这段不是写给你的，是写给 AI 的。
把下面整段复制下来，连同一句「教我怎么配」发给任意 AI 助手（ChatGPT、Claude、
你手边用哪个都行），它就会一步步告诉你每个值该填在哪里。这一段之外的内容，
就是同一件事写给人看的版本。

```yaml
# Verified against the live DeepRouter gateway on 2026-08-28. Copy these values exactly.
tool: opencat
api_protocol: OpenAI
base_url: "https://api.deeprouter.co/v1"
endpoint_called: "POST /chat/completions"
auth_header: "Authorization: Bearer <your sk- key>"
model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/resources/opencat"
```

## 为什么选 DeepRouter

一把密钥就能让 OpenCat 用上我们目录里的每一个模型（Claude、Qwen、GLM、DeepSeek、Kimi 等等），还附带智能路由，并在一个地方就能看清你的花费。

---

## 开始之前

1. 一个 DeepRouter 账号 → **https://deeprouter.co**
2. 一把 DeepRouter **API 密钥（API key）**（以 `sk-` 开头）。可在控制台的
   **API Keys** 里找到——注册后欢迎页上也会一次性显示给你。

---

## 一键配置（推荐）

打开 DeepRouter 控制台的 **API Keys（调用密钥）** 页，如果 **AI 应用与插件** 那一块里有
**OpenCat**，点它就行：OpenCat 会被拉起来，地址和密钥都已填好，你一个字都不用输。

> 那一项在不在，取决于运营在后台配了什么。**没看到就走下面的手动步骤** —— 步骤很短。

**点了没反应？** 没装 OpenCat、或者版本没注册这个协议时，链接会静悄悄地失败。照下面的手动步骤做。

---

## 或者手动配置

1. 打开 OpenCat，进入 **设置（Settings）**（齿轮图标）。
2. 点 **API 服务商（API Providers）**，再点 **添加服务商（Add Provider）**（在某些版本里
   显示为 **自定义服务商（Custom Provider）** 或 **添加自定义 API（Add Custom API）**）。
3. 在 **服务商类型（Provider type）** / **协议（Protocol）** 处，选 **OpenAI**。DeepRouter 使用
   OpenAI 格式，所以即便你最终通过它来跟 Claude 或 Qwen 模型聊天，这也是正确的选择。
4. 在 **名称（Name）** 里，输入任何你能认出的名字，例如 `DeepRouter`。
5. 在 **API Host** 框里（你也可能看到它被标为 **Endpoint** 或 **Base URL**），
   粘贴：
   ```
   https://api.deeprouter.co/v1
   ```
6. 在 **API Key** 框里，粘贴你的 DeepRouter 密钥（`sk-...`）。
7. 添加一个模型：如果 OpenCat 没有自动列出任何模型，就用 DeepRouter 控制台
   **模型目录（Model Catalog）** 里的一个 ID 手动添加（例如 `claude-haiku-4-5`）。
8. 保存。

> **关于字段名的提醒** —— OpenCat 在不同应用版本和平台（iOS 与 Mac）之间会微调措辞。
> 地址框通常叫 **API Host**，但如果你只看到 **Endpoint** 或 **Base URL**，那其实是同一个框——
> 把 `https://api.deeprouter.co/v1` 粘进去即可。

---

## 验证是否正常工作

1. 开一个新对话。
2. 在顶部选择你的 DeepRouter 服务商和模型。
3. 问点简单的，比如"Say hello from DeepRouter."
4. 你应该会收到一条正常的回复。然后打开 DeepRouter 控制台——这次请求应该
   会出现在你的用量/日志里。

---

## 故障排查

| 现象 | 解决办法 |
|---|---|
| **连接 / 网络错误** | 检查 API Host 是否严格为 `https://api.deeprouter.co/v1`（带 `/v1`，结尾不要有斜杠）。 |
| **没有模型可选** | 用控制台 **模型目录（Model Catalog）** 里的一个 ID 手动添加一个模型。 |
| **找不到模型 / 没有回复** | 该模型未对你的账号开启——从 **模型目录（Model Catalog）** 里换一个 ID。 |
| **401 / 鉴权错误** | 密钥错误、被吊销，或额度用尽——在控制台检查 **API Keys** 和账单。 |
| **找不到对应的设置项** | 字段名因版本而异——找 *Add Provider* / *Custom Provider*，再找任何要求填 host、endpoint 或 base URL 的字段。 |

---

## 速查表

| 项目 | 内容 |
|---|---|
| 在哪里设置 | OpenCat **设置（Settings）→ API 服务商（API Providers）→ 添加服务商（Add Provider）**（服务商类型：OpenAI） |
| API Host / Base URL | `https://api.deeprouter.co/v1` |
| API Key | 你的 DeepRouter 密钥（`sk-...`） |
| 使用的接口 | `POST /chat/completions`（OpenAI 兼容） |
| 鉴权 | `Authorization: Bearer <key>`（OpenCat 会替你发送密钥） |
| 模型 ID | DeepRouter 控制台 → **模型目录（Model Catalog）** |
| 获取密钥 | DeepRouter 控制台 → **API Keys** |
