# PRD — DeepRouter Onboarding 全旅程

> **Status**: 📝 Draft v0.1 · 待评审
> **Author**: Lightman + Claude
> **Date**: 2026-05-16
> **Owner**: DeepRouter Frontend + Platform
> **Parent**: [`docs/PRD.md`](../PRD.md), [`docs/tasks/api-key-simple-advanced-prd.md`](api-key-simple-advanced-prd.md)
> **范围**: 用户从访问站点 → 注册 → 首次成功 API 调用 → 长期留存 的完整 funnel

---

## 0. 版本变更

### v0.1（2026-05-16）
- 初稿：把已散落在 PR #1-#6 + 各 plan 文件里的 onboarding 决策汇总，加入未做的 Phase（注册 wizard / 默认 Key 展示 / 行业用量等画像字段 / 获客归因 / 开始清单 / 邮件验证流 / A/B 测试）

---

## 1. 背景与目标

### 1.1 现状

经过 PR #1 - #6，DeepRouter 已经把 upstream `new-api` 的 **B2B2C 运营商 UI** 重做成了 **B2C 终端用户 UI**：

- 用户旅程已打通：sign up → login → persona picker → playground 或 keys
- 已自动赠送 ¥1 试用额度（PR #4，`common.QuotaForNewUser = 500_000`）
- 已自动创建一把 default token（upstream 已有逻辑，`controller.Register` line 200-211）
- 6 个客户端教程页已就位（PR #4，`/onboarding/{cherry-studio,chatbox,lobechat,cursor,claude-code,code}`）
- 首次调用 🎉 toast 已就位（PR #6）

但**注册环节本身没动**：
- 注册表单仍是 upstream 的 username/password/email/code 单步表单
- 注册成功 → 跳 `/sign-in` 让用户再登一次（古怪的 funnel 二次跳转）
- 登录后才弹 persona picker
- 默认创建的 Key **用户不知道有**
- 试用额度 **用户不知道送了**
- 注册时**完全没有捕获用户画像**（除了邮箱/密码什么都没问）

结果：funnel 步数太多，转化文案缺失，画像数据空白，营销归因为零。

### 1.2 北极星指标

> **注册 → 首次成功 API 调用的 P50 时间**
>
> 目标：< 5 分钟。

### 1.3 二级指标

- 注册表单完成率（每步 conversion）
- persona 选择分布（casual / dev / team 各占比）
- brand 偏好分布（Claude / OpenAI / Gemini / DeepSeek 各占比）
- 落地页选择分布（哪个客户端教程 / playground / 直接 dashboard）
- 首次调用率（注册后 24h / 7d / 30d 内是否产生 ≥ 1 次成功调用）
- 试用额度耗尽率（多少用户用完免费 ¥1 → 转化为充值）
- 注册到首次充值的 P50 时间

---

## 2. 用户画像与典型旅程

按 PR #3 已上线的 persona 体系，主要 3 类。每类典型旅程不同：

### 2.1 Casual 🎨（聊天 / 创作 / 翻译 / 学习）

```
访问首页（pricing 或 home）
  → 看到"送 ¥1 试用"
  → 注册（邮箱 + 一键 OAuth）
  → 选 persona：Casual
  → 选 brand 偏好：Claude（最常见）
  → 选落地：Cherry Studio 教程
  → 教程页：复制 Base URL + Key + Model → 下载 Cherry Studio
  → Cherry Studio 配置 → 发 "你好" → 收到 Claude 回复
  → 回 dashboard 看到 "🎉 第一次调用成功了"
  → 余额：¥0.997（用了 ¥0.003）
  → 一段时间后自然消耗 → 来充值
```

**关键转化点**：Cherry Studio 安装能不能成功 + 配置能不能跑通

### 2.2 Dev 💻（个人开发者 / 写代码 / 集成 API）

```
访问首页
  → 看到"OpenAI 兼容 API + 多模型路由"
  → 注册
  → 选 persona：Developer
  → 选 brand：通常多选或无偏好
  → 选落地：Code（Python/Node 代码示例）
  → 教程页：复制 curl / Python snippet 一键 paste 到 IDE
  → curl/Python 调用成功
  → 回 dashboard 看 token 用量
  → 集成进自己的应用 / Cursor / Claude Code
  → 用量上来后充值
```

**关键转化点**：API 文档清晰度 + Python/Node 代码示例可粘贴即用

