# PRD — API Key 创建：Simple / Advanced 双模式

> **Status**: 📝 Draft v0.2 · 待评审
> **Author**: Lightman + Claude
> **Date**: 2026-05-16
> **Owner**: DeepRouter Frontend + Platform
> **Parent**: [`docs/PRD.md`](../PRD.md) §B2 用户控制台
> **范围**: `/keys` 路由下的 Create / Update API Key 表单（`web/default/src/features/keys/`）

---

## 0. 版本变更

### v0.2（2026-05-16）
- **核心变更**：Simple Mode 主轴从"品牌选择"改为"**目的选择**"（聊天 / 编程 / 图像 / 视频 / 语音 / 全部），品牌降级为可选过滤
- 新增 §3.5 Auto 模式价格上限设计
- 新增 §4.4 Dashboard 空状态引导
- §3.4 价格展示明确：per 1K tokens + 体感字符串（"¥1 聊 100 句"）
- §5.1 alias 表升级为 `purpose × brand → target model` 二维映射
- 关闭 3 个开放问题（价格单位 / Auto 上限 / 引导方式）

### v0.1（2026-05-16）
- 初稿：Simple / Advanced 双模式 + 品牌即模型 + Virtual Model Alias

---

## 1. 背景

### 1.1 现状

DeepRouter 当前的 "Create API Key" 表单（`api-keys-mutate-drawer.tsx`）继承自上游 `new-api`，是给**多级代理运营场景**设计的：

- 主区第一个字段就是 `Group` 下拉，右侧附带 `5x Ratio` 红色 Badge
- 必填项里有 `Quota (USD)`、`Expiration Time`
- 高级区有 `Model Limits`、`Allow IPs`、`Allow Subnets`、`Cross-group retry` 等

对终端付费用户而言，这一屏暴露的概念全部是**计费/运维语义**（倍率、配额、子网、跨组重试），而不是**他要做的事**（**用 AI 干什么、能不能用、怎么接到客户端**）。

### 1.2 问题

> 注册 → 创建 Key → 第一次成功调用，是 DeepRouter 营收漏斗最关键的一步。当前表单在第二字段（Group）就开始劝退非技术用户。

具体问题：

| 当前字段 | 用户疑惑（非技术） | 真实需求 |
|---|---|---|
| `Group` + `5x Ratio` | "5x 是不是被宰 5 倍？为什么要选？" | 不应该看见 |
| `Quota (USD)` | "这跟我账户余额什么关系？" | 默认用账户余额即可 |
| `Expiration Time` | "永久 vs 临时，我该选哪个？" | 默认永久 |
| `Model Limits`（折叠） | "不勾会怎么样？" | 默认按目的自动选 |
| `Allow IPs / Subnets` | 完全看不懂 | 不应该看见 |

**结果**：用户看完七个字段还是不知道下一步该怎么做，更不知道**自己应该选哪个模型来达成目的**。

### 1.3 关键产品洞察

#### 洞察 1：用户心智是"目的"，不是"品牌"或"模型版本号"

| 用户脑内想的 | 不是 |
|---|---|
| "我想让 AI 帮我写代码" | ~~"我要用 Claude"~~ / ~~"我要用 claude-sonnet-4-7"~~ |
| "我想生成一张图" | ~~"我要用 DALL-E-3"~~ |
| "我想做翻译" | ~~"我要用 GPT-4o"~~ |
| "我想做客服机器人，便宜稳定就行" | ~~"我要用 gpt-4o-mini"~~ |

**目的（purpose）比品牌（brand）更接近用户心智**。品牌偏好是高阶用户才有的认知 — 大多数用户其实是被推荐着用品牌的。模型版本号则完全是开发者语言。

#### 洞察 2：模型版本号对用户应该是黑盒

- ChatGPT 给普通用户的选项是 `GPT-4o` / `o1`，不是 `gpt-4o-2024-08-06`
- Cursor 模型选择器是 `Claude / GPT / Gemini` 卡片
- Perplexity 是 `Best / Fast / Pro`
- OpenRouter 有 `openrouter/auto` 自动路由

DeepRouter 应该更进一步：让用户**只表达目的**，平台负责挑模型。模型升级 / 替换对用户透明。

#### 洞察 3：DeepRouter 接入 40+ 上游 ≠ 给用户暴露 40+ 选项

