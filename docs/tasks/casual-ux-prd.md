# PRD — Casual 用户全场 UX：每一个配置都有人话

> **Status**: 📝 Draft v0.1 · 待评审
> **Author**: Lightman + Claude
> **Date**: 2026-05-17
> **Owner**: DeepRouter Frontend
> **Parent**: [`docs/PRD.md`](../PRD.md), [`docs/tasks/onboarding-prd.md`](onboarding-prd.md), [`docs/tasks/api-key-simple-advanced-prd.md`](api-key-simple-advanced-prd.md)
> **范围**: Casual persona 用户在 wizard / 教程 / 创建 Key / 充值 / playground 等所有**配置/操作页面**的文案密度与交互简化

---

## 0. 版本变更

### v0.1（2026-05-17）
- 初稿：在 onboarding PRD (PR #7) + 注册 wizard (PR #8) 上线之后，沉淀出"casual 用户在配置时具体哪里卡住"的产品答案

---

## 1. 背景

### 1.1 Workshop / 视频与产品的分工

DeepRouter 团队会通过 **workshop / 视频教程** 引导用户怎么用（这是运营 + 内容创作团队的职责）。**产品层（这份 PRD）的目标不是替代视频**，而是：

> 让用户在**没看视频、没问朋友、没加群**的情况下，光看屏幕上的提示就能猜对每一个配置。

视频 / workshop 帮**有动力学习**的用户；产品文案密度帮**没耐心看视频**的用户。两者互补，不冲突。

### 1.2 Casual persona 的真实画像（PR #3 三 persona 的进一步深化）

> **Casual 不只是"不会写代码"，是"看到陌生术语就退缩"**

更细的画像：

| 维度 | Casual | Dev |
|---|---|---|
| 看到 "Base URL" / "API endpoint" | 犹豫，需要解释 | 知道是啥 |
| 看到 "安装" | 想到的是双击 dmg / exe | 想到的是 `npm install` |
| 看到陌生英文术语 | 第一反应是退出 | 默认查 docs |
| 配置失败 | 第一反应是问朋友、加群、看小红书 | 看 stderr |
| 视频 vs 文档 | 信任视频 > 文档 | 文档优先 |
| 期望体验 | "装好就能用" | "能配清楚就行" |

### 1.3 问题：当前配置页文案密度严重不足

以已上线的 `/onboarding/cherry-studio` (PR #4) 为例：

```markdown
3. Fields:
   - Name: DeepRouter
   - Base URL: {dynamic}
   - API Key: {paste}
4. Model: type `deeprouter`
```

Casual 用户疑问（实际访谈/想象）：
- "Name 填啥？我自己起？还是这就是 'DeepRouter'？"
- "Base URL 末尾要加斜杠吗？带不带 /v1？"
- "API Key 在哪里找？这把我已经看过了的能再用一次吗？"
- "Model: deeprouter 是固定字符串还是我自己选的？为什么不能填 gpt-4o？"

每一个疑问都是流失点。

类似的密度不足出现在：
- `/welcome` wizard 的 persona / brand / client 卡片描述
- `/keys` Simple 模式的 6 张 purpose 卡片
- `/wallet` 充值金额输入框、CNY/USD 切换、支付方式选择
- `/playground` 模型下拉

### 1.4 目标

让 casual persona 用户在每一个**配置输入框 / 选择按钮 / 操作字段**旁边都能看到：

1. **一句白话解释**（这是什么、为什么需要、填什么）
2. **如果可能，给出具体例子或反例**（"别加斜杠"、"别给别人看"）
3. **配置失败 / 拿不准时的求助路径**（视频、微信群、AI 助手）

**Dev persona 用户**：同样的解释默认收起（不打扰），hover `?` 才弹出，保持高密度信息呈现。

---

## 2. 核心决策

### 2.1 引入 `<FieldHint>` 标准组件

**定位**：所有需要给 casual 用户解释的字段都用同一个组件，文案密度全站一致。

#### API 设计

```tsx
<Label className='flex items-center gap-1.5'>
  Base URL
  <FieldHint expanded={persona === 'casual'}>
    粘到 Cherry Studio "API 地址" 字段，末尾不要加斜杠。
  </FieldHint>
</Label>
```

#### 两种渲染模式

| 模式 | 触发条件 | 渲染 |
|---|---|---|
| **expanded**（casual 默认）| `persona === 'casual'` OR `forceExpanded` | inline 浅色文字直接显示在字段下方 |
| **collapsed**（dev 默认）| `persona === 'dev' \|\| 'team' \|\| 'admin'` | label 边一个 `?` 图标，hover/tap 显示 popover |

#### 内容规范

每条 hint 都要：
- 不超过 2 句话
- 用日常语言（非术语）
- 包含一个**具体动作**或**具体反例**
- 中英文都要写（i18n key）

**示例对比**：

| ❌ 不够具体 | ✅ 足够具体 |
|---|---|
| "API Key 是你的密钥" | "你的专属密钥，**别给别人看**。在客户端 'API Key' 一栏粘贴" |
| "选一个使用方式" | "选 **聊天** 适合用 Cherry Studio；选 **编程** 适合用 Cursor" |
| "选择支付方式" | "支付宝 / 微信适合国内用户，无需绑定信用卡" |

### 2.2 Casual 模式下隐藏高级配置项

PR #3 已经按 persona 分了 sidebar，PR #2 + #3 已经按 admin/非 admin 隐藏 Group / Cross-group。这一层是**进一步对 casual 隐藏**（非 admin 但也非 casual 的 dev 用户保留可见）：

| 字段 | 当前位置 | Casual | Dev | Admin |
|---|---|---|---|---|
| `/wallet` 货币切换（CNY/USD/Tokens）| 顶部 toggle | ❌ 隐藏（强制 CNY 或账户偏好）| ✅ 看到 | ✅ |
| `/wallet` 自定义金额 | 数字输入 | ❌ 隐藏，只显示预设档位 | ✅ | ✅ |
| `/wallet` 兑换码 redemption | 表单底部 | ❌ 隐藏（高级运营功能）| ✅ | ✅ |
| `/keys` Advanced 模式入口 | 弹窗第二张卡片 | ❌ 入口隐藏，只能用 Simple | ✅ | ✅ |
| `/playground` 模型参数（temperature 等）| 折叠区 | ❌ 默认完全隐藏 | 折叠可展开 | 同 |
| `/profile` 通知设置 / Webhook | 第二个 Tab | ❌ Tab 完全隐藏 | ✅ | ✅ |
| `/profile` 边栏自定义 | Sidebar 卡片 | ❌ 隐藏（保持预设）| ✅ | ✅ |

**实现**：在每个对应组件里 `usePersona() === 'casual'` 检测，包一层条件渲染。

### 2.3 全局求助浮窗

**位置**：除登录 / 注册 / 公开 pricing 页外，所有 `_authenticated/*` 页面右下角浮一个 FAB（Floating Action Button）。

```
┌─ 右下角浮窗（默认收起） ───────┐
│   ❓  ← 浮动图标，永远在那       │
└────────────────────────────────┘

点击展开：
┌────────────────────────────────┐
│  ❓ 搞不定？                    │
│                                 │
│  📺 看 3 分钟视频教程             │ ← 链 admin 配置的 workshop 视频 URL
│  💬 加微信群（带二维码）          │
│  📷 查看图文教程                  │ ← 跳 /onboarding/<current-context>
│  ✉️ 写信问 support@...           │
│  🤖 问 AI 助手（v2 再做）         │
└────────────────────────────────┘
```

#### Admin 后台可配项

- **视频教程 URL**（B 站 / YouTube 嵌入 URL）
- **微信群二维码图片 URL**
- **微信群号文字**
- **支持邮箱**

放在 `setting/operation_setting` 里，admin UI 可改。

#### Casual 用户的视觉权重

- Casual persona：浮窗图标默认带"NEW"红点，吸引点击
- Dev / team：浮窗图标更暗，不打扰

### 2.4 视频教程入口的多点植入

Workshop 录的视频在产品里至少有 4 个入口：

1. **求助浮窗第一项**（前面提到）
2. **`/welcome` wizard 顶部 banner**：注册完进 wizard 时一行"第一次来？[3 分钟入门视频]"
3. **`/onboarding/<slug>` 教程页顶部**：每个客户端教程都可以贴对应视频
4. **`/keys` 创建成功 dialog**（PR #2 已有的 success dialog）底部加"[看视频跟着配]"

视频 URL 都从同一个 admin 配置项读。

### 2.5 文案密度规则总结

| 元素 | Casual 看到的 | Dev 看到的 |
|---|---|---|
| 字段 label | "API Key（你的专属密钥）" | "API Key" |
| 字段说明 | 字段下方常驻 1-2 句白话 | `?` hover 才弹 |
| 字段示例 | 占位符里写具体例子 | 同 |
| 错误提示 | "粘贴时多了空格，删一下再试" | "401 Unauthorized" |
| 成功提示 | "✅ 成功了！现在你可以..." | "OK" |
| 推荐项 badge | "推荐"、"最常用" | 不显示或淡色 |

---

## 3. 实施范围：哪些页面/字段必须加 hint

### 3.1 必做（P0）

#### `/welcome` wizard（PR #8）

| 字段 | Casual 看到的 hint |
|---|---|
| Persona 卡片 Casual | "**你属于这类**：用 AI 聊天、写作、翻译、画图等，不写代码。我们推荐你用 Cherry Studio。" |
| Persona 卡片 Dev | "你属于这类：写代码、跑脚本、自己调 API。" |
| Persona 卡片 Team | "你属于这类：给团队配置共享 Key、做企业接入。" |
| Brand 卡片 Claude | "Anthropic 出品的 Claude，写作和写代码最强。" |
| Brand 卡片 OpenAI | "OpenAI 出品的 GPT-4o，通用最稳、生态最广。" |
| Brand 卡片 Gemini | "Google 出品，多模态强，长上下文便宜。" |
| Brand 卡片 DeepSeek | "国产，性价比高，写代码不错。" |
| Brand 卡片"无偏好" | "我们帮你自动选最合适的（一般推荐 Claude）。" |
| Client 卡片 Cherry Studio | "**推荐 casual 用户**：双击装好就能用的桌面 AI 客户端。" |
| Client 卡片 Cursor | "代码编辑器，写代码时用。" |
| Client 卡片 Claude Code | "命令行工具，写代码时用。" |
| Client 卡片 Code | "在你自己的 Python / Node 项目里调用 API。" |
| Client 卡片 Playground | "**最快试一下**：浏览器里直接聊，不用装东西。" |
| Client 卡片 Just look | "先看看 dashboard，之后再选客户端。" |

#### `/keys` Create API Key (Simple Mode)

PR #2 已有的 6 张 purpose 卡片，文案需要 casual 加强：

| Purpose | Casual 看到的 hint |
|---|---|
| 💬 Chat / Writing | "聊天、写作、翻译、学习。适合用 Cherry Studio / Chatbox。" |
| 💻 Coding | "AI 帮你写代码、改 bug。适合用 Cursor / Claude Code。" |
| 🎨 Image | "用文字生成图片。试试 DALL·E 或 Flux。" |
| 🎬 Video | "用文字生成视频片段。模型还在 beta。" |
| 🎙️ Voice | "语音转文字、文字转语音、配音。" |
| ⚡ All (Auto) | "我们根据你调用什么自动选。**最灵活**但要注意费用。" |

#### `/onboarding/<slug>` 教程页

所有 6 个客户端教程页的 Base URL / API Key / Model 字段都按 §2.5 规范加 hint。

**示例**（`/onboarding/cherry-studio`）：

```markdown
3. 在 Cherry Studio 设置里填这 3 个字段：

   📝 **Name 名称**：`DeepRouter`
   💡 你给这个 AI 服务起的名字，自己看的，叫啥都行。

   🌐 **Base URL / API 地址**：
   `https://deeprouter.ai/v1` [📋]
   💡 末尾必须有 `/v1`，**别加多余的斜杠**。

   🔑 **API Key**：
   `sk-xxxx...` [📋]
   💡 你的专属钥匙，**别给别人看**。
      离开本页就看不到了，可去 [我的 Keys](/keys) 重新生成。

4. 在客户端选 **DeepRouter** 这个服务商，模型填：
   `deeprouter` [📋]
   💡 一定要拼对。我们会根据注册时选的偏好自动用对应 AI。

5. 试一句"你好"，看到回复就成功了 ✅
```

#### `/wallet` 充值

| 字段 | Casual 看到的 hint |
|---|---|
| 预设金额按钮 | "约 N 次对话" 已有（PR #4），保留 |
| 自定义金额 | ❌ Casual 模式隐藏 |
| 货币切换 | ❌ Casual 模式隐藏 |
| 支付宝 | "🇨🇳 国内推荐，扫码即付" |
| 微信支付 | "🇨🇳 国内推荐，扫码即付" |
| Stripe | "💳 信用卡（Visa / Mastercard）" |
| 兑换码 | ❌ Casual 隐藏 |

### 3.2 可做（P1）

- `/playground` 模型下拉旁边加 "如果不知道选啥，[deeprouter] 一般够用了"
- `/profile` 语言切换旁边："这里改的是界面语言，不影响 AI 回复语言"
- `/dashboard/overview` 余额数字旁边："这里是你剩余的钱，会随调用减少"

### 3.3 不做

- `/system-settings/*` — admin only
- `/channels` / `/models metadata` — admin only
- `/api-docs` / OpenAPI viewer — dev 用户专属，文档密度本来就高

---

## 4. 关键组件设计

### 4.1 `<FieldHint>` 组件

**位置**：`web/default/src/components/ui/field-hint.tsx`

```tsx
import { HelpCircle } from 'lucide-react'
import {
  Popover, PopoverContent, PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { usePersona } from '@/hooks/use-persona'

type FieldHintProps = {
  children: React.ReactNode
  /** Force open regardless of persona — for super-important hints */
  forceExpanded?: boolean
  /** Force collapsed regardless of persona — for hints only experts need */
  forceCollapsed?: boolean
  className?: string
}

export function FieldHint({
  children, forceExpanded, forceCollapsed, className,
}: FieldHintProps) {
  const persona = usePersona()
  const expanded =
    forceExpanded || (persona === 'casual' && !forceCollapsed)

  if (expanded) {
    return (
      <p className={cn(
        'text-muted-foreground text-xs leading-snug mt-1',
        className
      )}>
        💡 {children}
      </p>
    )
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type='button'
          className='text-muted-foreground/60 hover:text-foreground inline-flex h-4 w-4 items-center justify-center'
          aria-label='Help'
        >
          <HelpCircle className='h-3.5 w-3.5' />
        </button>
      </PopoverTrigger>
      <PopoverContent className='max-w-xs text-xs leading-snug'>
        💡 {children}
      </PopoverContent>
    </Popover>
  )
}
```

### 4.2 `<HelpFab>` 全局求助浮窗

**位置**：`web/default/src/components/help-fab.tsx`

挂在 `AuthenticatedLayout` 里，所有 _authenticated/* 页面都看得到。

```tsx
export function HelpFab() {
  const persona = usePersona()
  const { status } = useStatus()
  const [open, setOpen] = useState(false)

  // Admin-configured URLs (read from status object)
  const videoUrl = status?.help_video_url || ''
  const wechatQR = status?.help_wechat_qr || ''
  const wechatId = status?.help_wechat_id || ''
  const supportEmail = status?.help_support_email || 'support@deeprouter.ai'

  // Casual users get NEW dot until they've clicked once
  const showNewDot =
    persona === 'casual' && !localStorage.getItem('dr_help_seen')

  return (
    <div className='fixed bottom-6 right-6 z-40'>
      {/* ... FAB icon + popover with options ... */}
    </div>
  )
}
```

### 4.3 视频 URL admin 配置

新增到 `setting/operation_setting/` 或 status 响应：

| Key | 说明 |
|---|---|
| `help_video_url` | Workshop 视频嵌入 URL |
| `help_wechat_qr` | 微信群二维码图片 URL |
| `help_wechat_id` | 微信群号文字 |
| `help_support_email` | 客服邮箱（默认 support@...）|

Admin 在 System Settings → Operations 里改。

### 4.4 Casual mode 字段隐藏 helper

**位置**：`web/default/src/hooks/use-casual-mode.ts`

```tsx
export function useIsCasual(): boolean {
  return usePersona() === 'casual'
}

export function HideForCasual({
  children,
}: { children: React.ReactNode }) {
  if (useIsCasual()) return null
  return <>{children}</>
}
```

让组件里写：

```tsx
<HideForCasual>
  <CurrencyToggle />
  <CustomAmountInput />
</HideForCasual>
```

---

## 5. 工时与排期

### Phase 1（PR #9 主体，~5 天）

| 子任务 | 工时 |
|---|---|
| `<FieldHint>` 组件 + storybook-style examples | 0.5 d |
| `useIsCasual` + `<HideForCasual>` helper | 0.5 d |
| `<HelpFab>` 全局求助浮窗 + admin 配置项 | 1 d |
| `/welcome` wizard hint 文案落地 | 0.5 d |
| `/keys` Simple 6 卡片 hint 增强 | 0.5 d |
| `/onboarding/<slug>` 教程页 hint 增强（6 个 client）| 1 d |
| `/wallet` casual 字段隐藏 + 支付方式 hint | 0.5 d |
| i18n（en + zh 大约 60-80 keys）| 0.5 d |
| **合计** | **~5 d** |

### Phase 2（PR #10，~2 天，可选）

- `/playground` casual hint
- `/dashboard/overview` casual hint
- `/profile` casual hint + Tab 隐藏

### 依赖（运营 / 内容）

| 项 | 状态 | 谁做 |
|---|---|---|
| Workshop 视频 URL | 待录 | 你 |
| 微信群二维码 | 待建 | 你 |
| 微信群号 | 待建 | 你 |
| 客户端截图（前面 PR #4 PRD 已列）| 待拍 | platform 团队 |

**工程层（PR #9）不依赖以上内容**——文案 / URL 都从 admin 配置读，没填就 fallback 到 placeholder（"视频教程上线中"）。

---

## 6. 兼容性与降级

### 6.1 现有用户

- 老用户 persona 是 `dev`（PR #3 legacy fallback）→ 看不到 inline hint，全 hover popover
- 老用户 dashboard / wizard 体验完全不变

### 6.2 Persona 切换时

用户在 /profile 切换 persona = casual → dev → 立刻所有 FieldHint 切换渲染模式（hooks reactive）

### 6.3 浮窗位置冲突

- 跟现有 sonner toast 冲突时，toast 高 6rem 起，FAB 放 6rem 处可见
- 移动端 FAB 缩到 bottom-4 right-4 避免遮挡 SheetFooter

### 6.4 视频 URL 未配置

- 求助浮窗第一项 "看 3 分钟视频教程" 显示但点击 disabled，提示 "视频教程整理中"
- 不显示死链

---

## 7. UX 关键文案（i18n keys 摘录）

完整列表在 PR #9 实现时一次性加进 `i18n/locales/{en,zh}.json`。摘 10 条关键的：

| Key | en | zh |
|---|---|---|
| `Help — Stuck?` | "Stuck? We're here to help." | "搞不定？我们帮你。" |
| `Watch the 3-min tutorial video` | "📺 Watch the 3-minute tutorial" | "📺 看 3 分钟视频教程" |
| `Join WeChat group` | "Join our WeChat group" | "加微信群求助" |
| `Email support` | "Email support" | "写信问客服" |
| `Help video coming soon` | "Help video coming soon" | "视频教程整理中" |
| `Casual recommends` | "Recommended for casual users" | "推荐普通用户使用" |
| `Hint: hover for details` | "Hover for details" | "鼠标停留看说明" |
| `Hint base url` | "Paste into Cherry Studio's API URL field. No trailing slash." | "粘到 Cherry Studio 的 API 地址栏。末尾不要加斜杠。" |
| `Hint api key` | "Your private key. Don't share it. Hidden after you leave this page." | "你的专属钥匙，别给别人看。离开本页就看不到了。" |
| `Hint model` | "Type exactly this. We auto-route to the right AI based on your signup." | "原样填入。我们根据你注册时的选择自动路由。" |

---

## 8. 风险 & 开放问题

### 8.1 风险

| 风险 | 缓解 |
|---|---|
| Inline hint 让页面变长，casual 用户也烦 | hint 限制 2 句话，颜色淡，不喧宾夺主 |
| Casual 隐藏的字段，用户反过来抱怨"少功能" | 在 Persona 选择卡片上明示 "Casual = 简化版" |
| Help FAB 跟其他 FAB / chat widget 冲突 | 暂无其他 FAB，预留位置 |
| 视频 URL 未配置时入口尴尬 | placeholder "整理中" 而非空白 |
| i18n key 暴涨 | 用 namespace 分组（hints.field.*）|

### 8.2 已决（v0.1）

- ✅ casual 模式 inline hint 默认展开
- ✅ dev 模式 popover hover 展开（不打扰）
- ✅ FAB 全站浮窗
- ✅ 视频 URL 从 admin 配置读
- ✅ Workshop / 视频不靠产品自己生成，运营负责

### 8.3 待拍板

- [ ] FAB 位置：右下角固定 vs 顶 nav 一颗按钮
- [ ] Casual 用户的 NEW 红点是看了一次后消失，还是每个新功能都重新点亮
- [ ] 视频是嵌入 iframe 还是跳外链（B 站 iframe 国内速度好）
- [ ] 隐藏字段的 casual 用户怎么"升级"为 dev 看到完整功能 — 走 profile 改 persona 即可？是否要在 hint 文案里告诉他

---

## 9. 关联 PR / 文档

- PRD: [docs/PRD.md](../PRD.md) — 总规
- PRD: [docs/tasks/onboarding-prd.md](onboarding-prd.md) — 注册旅程
- PRD: [docs/tasks/api-key-simple-advanced-prd.md](api-key-simple-advanced-prd.md) — Key 创建
- PR #2: API Key Simple/Advanced 双模式（已合）
- PR #3: Persona 系统（已合）
- PR #4: 客户端教程页 + 试用额度（已合）
- PR #5: Usage Logs / Playground 改造（已合）
- PR #6: Dashboard hero + Pricing ratio cleanup（已合）
- PR #7: Onboarding PRD（已合）
- PR #8: 注册 wizard + welcome page（已合）
- **PR #9（本 PRD 落地）**：FieldHint 组件 + casual 全场 UX 加密 + help FAB

---

## 10. 文案密度对照表（完整版示意）

### 10.1 注册到首次调用的 5 大配置时刻

每个时刻 casual 应该看到什么 vs dev 看到什么：

#### Moment 1: 注册表单
```
casual: "邮箱 (我们用来给你发账号确认)"  ← inline hint
dev:    "Email"  ← 简洁
```

#### Moment 2: Welcome wizard Step 2 (persona)
```
casual: 卡片下方常驻 "💡 这类用户大概有 60%，包括 PM、设计师、学生..."
dev:    卡片下方什么都没有，hover 才看到
```

#### Moment 3: Cherry Studio 教程页 Base URL
```
casual: "https://deeprouter.ai/v1 [📋]"
        "💡 粘到 Cherry Studio 的 'API 地址' 字段，末尾不要加斜杠"
dev:    "https://deeprouter.ai/v1 [📋] ?"
        (hover ? 才看 hint)
```

#### Moment 4: /wallet 充值预设按钮
```
casual: [¥30 约 3000 次对话]  ← PR #4 已有
        💡 选这一档够你用 1-2 个月
        (隐藏自定义金额 + 货币切换 + 兑换码)
dev:    [¥30] [¥100] [¥300] + 自定义金额 + 货币切换 + 兑换码
```

#### Moment 5: /keys 创建 Simple 6 卡片
```
casual: 每张卡片下方常驻 "💡 选这个 → 适合 Cherry Studio · 约 ¥1 聊 100 句"
dev:    卡片下方只有 estimate ("¥1 for 100 chats")
```