### 2.3 Team 👥（团队 / 企业接入）

```
访问首页或 BD 推荐
  → 注册
  → 选 persona：Team
  → 询问行业 / 团队规模（profile 后填）
  → 落地：dashboard（看模型 / 用量 / 团队管理 — 后续上）
  → 创建多把 Key 分配团队
  → 充值企业额度
  → 用量监控
```

**关键转化点**：团队管理 / 审计日志 / 子账号（DeepRouter 后续 phase）

---

## 3. 核心决策

### 3.1 注册时应该问什么？只问 3 件

#### 决策原则

每多问一个字段，funnel 掉 ~5-10% 转化。我们坚持「**注册时只问对路径选择关键的，profile 让用户慢慢补**」。

#### 注册时问

| 字段 | 后端存储 | 为什么必须在注册时 |
|---|---|---|
| **email + password**（或 OAuth）| `users.email`, `users.password` | 账号本体 |
| **persona** | `setting.persona` | 决定 sidebar、首页、Create Key 默认 mode（PR #3 已落地） |
| **brand_preference** | `setting.brand_preference` | 决定**自动创建的 default Key 的品牌绑定**（这把 Key 已经存在，只是没主动展示）|
| **preferred_client + 落地页** | `setting.preferred_client` | 决定注册成功跳到哪个客户端教程（funnel 最少跳转）|

#### 注册时**不**问，进 profile 慢慢填

| 字段 | 何时收集 |
|---|---|
| 显示名 `display_name` | profile 编辑，默认 = email 前缀 |
| 行业 / vertical `industry` | profile 编辑（dev / team persona 才显示） |
| 预期用量 `expected_volume` | 首次调用后弹一次（"想知道你的用量级别帮你优化"） |
| 营销邮件 opt-in `marketing_emails` | profile，默认 `false`（双 opt-in 合规） |
| 手机 / 微信 | profile 中的"账号绑定"区（已有） |

#### 静默捕获，用户感觉不到

| 字段 | 时机 |
|---|---|
| `setting.acquisition_channel` | 注册前自动捕获 `document.referrer` + URL `utm_*` 参数 |
| `setting.timezone` | 浏览器 `Intl.DateTimeFormat().resolvedOptions().timeZone` |
| `setting.signup_meta` | user agent / IP（已有）/ 注册时 page URL |
| `setting.onboarding_completed_at` | wizard 走完时打 timestamp |

### 3.2 注册成功后自动登录（绕过 /sign-in 二次跳转）

#### 现状

`controller.Register` 不 set session cookie。注册成功 → 前端跳 `/sign-in` → 用户再次输入邮箱密码登录。**funnel 多走一整步**。

#### 改造

`controller.Register` 注册成功后**直接调用与 Login 等效的 session 生成逻辑**（gin sessions），set cookie → 前端拿到响应后直接跳 `PERSONA_PRESETS[persona].defaultRoute`。

**风险 + 缓解**：
- 如果未来加 2FA / 邮箱二次验证：在 Register handler 里判断 `EmailVerificationEnabled` 是否要求第二步，仅当**全部凭据通过**时才 set cookie
- 如果用户开了 OAuth：原本 OAuth callback 就 set session，本就没问题

### 3.3 注册成功页面 / 落地：突出"你已经获得了什么"

#### 设计原则

用户付钱前已经拿到 3 样东西，我们要**显式告诉他**：

1. **¥1 试用额度（≈ 200 次对话）** — 后端 `QuotaForNewUser` 送的，但目前用户毫无知觉
2. **一把默认 API Key** — `controller.Register` line 200-211 自动创建的 `default` token，用户不知道
3. **persona / brand / client 偏好** — wizard 选好的，已经绑定

#### Welcome screen 长这样

```
┌───── 欢迎使用 DeepRouter ─────┐
│                                │
│  🎁 你已获赠 ¥1 试用额度        │
│      约 200 次对话              │
│                                │
│  🔑 我们已为你准备好一把 Key    │
│      sk-xxx... [📋 复制]        │
│      Base URL: ... [📋 复制]    │
│      Model: deeprouter         │
│                                │
│  📖 现在做什么？                │
│  ┌──────┬──────┬──────┐        │
│  │ 教程 │ 试聊 │ 文档 │        │
│  └──────┴──────┴──────┘        │
│                                │
└────────────────────────────────┘
```