聚合的价值不是"让用户选得更多"，而是"**让用户不用选**"。Simple Mode 必须把"40+ 上游 + 几百个模型"折叠为 **6 个目的卡片**。

---

## 2. 用户画像

按 AI API gateway 通用经验，注册用户的实际分布：

| 占比 | 画像 | 怎么用 DeepRouter | 看得懂什么 | 需要的字段 |
|---|---|---|---|---|
| **~60%** | **非技术 / 半技术**<br/>内容创作者、PM、翻译/学习党、AI 玩家 | 注册 → 拿 key → 粘到 Cherry Studio / Chatbox / LobeChat / 沉浸式翻译 / Raycast / NextChat | 用 AI 干什么、大致价格 | Name + 目的 |
| **~30%** | **个人开发者**<br/>写脚本、跑 Cursor / Claude Code、副业小项目 | API base url + key 接进代码 | 目的、模型、context length、rate limit | + 品牌偏好 / 配额 |
| **~10%** | **团队/企业技术负责人** | 多 key 隔离、cost 监控、IP 白名单 | 全部 | + IP 白名单 / 子网 / 跨组重试 / 独立配额 |

**结论**：当前表单只服务 10% 用户，对剩余 90% 是认知噪音。

---

## 3. 核心产品决策

### 3.1 Simple vs Advanced 双模式

| | Simple Mode（默认） | Advanced Mode |
|---|---|---|
| **目标用户** | 60% 非技术 + 30% 个人开发者 = **90%** | 10% 团队/企业 |
| **表单字段** | 2 个主字段：Name（可选）+ **目的**（必选） + 可选品牌偏好 | 全部字段 |
| **AI 选择粒度** | **目的级**（聊天 / 编程 / 图像 / 视频 / 语音 / 全部） | 模型级（具体型号多选） |
| **配额** | 用账户余额；Auto 目的有"消费档位"上限 | 可设独立配额 |
| **过期** | 永久 | 可设过期时间 |
| **IP / 子网** | 不暴露 | 可设 |
| **Group / 倍率** | 不暴露（后端按账号默认 group） | 可选（语义化标签，admin 才看数字倍率） |
| **创建后** | 立刻展示 key + base url + 客户端接入教程 | 同上 + 完整 cURL 示例 |

**切换**：表单顶部一个 `Simple ⇄ Advanced` Toggle。**默认 Simple**，浏览器记住上次选择。

### 3.2 Simple Mode 的"目的选择"是怎么落地的

**核心机制**：`purpose × brand → target model` 二维 alias 映射 + 客户端通用别名。

#### 用户视角

用户在 Simple Mode 选 `编程` + 品牌偏好 `无` → 拿到 key 后，在客户端模型字段填：

```
model: "deeprouter"
```

或者更细化（可选）：

```
model: "deeprouter-coding"   ← 显式声明 coding 目的
model: "deeprouter-image"    ← 用同一把 key 临时画图（如果 model_limits 允许）
```

后端按这把 key 创建时绑定的 `purpose` + `brand_preference` 路由到具体模型。

#### 后端 Alias 表（admin 可配置）

| Purpose | Brand | Target Model | 备注 |
|---|---|---|---|
| `chat` | auto | `claude-sonnet-4-7` | 通用聊天，平衡质量价格 |
| `chat` | claude | `claude-sonnet-4-7` | |
| `chat` | openai | `gpt-4o` | |
| `chat` | gemini | `gemini-2.x-pro` | |
| `chat` | deepseek | `deepseek-v3` | |
| `coding` | auto | `claude-sonnet-4-7` | Claude 是当前 coding SOTA |
| `coding` | claude | `claude-sonnet-4-7` | |
| `coding` | openai | `gpt-4o` | |
| `coding` | deepseek | `deepseek-coder-v3` | |
| `image` | auto | `flux-pro` | 综合质量价格 |
| `image` | openai | `dall-e-3` | |
| `image` | (none) | `flux-pro` | 图像独立厂商 |
| `video` | auto | `veo-3` | |
| `voice` | auto | `whisper-1` (STT) + `tts-1-hd` (TTS) | 按 task 类型再分流 |
| `all` | auto | — | 不绑定 alias，按调用时 model 字段路由 |

