# Cherry Studio → DeepRouter

[Cherry Studio](https://cherry-ai.com) 是一款好用的桌面聊天应用（支持 Windows、Mac、
Linux）。它允许你添加自己的"模型服务商"，这样你就能直接接入 DeepRouter，和我们的模型对话。
不用写代码、不用敲命令行——只要在设置里点几下就行。

> **一句话版** —— 在 **设置 → 模型服务商 → 添加** 里，新建一个 **OpenAI** 类型的服务商，填入：
>
> | 字段 | 值 |
> |---|---|
> | 服务商类型 | **OpenAI** |
> | API host（API 地址） | `https://api.deeprouter.co` |
> | API key | 你的 DeepRouter 密钥（`sk-...`） |
> | 模型 | 从控制台 **模型目录（Model Catalog）** 里添加一个（例如 `claude-haiku-4-5`） |

---

## 给 AI 助手看的事实块

**看不懂？没关系** —— 这段不是写给你的，是写给 AI 的。
把下面整段复制下来，连同一句「教我怎么配」发给任意 AI 助手（ChatGPT、Claude、
你手边用哪个都行），它就会一步步告诉你每个值该填在哪里。这一段之外的内容，
就是同一件事写给人看的版本。

```yaml
# Verified against the live DeepRouter gateway on 2026-08-28. Copy these values exactly.
tool: cherry-studio
api_protocol: OpenAI
base_url: "https://api.deeprouter.co"
base_url_warning: "HOST ONLY - do not add /v1. Cherry Studio appends it; writing it yourself gives /v1/v1 and a 404 whose message does not name the cause."
endpoint_called: "POST /chat/completions"
auth_header: "Authorization: Bearer <your sk- key>"
model_example: "claude-haiku-4-5"
model_auto: "deeprouter-auto"   # smart routing; enable it for your account first
model_source: "https://deeprouter.co console -> Model Catalog"
get_a_key: "https://deeprouter.co console -> API Keys"
guide: "https://deeprouter.co/resources/cherry-studio"
```

## 为什么选 DeepRouter

一把密钥，畅用所有模型——Claude、Qwen、GLM、DeepSeek、Kimi 等等——自动路由，还能在同一个地方查看你的用量和花费。

---

## 开始之前

1. 一个 DeepRouter 账号 → **https://deeprouter.co**
2. 一把 DeepRouter **API key**（以 `sk-` 开头）。在控制台的 **API Keys** 里可以找到
   （注册后欢迎页上也会显示一次）。
3. 电脑上装好 **Cherry Studio**。

---

## 一键配置（推荐）

下面这些其实你不用手填。如果这台机器上已经装了 Cherry Studio：

1. 打开 DeepRouter 控制台的 **API Keys（调用密钥）** 页。
2. 在 **AI 应用与插件** 那一块，点 **Cherry Studio**。
3. 浏览器会问你要不要打开这个应用 —— 允许。
4. Cherry Studio 被拉起来，服务商、地址、密钥、模型都已经填好。**你一个字都不用输。**

> **应用里显示的名字是「New API」，不是 DeepRouter。** 这条链接匹配的是它内置的那个预设，
> 名字来自上游，我们改不了 —— 看到「New API」就说明点对了。

**点了没反应？** 这类链接在以下几种情况下会静悄悄地失败：没装这个应用、版本太老没注册
协议、或者公司电脑禁用了自定义协议。遇到这种情况，照下面的手动步骤做就行 —— 那条路一定通。

---

## 或者手动配置

1. 打开 Cherry Studio。点击左侧栏的 **设置**（齿轮 ⚙️）图标。
2. 打开 **模型服务商**（Model Providers）标签页（有时显示为"模型服务"）。
3. 在服务商列表底部，点击 **+ 添加**。
4. 给它起一个你认得出的名字——比如 **DeepRouter**——**服务商类型** 选 **OpenAI**。点击 **确定 / 添加**。
5. 选中你新建的 DeepRouter 服务商，填写：
   - **API key**：你的 DeepRouter 密钥（`sk-...`）
   - **API host**（也叫 *API 地址* / *Base URL*）：`https://api.deeprouter.co`
6. 滚动到 **模型（Models）** 区域，点击 **+ 添加**（或 **管理**）。输入一个来自 DeepRouter
   控制台 **模型目录（Model Catalog）** 的模型 ID，例如 `claude-haiku-4-5`，然后添加它。
7. 确认服务商的开关（在它面板顶部）已经 **打开**。

### 关于 API host 末尾斜杠的一个小提示

Cherry Studio 对你粘贴的地址有一个小规则：

- 如果地址 **不是** 以斜杠结尾（比如 `https://api.deeprouter.co`），Cherry Studio 会
  自动帮你补上 **`/v1`**——这正是你想要的，得到的就是正确的
  `https://api.deeprouter.co/v1`。
- 如果你加了 **末尾斜杠**（`https://api.deeprouter.co/v1/`），Cherry Studio 会
  **完全按你输入的内容** 使用，**不会** 再添加任何东西。所以如果你想自己写出
  `/v1`，那就写成带末尾斜杠的 `https://api.deeprouter.co/v1/`。

两种写法都行——只是别写成不带末尾斜杠的 `https://api.deeprouter.co/v1`，
否则你会得到重复的 `/v1/v1`。

---

## 验证是否生效

1. 回到服务商面板，点击 API key 旁边的 **检查（Check）** 按钮（Cherry Studio 会
   去 ping 一下服务商，确认密钥和地址正确）。出现成功提示就说明连上了。
2. 新建一个 **对话**，在模型选择器里选你的 DeepRouter 模型，问点简单的，
   比如"Say hello from DeepRouter."。
3. 打开 DeepRouter 控制台——这次请求应该会出现在你的用量/日志里。

---

## 常见问题排查

| 现象 | 解决办法 |
|---|---|
| **"检查"失败 / 连接错误** | 用 `https://api.deeprouter.co`（不带末尾斜杠，让 Cherry 自动补 `/v1`），**或** `https://api.deeprouter.co/v1/`（带末尾斜杠）。不要用不带末尾斜杠的 `/v1`。 |
| **报 404 或路径出现重复** | 你很可能输入了不带末尾斜杠的 `…/v1`，于是变成了 `/v1/v1`。去掉 `/v1`，让 Cherry 自动补。 |
| **401 / 鉴权错误** | 密钥写错了、被吊销了，或额度用完了——去控制台检查 **API Keys** 和账单。 |
| **找不到模型** | 用控制台 **模型目录（Model Catalog）** 里准确的模型 ID。 |
| **服务商变灰 / 没有列出模型** | 把服务商的开关 **打开**，然后在 **模型（Models）** 下至少添加一个模型。 |

---

## 参考信息

| 项目 | 值 |
|---|---|
| 在哪里设置 | Cherry Studio **设置 → 模型服务商 → 添加（类型：OpenAI）** |
| API host | `https://api.deeprouter.co`（Cherry 会自动补 `/v1`） |
| 使用的接口 | `POST /chat/completions`（兼容 OpenAI） |
| 鉴权方式 | `Authorization: Bearer <key>`（Cherry 会帮你发送） |
| 模型 ID | DeepRouter 控制台 → **模型目录（Model Catalog）** |
| 获取密钥 | DeepRouter 控制台 → **API Keys** |