3 个落地选项基于 Step 4 选择默认高亮，但用户可以现场改。

### 3.4 Dashboard 开始清单（"Getting Started"）

第一次进 dashboard / 没走完 onboarding 的用户，顶部显示一个 **dismissable** 的进度清单：

```
开始使用 DeepRouter
☑ 创建账号
☑ 选择使用方式
☐ 拿到第一把 Key →（其实已经有了，[展示]）
☐ 完成第一次调用 →（Cherry Studio / Cursor / Playground）
☐ 第一次充值 →（可选，¥1 试用额度可用到耗尽）
```

完成全部 → 自动隐藏。
用户点 X → 写 `setting.onboarding_dismissed_at` → 永不再显示。

Linear / Stripe / Notion 都是这个模式。

### 3.5 试用额度的命运

#### 当前

`common.QuotaForNewUser = 500_000`（= $1 USD = ~500K tokens ≈ 200 次平均对话）

#### 决策点

- 是否需要**可配置**？是。已经在 admin options map 里，运营可改。
- 是否需要**显示倒计时 / 进度条**？低优先级，profile 页够了。
- 试用额度耗尽后的转化文案：耗尽时弹通知"你的试用余额已用完，[充值] 开始畅享" — Phase 4 做。

### 3.6 OAuth 上提

将 GitHub / Google / 微信 / Linux.do 登录按钮**移到表单上方**，加大尺寸。

理由：现代 SaaS（Vercel / Linear / Cursor）都把 OAuth 摆在主要位置。中国市场尤其重视微信登录。一键登录免去填表步骤，直接到 Step 2 persona。

---

## 4. 注册 Wizard 详细设计

### 4.1 流程图

```
                          ┌─────────────────┐
                          │  /sign-up 进入   │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │ Step 1: 凭据    │
                          │  email/password │
                          │  或 OAuth 一键   │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │ 邮箱验证 (可选)  │
                          │  仅当后台开启    │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │ Step 2: persona │
                          │ [casual][dev][team] │
                          │     [跳过]       │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │ Step 3: brand   │
                          │ [Claude][GPT][...] │
                          │     [跳过]       │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │ Step 4: client  │
                          │ [Cherry][Cursor]│
                          │ [Playground][SDK]│
                          │     [跳过]       │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │  自动 sign-in    │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────────────┐
                          │ Welcome screen           │
                          │ - "🎁 已送 ¥1 试用"      │
                          │ - "🔑 已生成 default Key"│
                          │ - 3 个落地按钮            │
                          └────────┬────────────────┘
                                   │
                                   ▼
                          Step 4 选的落地页 或
                          PERSONA_PRESETS.defaultRoute
```

### 4.2 Step 1：账号凭据

```
┌─────────────────────────────────────┐
│  开始使用 DeepRouter                  │
│  已送 ¥1 试用 · 无需绑卡              │
│                                      │
│  ┌─────────────────────────────┐    │
│  │ 🐙 用 GitHub 一键登录         │    │
│  └─────────────────────────────┘    │
│  ┌─────────────────────────────┐    │
│  │ 🌐 用 Google 一键登录          │    │
│  └─────────────────────────────┘    │
│  ┌─────────────────────────────┐    │
│  │ 💬 用 微信 一键登录            │    │
│  └─────────────────────────────┘    │
│                                      │
│  ─────── 或 用邮箱注册 ───────       │
│                                      │
│  Email     [________________]        │
│  Password  [________________]        │
│  Verify    [____] [发送验证码]       │
│                                      │
│  [        下一步 →        ]          │
│                                      │
│  已有账号？[去登录]                   │
└─────────────────────────────────────┘
```

UI 来自 PR #3 的 `PersonaPickerDialog` 风格（黑底白卡 + ChevronRight）。

#### OAuth 路径

OAuth 成功 → 直接进 Step 2，跳过邮箱验证。如果 OAuth provider 提供了头像/displayName/email，自动预填到 `setting`。

### 4.3 Step 2：persona

PR #3 已有的 `PersonaPickerDialog`，inline 化成卡片选择：

```
┌─────────────────────────────────────┐
│  你打算怎么用 DeepRouter？           │
│  以后可以随时在个人资料里改           │
│                                      │
│  ┌──────────┬──────────┬──────────┐ │
│  │ 🎨 聊天/  │ 💻 写代码 │ 👥 团队/  │ │
│  │  创作    │  集成 API │  企业    │ │
│  │  推荐    │          │          │ │
│  └──────────┴──────────┴──────────┘ │
│                                      │
│  [跳过]            [下一步 →]        │
└─────────────────────────────────────┘
```