**关键性质**：
- Admin 后台可改 alias 映射，platform 团队对模型市场更敏感
- 模型升级（Claude 4.7 → 4.8）对用户完全透明
- 用户充值后没有"我必须重新研究新模型名"的迁移成本

#### Model Limits 自动生成

创建 Simple key 时，后端根据 `purpose` 自动生成 `model_limits`：

```
purpose=coding  → model_limits = [claude-*, gpt-4*, deepseek-coder-*, ...所有适合 coding 的真实模型 + 该 purpose 的所有 alias]
purpose=image   → model_limits = [dall-e-*, flux-*, sdxl-*, ...所有图像生成模型]
purpose=all     → model_limits = [] (不限)
```

这样即使用户在客户端填具体 model 名（如 `claude-sonnet-4-7`），只要在允许集合内也能调用。

### 3.3 Group / 倍率怎么处理

| 模式 | Group 字段 | 倍率展示 |
|---|---|---|
| **Simple** | 完全隐藏，后端按用户账号默认 group 走 | 不展示 |
| **Advanced（普通用户）** | 显示 `通道` 字段，选项语义化为 `⚡ 高速 / 🎯 标准 / 💰 经济` | 只展示中文描述，不展示 `5x` 数字 |
| **Advanced（admin / root）** | 显示原始 group 名 + 倍率数字 | 展示 `5x Ratio` |

倍率 Badge 是当前最迫切要止血的点（Phase 0 立即处理）。

### 3.4 价格展示

**两个原则**：

1. **单位**：统一 `per 1K tokens`（中国用户更习惯 1K；OpenAI / Anthropic 官网用 1M，但 1M 数字对小白门槛高）。币种跟随用户账户币种（DeepRouter 已有 `getCurrencyDisplay()`）
2. **加体感字符串**：体感比标价更直观，admin 配置（不要算法估算，容易翻车）

| 场景 | 标价（卡片小字） | 体感（卡片主显） |
|---|---|---|
| 聊天 / 写作 / 翻译 | ¥0.01–0.10 / 1K tokens | 约 **¥1 聊 100 句** |
| 编程 / Coding | ¥0.02–0.15 / 1K tokens | 约 **¥1 改 50 段代码** |
| 图像 | ¥0.3–1.0 / 张 | 约 **¥10 生成 20 张** |
| 视频 | ¥3–10 / 段 | 约 **¥50 生成 10 段 5 秒视频** |
| 语音 | ¥0.05 / 分钟 | 约 **¥10 转写 200 分钟** |
| 全部 (Auto) | 按使用计费 | **从最便宜的开始路由** |

**位置**：体感字符串展示在 Simple Mode 卡片正面；标价数字作为小字附在下面。详细按模型分价（input / output / cached）放在独立 `/pricing` 页。

### 3.5 Auto 模式价格上限（防爆账单）

Auto 模式最大的恐惧是"一觉醒来账单几千"。**默认必须给安全网**。

#### "全部 (Auto)" 卡片下加消费档位选择：

```
⚡ 全部 (Auto)
按任务自动路由

最高消费档位：
○ 💰 经济档     ¥0.001 – 0.02 / 1K       只走便宜模型，绝不上 Opus/o1
● 🎯 标准档     ¥0.001 – 0.10 / 1K       覆盖大部分场景，避免顶配  ← 默认
○ 🚀 高级档     ¥0.001 – 0.30 / 1K       含 Claude Opus、GPT-4o
○ 👑 顶配档     无上限                    含 o1、Opus、Gemini Ultra
```

**实现要点**：
- 默认 "标准档"
- 顶配档要用户显式确认（点击时弹 confirm dialog "可能产生较高费用，确认开启？"）
- 技术上：每个模型按 `(input_price + output_price) / 2` 算单价档位，存在 `model_pricing` 表
- 路由时只在所选档位内挑模型（如果该档位无可用模型则降级返回 error，不偷偷升级到更贵档位）

这本质是 **price-cap routing**，比 quota 限额更友好（quota 是"上限触顶报警"，price-cap 是"每次调用都安全"）。

---

## 4. UI 设计稿（文字版）

### 4.1 Simple Mode（默认）