跳过 → `setting.persona = "unset"`，落地 dashboard 时再弹 picker（PR #3 兜底逻辑保留）

### 4.4 Step 3：brand 偏好（可跳过）

```
┌─────────────────────────────────────┐
│  你常用哪家 AI？                      │
│  我们会把你的默认 Key 绑定到这家       │
│                                      │
│  ┌────┬────┬────┬────┬────┐         │
│  │ 🟣  │ 🟠  │ 🔵  │ 🟢  │ 都行 │      │
│  │ Cla │ GPT │ Gem │ DS │      │      │
│  └────┴────┴────┴────┴────┘         │
│                                      │
│  [← 上一步]   [跳过]   [下一步 →]    │
└─────────────────────────────────────┘
```

跳过 → `brand_preference = ""`，default Key 不绑定 brand 走 `auto` 路由。

### 4.5 Step 4：落地选择（可跳过）

```
┌─────────────────────────────────────┐
│  注册完了想从哪里开始？               │
│  根据你的 persona 推荐你的首选        │
│                                      │
│  ┌──────────────────────┐  推荐      │
│  │ 🍒 Cherry Studio 教程 │           │
│  └──────────────────────┘            │
│  ┌──────────────────────┐            │
│  │ 💬 Chatbox 教程       │           │
│  └──────────────────────┘            │
│  ┌──────────────────────┐            │
│  │ 🎯 Cursor 接入        │           │
│  └──────────────────────┘            │
│  ┌──────────────────────┐            │
│  │ 💻 Python / Node 代码 │           │
│  └──────────────────────┘            │
│  ┌──────────────────────┐            │
│  │ 🧪 浏览器试聊（playground） │      │
│  └──────────────────────┘            │
│  ┌──────────────────────┐            │
│  │ 🏠 先看看 dashboard   │           │
│  └──────────────────────┘            │
│                                      │
│  [← 上一步]      [完成注册]          │
└─────────────────────────────────────┘
```

推荐顺序按 persona：
- casual → Cherry Studio 推荐
- dev → Cursor + Code
- team → dashboard

### 4.6 Welcome screen（注册成功后第一屏）

无论用户在 Step 4 选了什么，**先看一眼"你已经得到了什么"再去**。这是给用户的 dopamine。

```
┌─────────────────────────────────────┐
│ 👋 欢迎，{display_name}              │
│                                      │
│ 🎁 试用额度  ¥1.00  ≈ 200 次对话     │
│ 🔑 默认 Key  sk-abc...123  [📋 复制] │
│ 🌐 Base URL  https://...  [📋 复制] │
│ 🤖 推荐模型  deeprouter   [📋 复制] │
│                                      │
│ ┌──────────────────────────────┐    │
│ │ [继续前往：Cherry Studio 教程] │    │ ← Step 4 选的
│ └──────────────────────────────┘    │
│                                      │
│ 或者：                                │
│ [浏览器试聊]  [看 dashboard]         │
└─────────────────────────────────────┘
```

3-5 秒后自动跳 / 用户点继续 → 落地页。

---

## 5. Schema 改动

### 5.1 后端 — `dto.UserSetting` 扩展（Go）

**File**: `dto/user_settings.go`

```go
type UserSetting struct {
    // ... existing ...

    // === Onboarding-captured fields (PR #7+) ===

    // BrandPreference: 'claude' | 'openai' | 'gemini' | 'deepseek' | ''
    // Set during signup Step 3. Drives the auto-created default token's
    // simple_brand binding so Cherry Studio etc. immediately route to
    // the user's preferred provider when they type model: "deeprouter".
    BrandPreference string `json:"brand_preference,omitempty"`

    // PreferredClient: 'cherry-studio' | 'chatbox' | 'lobechat' |
    // 'cursor' | 'claude-code' | 'code' | 'playground' | 'dashboard' | ''
    // Drives the post-signup redirect target. /onboarding/<client>.
    PreferredClient string `json:"preferred_client,omitempty"`

    // AcquisitionChannel: captured silently from document.referrer +
    // utm parameters at signup time. Marketing attribution.
    AcquisitionChannel string `json:"acquisition_channel,omitempty"`

    // Timezone: IANA tz string from browser. Used for billing-period
    // boundaries + activity charts.
    Timezone string `json:"timezone,omitempty"`

    // OnboardingCompletedAt: ISO timestamp set when the user finishes
    // the registration wizard (skipping steps still counts). Drives
    // the "Getting Started" checklist on dashboard.
    OnboardingCompletedAt string `json:"onboarding_completed_at,omitempty"`

    // === Optional profile fields (PR #8+, edited in /profile not signup) ===

    // Industry: 'education' | 'finance' | 'ecommerce' | 'gaming' |
    // 'individual' | 'saas' | 'other' | ''
    Industry string `json:"industry,omitempty"`

    // ExpectedVolume: 'trying' | 'daily-low' | 'daily-medium' |
    // 'daily-high' | ''  (admin tooltips on rate-limit budgeting)
    ExpectedVolume string `json:"expected_volume,omitempty"`

    // MarketingEmails: explicit opt-in. Default false.
    MarketingEmails bool `json:"marketing_emails,omitempty"`

    // OnboardingChecklistDismissedAt: user clicked X on the Getting
    // Started widget. Don't show again.
    OnboardingChecklistDismissedAt string `json:"onboarding_checklist_dismissed_at,omitempty"`
}
```

**所有字段都在已存在的 `setting` JSON 列上，零 schema migration**。

### 5.2 前端 — TS 镜像

**File**: `web/default/src/features/profile/types.ts`

```ts
export interface UserSettings {
  // ... existing ...
  brand_preference?: 'claude' | 'openai' | 'gemini' | 'deepseek' | ''
  preferred_client?:
    | 'cherry-studio' | 'chatbox' | 'lobechat'
    | 'cursor' | 'claude-code' | 'code'
    | 'playground' | 'dashboard' | ''
  acquisition_channel?: string
  timezone?: string
  onboarding_completed_at?: string
  industry?: string
  expected_volume?: 'trying' | 'daily-low' | 'daily-medium' | 'daily-high' | ''
  marketing_emails?: boolean
  onboarding_checklist_dismissed_at?: string
}
```

### 5.3 controller.Register 改动

**File**: `controller/user.go::Register` (lines 137-220)

1. **接受新字段**：JSON body 多接受 `persona`, `brand_preference`, `preferred_client`, `acquisition_channel`, `timezone` 五个 onboarding 字段
2. **写入 setting**：在 `defaultSetting := dto.UserSetting{...}` 处一次性填入（兜底逻辑：persona 为空时仍写 "unset"，brand/client 为空就不写）
3. **创建 default token 时绑 brand**：`controller.Register` 在 line 200-211 自动创建 token 的逻辑里，加 `SimplePurpose + SimpleBrand`（来自新字段）
4. **session 自动 sign-in**：注册成功后调 `setupLogin(user, c)` (从 controller/user.go::Login 提取共享 helper)，set gin session cookie

### 5.4 注册 API 响应格式扩展

`POST /api/auth/register` 返回 body 扩展：

```json
{
  "success": true,
  "message": "",
  "data": {
    "user": { "id": 123, "email": "..." },
    "default_token": "sk-xxx",
    "trial_quota": 500000,
    "next": "/onboarding/cherry-studio"
  }
}
```

`data.next` 是前端跳转目标（按 preferred_client 计算）。`data.default_token` 暴露默认 Key 一次 —— 之后只能去 /keys 重新生成（**关键** —— 这是 token 唯一一次被明文返回）。

---

## 6. 兼容性 & 降级

### 6.1 老用户（已注册）

- 所有新字段 `setting.brand_preference / preferred_client / ...` 为空
- 现有 dashboard 弹窗 picker（PR #3）逻辑不变：persona 为空 → 弹 picker
- 老用户进 dashboard **不会自动给 brand_preference 弹 picker**（他们已经手动管理 Key 了，不要打扰）
- profile 编辑里能补填

### 6.2 wizard 中途关闭

- 任何 step 都有"跳过"按钮 → 写 `persona = "unset"` / `brand = ""` / `client = ""` → 兜底
- 关浏览器中途退出：account 已经创建（Step 1 提交时就创建了），后续 fields 空 → dashboard picker 兜底

### 6.3 邮箱验证开启时

- 当前 upstream 逻辑：如果 `common.EmailVerificationEnabled = true`，注册时强制要邮箱 + 验证码
- 我们的改造：在 Step 1 内嵌入验证码字段（保持现状），不增加额外步骤