```
┌─ Create API Key ─────────────────────  Simple ●  Advanced ○ ┐
│                                                              │
│  Name (optional)                                             │
│  ┌────────────────────────────────────────────────────┐     │
│  │ default-key                                        │     │
│  └────────────────────────────────────────────────────┘     │
│                                                              │
│  你想用这把 Key 做什么？                                       │
│  ┌──────────────────┬──────────────────┬──────────────────┐ │
│  │ 💬 聊天 / 写作    │ 💻 编程 / Coding │ 🎨 图像生成       │ │
│  │ 翻译、创作、对话   │ 代码补全、AI 编程 │ 文生图、改图       │ │
│  │ ¥1 聊 100 句     │ ¥1 改 50 段代码  │ ¥10 生成 20 张    │ │
│  │                  │        ✓         │                  │ │
│  └──────────────────┴──────────────────┴──────────────────┘ │
│  ┌──────────────────┬──────────────────┬──────────────────┐ │
│  │ 🎬 视频生成      │ 🎙️ 语音 / TTS    │ ⚡ 全部 (Auto)   │ │
│  │ 文生视频、改视频   │ 转写、配音、克隆   │ 自动按任务路由     │ │
│  │ ¥50 生成 10 段   │ ¥10 转写 200 min│ 从最便宜开始       │ │
│  └──────────────────┴──────────────────┴──────────────────┘ │
│                                                              │
│  ▽ 偏好某家厂商？(可选)                                        │
│  [无偏好]  [Claude]  [OpenAI]  [Gemini]  [DeepSeek]          │
│                                                              │
│                                  [ Cancel ]  [ Create Key ] │
└──────────────────────────────────────────────────────────────┘
```

**选 "全部 (Auto)" 时**，UI 展开消费档位选项（见 §3.5）。

### 4.2 创建成功页（关键页面，决定首次调用转化）

```
┌─ ✅ Key created ─────────────────────────────────────────────┐
│                                                              │
│  Your new API key (only shown once, copy it now!)            │
│  ┌────────────────────────────────────────────────────┐ 📋  │
│  │ sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx   │     │
│  └────────────────────────────────────────────────────┘     │
│                                                              │
│  Base URL                                                    │
│  ┌────────────────────────────────────────────────────┐ 📋  │
│  │ https://deeprouter.ai/v1                           │     │
│  └────────────────────────────────────────────────────┘     │
│                                                              │
│  Model to use in your client                                 │
│  ┌────────────────────────────────────────────────────┐ 📋  │
│  │ deeprouter   (routes to Claude Sonnet for coding)  │     │
│  └────────────────────────────────────────────────────┘     │
│                                                              │
│  ── How to use ────────────────────────────────────────      │
│                                                              │
│  [Cherry Studio]  [Chatbox]  [LobeChat]                      │
│  [Cursor]  [Claude Code]  [Python / Node code]               │
│       ↑ 点任一卡片，跳到截图教程页                            │
│                                                              │
│                                          [ Done ]            │
└──────────────────────────────────────────────────────────────┘
```

### 4.3 Advanced Mode

保留当前 form 的全部字段，但仍做以下整改：

1. Group 字段标签改为「通道」，选项语义化（`⚡ 高速 / 🎯 标准 / 💰 经济`），倍率 Badge 仅 admin 可见
2. Model Limits 从 Advanced 折叠区提到主区，多选项后附**最终单价**（已含倍率），不再露 ratio 数字
3. Quota 字段加 inline 帮助："留空则使用账户余额"
4. IP 白名单 / Subnet / Cross-group retry 保留在 Collapsible 内

### 4.4 Dashboard 空状态引导（替代 multi-step tour）

注册首次进入 `/keys` 路由时：