### 6.4 OAuth 注册路径

- OAuth callback 完成账号创建后，**仍要走 Step 2 - 4**（不能跳过这些）
- 不过 OAuth callback 已经 set 了 session cookie → 自动登录免做

---

## 7. UX 关键文案

每个文案都需要 en + zh 两版，加进 `i18n/locales/{en,zh}.json`：

| Key | en | zh |
|---|---|---|
| 注册首屏副标题 | "Start in 30 seconds. ¥1 trial credit included — no card needed." | "30 秒开始用，注册即送 ¥1 试用额度，不用绑卡。" |
| Welcome screen 标题 | "Welcome, {name}" | "欢迎你，{name}" |
| 试用额度展示 | "🎁 Trial credit: ¥1.00 (~200 chats)" | "🎁 试用额度：¥1.00（约 200 次对话）" |
| 默认 Key 提示 | "🔑 Your default API key is ready — copy and store it now, you won't see it again." | "🔑 你的默认 Key 已就绪 —— 立刻复制保存，下次进来就看不到了。" |
| 落地 CTA（casual）| "Continue with Cherry Studio guide" | "继续：Cherry Studio 教程" |
| 落地 CTA（dev）| "See code examples" | "看代码示例" |
| 首次进 dashboard banner | "You haven't made an API call yet. Try the [Playground] or follow the [Cherry Studio guide]." | "你还没调用过 API，试试 [Playground] 或者跟着 [Cherry Studio 教程] 接入。" |
| Getting started checklist 标题 | "Get started with DeepRouter" | "开始使用 DeepRouter" |

---

## 8. 上线指标 & A/B 测试

### 8.1 必埋的事件（前端 → 暂存 console / Sentry / 自研 analytics）

```
- signup_step1_shown
- signup_step1_submitted     (with: source = 'form' | 'github' | 'google' | 'wechat')
- signup_step2_persona_picked  (with: persona)
- signup_step2_skipped
- signup_step3_brand_picked    (with: brand)
- signup_step3_skipped
- signup_step4_client_picked   (with: client)
- signup_step4_skipped
- welcome_screen_shown
- welcome_screen_action        (with: action = 'cherry-studio' | 'playground' | 'dashboard' | ...)
- onboarding_completed
- first_api_call               (already tracked via request_count delta in PR #6)
```

### 8.2 关键指标 dashboard（待 admin 后台支持）

- step-by-step funnel 漏斗图
- persona / brand / client 三维分布
- 注册到首次调用时间分布
- 注册 → 首充时间分布
- 各 acquisition_channel 的转化率

### 8.3 第一批 A/B 实验

| 实验 | 假设 | 测试组 vs 对照 |
|---|---|---|
| A1 | OAuth 大按钮上提能提高完成率 | 上提版 vs 当前底部小按钮 |
| A2 | brand 偏好这一步是不是必要 | 4 步 vs 3 步（跳过 Step 3） |
| A3 | Welcome screen 是否影响首次调用率 | 显示 vs 直接落地 |
| A4 | 试用额度从 ¥1 → ¥3 能否拉升首充率 | ¥1 vs ¥3 |

---

## 9. 风险 & 开放问题

### 9.1 已识别风险

| 风险 | 严重度 | 缓解 |
|---|---|---|
| 4 步注册 wizard 反而降低完成率 | 高 | 每步都有"跳过"按钮 + A/B 测 4 步 vs 2 步 |
| 自动 sign-in 绕过 2FA / 邮箱验证 | 高 | session 生成在 Register handler 里**等所有强制验证通过之后** |
| Welcome screen 暴露 default Key 字符串 → 用户截图泄漏 | 中 | 加 "立刻复制保存，下次进来就看不到了" 提示 + 仅注册当下展示，刷新即消失 |
| 默认创建的 Key 会被自动 binding brand → 用户没勾"无偏好"时混乱 | 低 | brand_preference 为空时 default token 不绑 brand 走 `auto` |
| 老用户进 dashboard 弹两层 picker（persona + brand） | 中 | 老用户只触发 persona picker，不触发 brand（profile 里能补） |
| 注册成功暴露 token 字符串影响 sleep（重新登录就看不到） | 低 | 设计就这样；要复用 ApiKeySuccessDialog 的"只显示一次"模式 |

### 9.2 待拍板的开放问题

- [ ] **试用额度数值**：¥1（500K token）是不是合适？太少不够试，太多被薅。考虑 ¥0.5 / ¥1 / ¥3 / ¥5 几种
- [ ] **brand 偏好"都行"选项**：选了的话 default token 走 `auto` group / 普通 group？要不要给 `auto` 单独跑通
- [ ] **default token 命名**：upstream 默认叫 "default"，要不要改成更有亲和力的"我的第一把 Key"？
- [ ] **Cherry Studio 教程 vs Playground**：casual 用户默认推哪个？目前我倾向 Playground（零安装），但 Cherry Studio 更接近真实使用场景
- [ ] **wizard 步骤是否能在 footer 显示"4/4"进度条**？或者只显示当前步号"Step 2 of 4"？

### 9.3 已决（v0.1）

- ✅ 注册问 3 件（persona / brand / client）
- ✅ profile 慢慢填的 4 件（行业 / 用量 / 营销 opt-in / 显示名）
- ✅ 静默捕获 3 件（acquisition / timezone / signup_meta）
- ✅ 注册成功自动 sign-in，绕过 /sign-in 跳转
- ✅ Welcome screen 显式展示"获赠的 3 样东西"
- ✅ 老用户 dashboard 弹窗兜底（PR #3 不破坏）
- ✅ 邮箱验证开启时第 1 步内嵌（不增加步骤）

---

## 10. Phase 计划

### Phase 1（PR #7，~6 天）— 注册 wizard MVP

- [x] 已盘点：dto.UserSetting + TS UserSettings 加 5 个字段
- [ ] 注册 wizard 4 步（账号 + persona + brand + client）
- [ ] OAuth 按钮上提
- [ ] controller.Register 接受 5 个新字段
- [ ] 注册成功自动 sign-in
- [ ] 注册成功 Welcome screen（显示 default token + 试用额度 + 落地 CTA）
- [ ] 兼容老用户 dashboard picker 兜底

### Phase 2（PR #8，~4 天）— Dashboard 引导

- [ ] Getting Started checklist 组件 + dismiss 逻辑
- [ ] 首次调用前的 banner（在 dashboard 显示"还没调用过"）
- [ ] /usage-logs 空状态（"还没调用过 → 试试 Playground"）
- [ ] 默认 token 进入 /keys 列表（已有，但首次显示加强突出）

### Phase 3（PR #9，~3 天）— Profile 扩展

- [ ] profile 增加"个人资料"扩展区
- [ ] 显示名编辑
- [ ] 行业 / 用量 / 营销 opt-in 表单
- [ ] persona / brand / client 偏好的再编辑（已绑 wizard 选择，profile 可改）

### Phase 4（PR #10+，持续）— A/B 测试 + 优化

- [ ] 事件埋点
- [ ] 实验 A1-A4
- [ ] 试用额度耗尽通知
- [ ] 首充转化文案
- [ ] 留存 7d/30d 通知

---

## 11. 参考

### 内部
- `docs/PRD.md` — 主 PRD
- `docs/tasks/api-key-simple-advanced-prd.md` — API Key 创建子 PRD
- PR #2 - #6 description
- `controller/user.go::Register` — 当前注册逻辑
- `controller/user.go::Login` — 当前 session 生成逻辑（要提取共享）
- `dto/user_settings.go` — UserSetting JSON 结构
- `setting/alias_setting/seed/aliases.yaml` — purpose × brand 映射
- `web/default/src/components/persona-picker-dialog.tsx` — Step 2 复用
- `web/default/src/features/keys/components/api-key-purpose-picker.tsx` — Step 3/4 复用
- `web/default/src/features/keys/components/api-key-success-dialog.tsx` — Welcome screen 的"只显示一次"模式参考

### 外部
- **Vercel signup**：邮箱 + OAuth + 多步选 framework + 落地空 project
- **Linear signup**：单步注册 + 邀请 + 团队 + 个性化 settings 多步
- **Cursor signup**：OAuth-only + 立刻 dashboard + tooltip 引导
- **Stripe signup**：邮箱 → 业务类型多步 + KYC（B2B-only）
- **ChatGPT signup**：邮箱 + 手机 → 立刻试聊（最低 friction）

DeepRouter 走的是 **Linear + Cursor 中间路线**：注册分多步以获得画像，但每步都能跳过、不强制。