```
┌──────────────────────────────────────────────────────────┐
│                                                          │
│                       🔑                                 │
│              Create your first API key                   │
│                                                          │
│           30 秒拿到 key，立即接入 AI 客户端                │
│                                                          │
│                   [ Get Started → ]                      │
│                                                          │
│         💡 不需要懂技术，跟着引导走就能完成                │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

**关键设计**：
- 大号 CTA 占满空 list，不是 toast / overlay
- 点击直接打开 Simple Mode 创建抽屉（不打开 toggle 选择）
- 创建成功后跳 success page，**该 page 即 onboarding 终点**
- 已有 key 的用户从未见过此空状态
- 不做 multi-step overlay tour（Linear / Notion / Vercel 都已证明空状态 CTA 转化率更高）

---

## 5. 后端改动要点

### 5.1 Purpose × Brand Alias 表

新建 `setting/model_alias.go` + `model/model_alias.go`：

```go
type ModelAlias struct {
    ID          uint   `gorm:"primaryKey"`
    Purpose     string `json:"purpose" gorm:"index"`     // chat / coding / image / video / voice
    Brand       string `json:"brand" gorm:"index"`       // claude / openai / gemini / deepseek / auto
    TargetModel string `json:"target_model"`              // 真实模型名
    Description string `json:"description"`               // admin 视角说明
    Priority    int    `json:"priority"`                  // 多 alias 命中时的优先级
    PriceTier   string `json:"price_tier,omitempty"`     // economy / standard / premium / ultra (用于 Auto 价格上限)
}
```

- Admin 后台 CRUD（参考现有 channel / pricing 管理）
- 提供 seed 数据（首次部署自动建表 + 默认映射）
- Relay 层进入 distribution 前先 resolve alias

### 5.2 Key 创建 API 扩展

`POST /api/token`（创建 key）接受新字段：

```go
type CreateTokenRequest struct {
    // 原有字段 ...
    SimplePurpose   string `json:"simple_purpose,omitempty"`    // chat / coding / image / video / voice / all
    SimpleBrand     string `json:"simple_brand,omitempty"`      // 可选品牌偏好
    SimplePriceTier string `json:"simple_price_tier,omitempty"` // 仅 purpose=all 时有效
}
```

后端逻辑：
- `simple_purpose` 非空 → 按 alias 表自动生成 `model_limits`（覆盖该 purpose 下所有可用模型）
- `simple_purpose=all` + `simple_price_tier` → 创建一个 price-tier-bounded 路由策略
- 三个字段全空 → Advanced 行为，原逻辑不变

### 5.3 Pricing 区间 API

`GET /api/pricing/purpose-summary` 返回：

```json
[
  {
    "purpose": "chat",
    "label": "聊天 / 写作",
    "icon": "💬",
    "desc": "翻译、创作、对话",
    "price_range": "¥0.01–0.10 / 1K tokens",
    "human_estimate": "约 ¥1 聊 100 句",
    "recommended_brand": "claude",
    "available_brands": ["claude", "openai", "gemini", "deepseek"]
  },
  ...
]
```

体感字符串 (`human_estimate`) 由 admin 配置（不算法估算）。

### 5.4 Price Tier 元数据

`model` 表加 `price_tier` 列（`economy` / `standard` / `premium` / `ultra`），admin 在模型管理后台标注。

Auto 模式路由时按所选 tier 过滤候选模型。

---

## 6. 前端改动要点

### 6.1 文件清单

| 文件 | 改动 |
|---|---|
| `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx` | 主表单：加 Simple/Advanced toggle，Simple 分支渲染目的卡片 |
| `web/default/src/features/keys/components/api-key-group-combobox.tsx` | **删除** `GroupRatioBadge`（紧急止血）；Advanced 模式下保留 desc，admin 才看 ratio |
| `web/default/src/features/keys/components/api-key-purpose-picker.tsx` | **新建**：6 张目的卡片选择器组件 |
| `web/default/src/features/keys/components/api-key-brand-filter.tsx` | **新建**：品牌偏好选择器（5 个 chip） |
| `web/default/src/features/keys/components/api-key-price-tier.tsx` | **新建**：Auto 模式的 4 档消费上限选择 |
| `web/default/src/features/keys/components/api-key-success-page.tsx` | **新建**：创建成功页，含 key/url/model + 客户端教程入口 |
| `web/default/src/features/keys/components/api-keys-empty-state.tsx` | **新建**：dashboard 空状态 CTA |
| `web/default/src/features/keys/api.ts` | 加 `simple_purpose` / `simple_brand` / `simple_price_tier` 字段；加 `getPurposeSummary()` |
| `web/default/src/features/keys/types.ts` | Schema 加新字段 |
| `web/default/src/routes/_authenticated/keys/` | 空状态接入 |
| `web/default/src/i18n/locales/en.json` + `zh.json` | 加全部新文案 key |
| `web/default/src/features/onboarding/`（**新建**） | 客户端教程页（Cherry Studio / Chatbox / LobeChat / Cursor / Claude Code / Code 各一页） |

### 6.2 状态保留

- `localStorage.setItem('apiKeyFormMode', 'simple' | 'advanced')`
- 默认 `simple`

---

## 7. 上线指标 & 验收

### 7.1 北极星指标

> **注册 → 首次成功 API 调用 的 P50 时间**
>
> 目标：从当前的"未测量"降到 **< 5 分钟**

### 7.2 二级指标

- Simple / Advanced 创建占比（预期 Simple ≥ 80%）
- Simple Mode 内目的分布（用于决定客户端教程页优先级）
- Key 创建后 24h 内首次 API 调用率（预期 ≥ 50%）
- 客户端教程卡片点击率（预期 ≥ 30%）
- 用户首次充值前的成功调用次数（预期 ≥ 10）
- Auto 模式价格档位分布（预期标准档 ≥ 60%，顶配档 ≤ 5%）
- 空状态 CTA 点击转化率（预期 ≥ 70%）

### 7.3 功能验收 Checklist

- [ ] Simple Mode 默认打开
- [ ] 切换 Advanced 后再次打开抽屉保留选择
- [ ] Simple Mode 只显示 Name + 6 张目的卡 + 可选品牌过滤
- [ ] 选 "编程" 后，客户端用 `deeprouter` 作为 model 名调用，返回正常 coding 模型响应
- [ ] 选 "图像" + 品牌 "OpenAI" → 路由到 DALL-E-3
- [ ] 选 "全部 (Auto)" + 标准档 → 不会调用 Opus / o1 / Ultra 这类顶配模型
- [ ] 顶配档勾选时弹出二次确认 dialog
- [ ] 倍率 Badge 在非 admin 视角不出现
- [ ] 创建成功页 key 复制按钮工作
- [ ] 客户端教程页至少有 Cherry Studio + Code 两个完整示例
- [ ] 移动端（< 640px）目的卡片 2 列 × 3 行堆叠正常
- [ ] 空状态 CTA 在首次进入 `/keys` 时显示
- [ ] i18n：en / zh / fr / ru / ja / vi 全部翻译

---

## 8. 里程碑 & 排期建议

### Phase 0 — 止血（**今天 / 本周**，半天）

1. 删 `GroupRatioBadge` 的两处渲染（trigger + list item）
2. Group label 从 "Group" 改为 "通道 / Channel"
3. Group desc 改人话（运营在 admin 后台改）

**目的**：先把 `5x Ratio` 红 badge 暴露给客户的问题挡住。不动数据模型、不动后端。

### Phase 1 — Simple Mode MVP（**2 周**）

1. 新建 `api-key-purpose-picker.tsx`，6 张目的卡片硬编码
2. 新建 `api-key-brand-filter.tsx`（5 个 chip）
3. 新建 `api-key-price-tier.tsx`（Auto 4 档）
4. 后端 `simple_purpose` / `simple_brand` / `simple_price_tier` 字段 → 自动填 `model_limits` + 路由策略
5. Purpose × Brand alias 表后端实现 + seed 数据 + admin CRUD
6. `model.price_tier` 列 + 价格档位路由
7. 创建成功页（key + url + model + 文字版"How to use"，截图教程占位）
8. Simple / Advanced toggle，默认 Simple
9. Dashboard 空状态 CTA
10. 移动端适配 + i18n

**交付**：Simple 流可用，非技术用户能闭环。

### Phase 2 — 客户端教程页（**1 周**）

1. `/onboarding/cherry-studio`、`/onboarding/chatbox`、`/onboarding/lobechat`、`/onboarding/cursor`、`/onboarding/claude-code`、`/onboarding/code`
2. 每页：截图 + 字段对应（base url / api key / model 填什么）+ "Test Now" 按钮
3. 教程页按 purpose 自适应（聊天目的优先展示 Cherry Studio，编程目的优先 Cursor）
4. 创建成功页卡片链到这些页

**交付**：从注册到第一次调用全程有图文引导。

### Phase 3 — Advanced 整改 + Pricing API（**1 周**）

1. Model Limits 提到主区，附最终单价（不暴露 ratio）
2. 通道字段语义化（高速 / 标准 / 经济）
3. `GET /api/pricing/purpose-summary` 后端 + admin 配置 UI（包含体感字符串）
4. 目的卡片价格区间从 API 拉取（不再硬编码）

**交付**：Advanced 用户也能在不看倍率的前提下做精细配置。

### Phase 4 — 数据驱动迭代（持续）

1. 埋点：mode 切换、目的选择分布、价格档位分布、创建后 24h 调用率
2. A/B 测：注册引导步骤要不要加"你想怎么用？"前置选择
3. 教程页点击率 → 客户端优先级排序
4. 目的卡片 reorder（按使用率排序）

---

## 9. 风险 & 开放问题

| 风险 / 问题 | 说明 | 缓解 |
|---|---|---|
| Purpose × Brand alias 与现有 channel routing 冲突 | DeepRouter 已有 `auto` group 跨通道路由，alias 解析时序要协调 | 在 distribution middleware 之前 resolve；写 e2e 测试 |
| 模型升级时 alias 切换的"破坏性" | Claude 4.7 → 4.8 时，用户 prompt 可能行为微变 | Admin 后台明示当前 alias 映射，可选"锁定版本"；公告通知用户 |
| Simple 用户想升级到 Advanced 的迁移 | 编辑现有 simple key 切到 advanced 会怎样 | 切换不丢字段，已选 purpose 对应的 model_limits 直接展示在 advanced，用户可继续编辑 |
| `Auto` 标准档对部分用户不够用 | 长文本场景需要 Opus / o1 才能完成 | 创建成功页提示"如果遇到能力不足，可切换到高级/顶配档"，提供深链回 advanced |
| 目的卡片数量增长 | 未来加 Embedding / Rerank / Search 等新场景 | 设计稿支持 2 列 × N 行；超过 8 个考虑"主流 / 更多 ▼" |
| 用户填了非 `deeprouter` 的 model 名 | Simple key 绑定 purpose 后，用户客户端如果填 `gpt-4o` 怎么办 | 在 model_limits 允许集合内就放行，超出范围就 reject 并返回提示"该 key 限制为 coding 用途" |
| 体感字符串"¥1 聊 100 句"准确性 | 不同模型 token 消耗差异大 | admin 配置（不算法估算），按"典型 chat 场景 500 token in + 500 token out"基准；卡片底部小字 disclaimer "实际用量按模型计价" |

**已决（v0.2）**：

- ✅ 价格单位：per 1K tokens（不用 1M），币种跟随账户；卡片正面是体感字符串
- ✅ Auto 模式价格上限：4 档（经济/标准/高级/顶配），默认标准档，顶配需二次确认
- ✅ Dashboard 引导：空状态 CTA，不做 multi-step overlay
- ✅ 主轴：目的导向（非品牌导向），品牌作为可选过滤

**仍开放** — 需要 review 时拍板：

- [ ] Phase 1 是否要把 Purpose × Brand alias **全部 admin UI 化**，还是先用 YAML seed + 数据库手改？YAML seed 上线快但运营改不动
- [ ] 客户端别名用 `deeprouter` 还是 `dr` 还是 `auto`？(deeprouter 太长，dr 太短，auto 与现有 group 名冲突)
- [ ] 现有用户的 key 是否需要"迁移到 Simple"提示？还是只对新 key 默认 Simple？
- [ ] 视频 / 语音目的的体感字符串和价格区间，需要 platform 团队提供准确数字
- [ ] 当用户首次"全部 (Auto)" 选了顶配档触发二次确认，要不要在确认 dialog 加一个"我已了解高费用"checkbox（避免用户误点）

---

## 10. 参考

- `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx` — 当前表单实现
- `web/default/src/features/keys/components/api-key-group-combobox.tsx` — 倍率 Badge 来源
- `controller/group.go::GetUserGroups` — 后端 group + ratio 数据来源
- [`docs/PRD.md`](../PRD.md) §B2 用户控制台 — 上层 PRD
- [`docs/DESIGN.md`](../DESIGN.md) — 视觉规范（卡片配色参考 §3 Color Tokens）
- 类似产品参考：
  - OpenRouter `openrouter/auto` 路由
  - Cursor 模型选择卡片
  - Perplexity Quick/Pro 模式
  - Linear / Notion / Vercel 空状态引导
