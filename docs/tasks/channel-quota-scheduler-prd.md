---
title: "PRD — L2 凭证池配额感知调度（Channel Quota-Aware Scheduling）"
date: 2026-09-26
status: spec
owner: "@yurunyu"
priority: high
due: ""
branch: "yurunyu/api-pool"
repo: "deeprouter"
---

# PRD — L2 凭证池配额感知调度（Channel Quota-Aware Scheduling）

> **Version**: 📝 Draft v0.1 · 待评审
> **Author**: @yurunyu + Claude
> **Date**: 2026-09-26
> **Owner**: DeepRouter Platform
> **Branch**: `yurunyu/api-pool`（`deeprouter/`）
> **Parent**: [`docs/PRD.md`](../PRD.md)
> **Sibling**: `smart-router/docs/embedding-routing-prd.md` —— 那份是 L1 模型路由，本 PRD 是 L2 渠道路由；两者是同一条成本链上的上下游（见 §1.3）
> **范围**: 给 L2 渠道选择（`model/channel_cache.go:GetRandomSatisfiedChannel`）加**按真实剩余配额调度**的能力。数据源是上游响应头，不是推断。
> **License 边界**: 本功能**全部落在 `deeprouter/`（AGPL v3）**，不碰 `smart-router/`，不跨进程边界，不加宽既有 HTTP 契约。理由见 §2.3。

---

## 0. 版本变更

### v0.1（2026-09-26）

- 初稿：定义配额状态四态、权重调制式调度、四条降级规则、五个待决策项
- **刻意把范围收口到 API key 池**。消费级订阅凭证池不在本 PRD 内——它是**供给来源决策**而非调度器的技术选项，作为 §9 **D1** 留给业务侧拍板
- 部署形态确认：无新增产物，现有 `new-api` 二进制内部变更

---

## 1. 背景

### 1.1 现状

L2 渠道路由的全部逻辑在 `model/channel_cache.go:GetRandomSatisfiedChannel(group, model, retry)`：

```
group2model2channels[group][model]        → 能服务该 model 的一组 channel id
  → 收集 unique priority，降序排序        → 优先级分层
  → 层内按 weight 加权随机                → 选出一个 channel
  → 重试第 N 次 → 跳到第 N 档 priority     → 重试即降级
```

这个函数是**完全无状态的**：它不知道任何一个 channel 当前还剩多少配额。
每次选择只看静态配置（priority、weight、group、model list），不看运行时容量。

同时，上游**每个响应都在头里告诉了我们剩余额度**（`anthropic-ratelimit-*`、
`x-ratelimit-*`），而这些数据目前被直接丢弃。

### 1.2 为什么需要配额感知调度

无状态选择带来四个具体后果：

**① 撞墙式发现限流。** 已打满 RPM/TPM 的 channel 仍被等概率选中 → 429 →
进重试 → 白付一次上游往返的延迟。上游明明提前告知过，我们没读。

**② 重试会不必要地跳档涨价。** 重试第 N 次跳到第 N 档 priority，而 priority
分层通常就是成本分层。**同层还有余量的 channel 没被尝试，就先跳到了更贵的一档。**

**③ 突发流量无法整形。** 网关流量天然突发。随机选择意味着突发均匀打在所有
channel 上，而不是优先打在余量最多的那个上——于是多个 channel 同时接近上限。

**④ 这会抵消 smart-router 的成本优化。** 见下节。

另外 [`docs/DeepRouter-BP.md`](../DeepRouter-BP.md) §9 已把"上游模型方限制
（OpenAI 封 IP/账户）"列为中高风险，对策写的是"多上游冗余 + 多账号池，
降低单点依赖"。**本 PRD 是那条对策缺失的工程实现**——今天有"多账号"，
但没有"会调度的池子"。

### 1.3 与 smart-router（L1）的关系

这是本 PRD 最重要的一条论证，也是"为什么现在做"的答案。

按 [`../../CLAUDE.md`](../../../CLAUDE.md) 的两层路由模型：

| 层 | 组件 | 输入 | 输出 |
|---|---|---|---|
| L1 模型路由 | `smart-router` | prompt + tenant_id + constraints | **模型名** + fallback chain |
| L2 渠道路由 | `deeprouter` | 模型名 | **用某个 API key 发出去** |

L1 的全部价值是"挑出最便宜够用的模型"（`smart-router/docs/PRD.md` G3：
auto 模式降本 ≥50%）。但如果 L2 在那个便宜模型上 429 了、并跳档到更贵的渠道：

```
L1: 这个请求用 deepseek-chat 就够 → 省 90%
L2: deepseek 渠道满了 → 429 → 重试跳档 → 落到 gpt-4o 渠道
净效果: L1 省下的钱在 L2 全额吐回去，还多付了一次往返延迟
```

**配额感知的 L2 是让 smart-router 的节省真正落地的前提。**
两件事不是并行的两个优化，是同一条成本链上的上下游——L1 做得再好，
L2 漏水就没有意义。

### 1.4 关键技术洞察

#### 洞察 1：可观测性决定了调度能不能做对

这是决定 §2.2 范围收口的根本原因。

| | API key | 消费级订阅 |
|---|---|---|
| 剩余额度可查？ | ✅ 响应头 `x-ratelimit-remaining-*` / `anthropic-ratelimit-*-remaining` | ❌ **无任何接口** |
| 怎么知道用超了 | 提前从响应头看到 | **被拒之后才知道** |
| 限额是否公开稳定 | 按 tier 公布 | 会静默调整 |
| 调度形态 | **确定性**（读真实数据） | **推断式**（自己数 + 撞 429 校准） |

这不是"两种供给都能调度、只是难度不同"，**这是两个不同的工程问题**：
一个读数据，一个猜数据并靠用户可见的失败来校准。可靠性差一个量级。

#### 洞察 2：最小侵入优于最优算法

这是一个 fork。[`../AIRBOTIX.md`](../AIRBOTIX.md) 的核心约束是**让 upstream
cherry-pick 面尽量小**。

一个"理论更优的调度算法"如果重写了 `GetRandomSatisfiedChannel` 的骨架，
那么每次从 `QuantumNous/new-api` rebase 都要重新解一遍冲突——而这个函数
是上游的核心热路径，改动概率不低。

**所以设计选择是：完全保留 priority 分层 + 层内加权随机，只在抽签前对
weight 乘一个系数。** 牺牲一点理论最优，换长期可维护性。

#### 洞察 3：陈旧状态必须按乐观处理，否则会自我放大成故障

配额状态来自"上次请求该 channel 时的响应头"。**如果一个 channel 很久没被
选中，它的状态天然是陈旧的。**

假设我们把陈旧状态当作"可能已耗尽"从而降权，就会出现负反馈死锁：

```
状态陈旧 → 降权 → 更不容易被选中 → 更久没有新数据 → 状态更陈旧 → 权重更低 → …
```

一个健康的 channel 会被永久排除在池子外，而且**没有任何错误日志**。
所以规则必须是：**陈旧 = 乐观 = 视为 healthy**（§4.4 规则 3）。

#### 洞察 4：粘性会话、配额调度、L1 动态选型，三者不可兼得

- **粘性会话**要求：同一会话锁定同一凭证
- **配额调度**要求：把流量迁移到还有余量的凭证上
- **L1 动态选型**要求：每个请求按 prompt 重新挑模型（可能换 provider）

三个目标两两冲突，不存在同时满足的调度策略。本 PRD 选择保住**配额调度 +
L1 动态选型**，放弃粘性（§2.2）。这也是为什么 §9 **D1** 的订阅方案不是
"加个功能"而是"换一套互斥的架构"。

---

## 2. 产品范围与非目标

### 2.1 V0 范围

- 从上游响应头采集 per-channel 配额快照（Anthropic 式 + OpenAI 式两种头名）
- 配额状态四态：`healthy` / `throttled` / `exhausted` / `unknown`
- 权重调制式调度（不改选择算法骨架）
- 429 冷却期（由 `retry-after` 推出）
- 四条降级规则（§4.4），保证不因配额逻辑产生硬失败
- Admin 渠道页只读展示当前配额状态
- feature flag 总开关，**默认关闭**

### 2.2 V0 非目标

- ❌ **消费级订阅 / session token 凭证池** —— 见洞察 1 与 §9 **D1**。
  这是供给来源决策，需业务侧拍板后另立 PRD
- ❌ **粘性会话（sticky session）** —— 见洞察 4，与本功能目标及 L1 动态选型互斥
- ❌ **预测式 / 学习型负载预估** —— V0 只对真实响应头做反应式调度
- ❌ **选择时做跨 provider 成本优化** —— 那是 L1 的职责，不在 L2 重复实现
- ❌ **改动 priority 分层语义** —— 见洞察 2
- ❌ **per-user 公平性 / 请求排队**
- ❌ **配额状态历史落库与趋势分析** —— 见 §9 **D4**

### 2.3 与上游 NewAPI 的兼容

上游 `QuantumNous/new-api` 无配额感知调度，本功能为 DeepRouter 自研。
按 [`../../rules/fork-and-branding.md`](../../../rules/fork-and-branding.md)
的隔离约定：

```
internal/channelquota/        ← 新建。快照类型、状态判定、权重调制、存储
relay/                        ← 只在完成路径加一次头解析调用（与计费同位置）
model/channel_cache.go        ← 唯一的上游文件改动：抽签前乘一次系数
controller/channel.go         ← 只读展示，不改写入逻辑
```

**上游文件改动只有 `channel_cache.go` 一处，且是"乘一个系数"级别的插入**，
rebase 冲突面最小化（洞察 2）。

**为什么不放 `smart-router/`**：
1. `smart-router/CLAUDE.md` §1 明确写 *"Smart-router never touches API keys."*
   而渠道池就是 API key 的池子
2. License 边界：AGPL v3 与 Apache 2.0 不能共享 Go module graph
3. [`../../rules/solid.md`](../../../rules/solid.md) 接口隔离：跨进程契约就两个端点，
   *"Do not widen these contracts to leak internal state across the boundary."*
   channel 级配额恰好是内部状态
4. `smart-router` 的 <5ms p99 预算明确禁止 *"new network calls in the `/route` hot path"*
5. 更根本的：`GET /internal/router-catalog` 返回的是 **model**，不是 channel。
   smart-router 的世界观里不存在"凭证"这个概念——**这层抽象是承重的**

---

## 3. 核心概念

| 术语 | 定义 |
|---|---|
| **Channel** | 一份上游凭证 + 其配置（key、base URL、model list、priority、weight、group）。DeepRouter 里"池子里的一个成员"就是一个 channel |
| **L2 渠道路由** | 模型名 → 具体 channel 的选择过程。本 PRD 的作用域 |
| **RateLimitSnapshot** | 一次响应头解析的产物：`RequestsRemaining` / `RequestsLimit` / `TokensRemaining` / `TokensLimit` / `ResetAt` / `ObservedAt` / `CooldownUntil` |
| **剩余比例** | `Remaining / Limit`，请求数与 token 数分别计算，取**更紧的那个**作为判定依据 |
| **`healthy`** | 剩余比例 > 阈值（默认 0.2）。weight ×1.0 |
| **`throttled`** | 剩余比例 ≤ 阈值。weight 按剩余比例线性衰减 |
| **`exhausted`** | 剩余 ≈ 0，或处于 429 冷却期内。weight ×0（跳过） |
| **`unknown`** | 无任何配额数据（上游不返回头 / 从未被请求过）。**weight ×1.0，行为等同现状** |
| **冷却期（`CooldownUntil`）** | 收到 429 后由 `retry-after` 推出的禁选截止时刻 |
| **陈旧（stale）** | `ObservedAt` 早于 `CHANNEL_QUOTA_STALE_SECONDS` 且无有效 `ResetAt`。**按乐观处理**（洞察 3） |
| **窗口滚动** | `now > ResetAt`，视为配额已恢复 |
| **权重调制** | 不替换选择算法，只在既有 weight 上乘一个 `[0, 1]` 系数（洞察 2） |

---

## 4. 技术方案

### 4.1 数据采集（写路径）

在 relay 完成路径解析上游响应头——**与计费同一位置**
（`PostTextConsumeQuota` 附近，`service/airbotix_billing.go` 的同级钩子）。

两家主流头名：

| 上游 | 请求数 | Token 数 | 429 |
|---|---|---|---|
| Anthropic | `anthropic-ratelimit-requests-remaining` / `-limit` / `-reset` | `anthropic-ratelimit-tokens-remaining` / `-limit` / `-reset` | `retry-after` |
| OpenAI 及兼容 | `x-ratelimit-remaining-requests` / `x-ratelimit-limit-requests` / `x-ratelimit-reset-requests` | `x-ratelimit-remaining-tokens` / `x-ratelimit-limit-tokens` / `x-ratelimit-reset-tokens` | `retry-after` |

> ⚠️ **上表头名必须在实现时对真实上游响应核实一遍**，不得只凭文档。
> 这是 [`../../CLAUDE.md`](../../CLAUDE.md) §0 Rule 3 的硬要求：
> 每个用到的值都要对活上游验证过。

**不改 37 个适配器。** 走 Go 惯用的可选接口断言：

```go
// 默认提取器覆盖 Anthropic 式 + OpenAI 式两种头名（吃掉 37 家里的大多数）；
// 形状不同的 provider 自行实现该接口，其余零改动。
type RateLimitReporter interface {
    ExtractRateLimit(http.Header) *RateLimitSnapshot
}
```

### 4.2 状态存储

- **权威存储 Redis**：`channel_rl:{channel_id}` → hash，TTL = `ResetAt` + 余量
- **热路径读内存**：与现有 `channelSyncLock` / `channelsIDM` 同层的内存副本

**为什么要 Redis 而不是纯内存**：今天是单机，但水平扩容是既定路径。
配额状态若不共享，每个实例只有局部视图，会**集体超发**。

**已知取舍（必须写进实现注释）**：内存读 + Redis 写意味着跨实例有秒级滞后。
可接受——限流本身是软约束，轻微超发由 429 路径兜住。

### 4.3 调度（读路径）

**不新增选择算法，只调制权重。** 现有骨架完全保留，在随机抽取前对每个
候选 channel 的 weight 乘一次系数（状态→系数映射见 §3 术语表）。

### 4.4 降级规则（硬要求）

[`../../CLAUDE.md`](../../../CLAUDE.md) 把优雅降级列为硬约束，本功能必须遵守：

1. **同一 tier 全部 `exhausted`** → 降到下一 tier，**不返回 nil**
2. **所有 tier 全部 `exhausted`** → 仍返回一个 channel（选冷却期最早到期的）。
   **宁可试一次失败，不可无尝试地硬失败**
3. **状态陈旧** → 按乐观处理，视为 `healthy`。**绝不允许陈旧的悲观状态把
   channel 永久排除**（洞察 3）
4. **`now > ResetAt`** → 视为窗口已滚动、配额已恢复

### 4.5 配置开关

| 开关 | 默认 | 作用 |
|---|---|---|
| `CHANNEL_QUOTA_SCHEDULING_ENABLED` | **false** | 总开关。默认关闭，灰度放开 |
| `CHANNEL_QUOTA_THROTTLE_THRESHOLD` | `0.2` | `throttled` 判定阈值 |
| `CHANNEL_QUOTA_STALE_SECONDS` | `60` | 陈旧判定窗口 |

**默认关闭是刻意的**：这动的是生产热路径，而生产上跑着 `airbotix-kids`
与 `jr-academy` 的真实流量，且**没有 staging 环境**
（[`../../rules/branching.md`](../../../rules/branching.md) §0）。

---

## 5. 数据模型

**V0 不引入 schema 变更**（见 §9 **D4**）。配额状态只存 Redis：

```
key:   channel_rl:{channel_id}
type:  hash
fields:
  requests_remaining   int
  requests_limit       int
  tokens_remaining     int
  tokens_limit         int
  reset_at             unix ts
  observed_at          unix ts
  cooldown_until       unix ts   # 429 时由 retry-after 推出，否则 0
TTL:   max(reset_at, cooldown_until) + 余量
```

因此**三库兼容（`AGENTS.md` Rule 2）不受影响**——没有迁移。
若 D4 决定落库，需另行评估三库 + 写放大成本。

---

## 6. 可观测性与管理后台

- **Admin 渠道列表页**：每个 channel 展示当前配额状态（只读）——
  状态四态、剩余比例、`reset_at`、是否在冷却期
- **计数器**：因 `exhausted` 被跳过的选择次数、跳档次数、陈旧态命中次数
- **不在热路径加日志**——每请求一条结构化日志已是既定预算上限

---

## 7. 实现拆解（参考）

```
internal/channelquota/
├── snapshot.go       — RateLimitSnapshot 类型 + 四态判定
├── extract.go        — 默认头解析（Anthropic 式 + OpenAI 式）+ RateLimitReporter 接口
├── store.go          — Redis 读写 + 内存副本
├── weight.go         — 状态 → weight 系数映射
└── *_test.go         — 单测（rules/unit-tests.md）

model/channel_cache.go    — 抽签前调用 weight.Modulate()（唯一上游文件改动）
relay/                    — 完成路径调用 extract + store.Put（与计费同位置）
controller/channel.go     — 只读展示
```

---

## 8. 性能预算

L2 选择在**每个请求的热路径上**，因此设硬上限：

- **选择路径增量 < 0.1ms**：仅内存 map 读 + N 次浮点乘法
- **选择路径不得引入任何同步网络调用**（含 Redis 读）
- Redis 写发生在**响应之后**（与计费同路径），不占用请求延迟
- 内存副本刷新不得持有 `channelSyncLock` 的写锁超过现状

---

## 9. 待决策项（Open Decisions）

> 业务/架构侧拍板后再进入实现。每条带建议但不锁死。

### D1: 订阅凭证池要不要做？

**这是供给来源决策，不是调度器的技术选项。**

| 选项 | 说明 |
|---|---|
| A. 只用 API key（本 PRD 范围） | 配额可观测 → 确定性调度。✅ V0 建议 |
| B. 加入消费级订阅池 | 配额不可观测 → 只能推断式调度（洞察 1），且与粘性/L1 动态选型互斥（洞察 4） |

选 B 需要：另立 PRD、重新评估 §4.3 调度设计、放弃粘性或放弃配额调度其一，
并由业务侧确认条款与风险。**本 PRD 不预设结论，但技术上 A 与 B 不是"同一个功能的两种配置"。**

### D2: 重试语义要不要改？

- 现状：重试第 N 次跳到第 N 档 priority
- 可选：重试时**先试同 tier 内还有余量的** channel，再跳档

后者更省钱（避免不必要涨价，直接服务 §1.2 ②），但**改变了既有行为**，
需单独 flag 门控。建议 V0 保留现状、作为 follow-up。

### D3: V0 要不要上 Redis？

今天单机，纯内存能省一层复杂度；但补 Redis 时要改状态读写接口。
建议 **V0 直接走 Redis**（接口一次成型），除非有明确交付压力。

### D4: 配额状态要不要落库？

| 选项 | 利 | 弊 |
|---|---|---|
| A. Redis-only | 无 schema 变更、无三库成本 | 丢历史，排查无回溯 |
| B. 落 `channels` 表 | 可回溯 | 迁移 + 三库兼容 + 高频写放大 |

建议 **A**，历史数据用 ops 指标另行采集。

### D5: 要不要给 L1 加配额反馈通道？

若 L2 因配额把请求降到明显更贵的 channel，L1 是否应被告知、从而下次换模型？

这需要扩 `POST /route` 响应或新增反馈通道——**属于跨进程契约变更**，
按 [`../../rules/process-boundary.md`](../../../rules/process-boundary.md)
必须先摆出来讨论；按 `rules/adlc.md` §2 要拆成两张卡用 `depends_on` 关联，
**不能开跨仓库分支**。建议 V0 不做，记录备忘。

---

## 10. 风险与对策

| 风险 | 影响 | V0 对策 |
|---|---|---|
| 上游改头名 / 不返回配额头 | 调度退化 | `unknown` 态 weight ×1.0，**严格等同现状**；实现时对活上游核实头名 |
| 跨实例状态滞后 → 集体超发 | 轻微 429 上升 | 限流是软约束，429 路径兜住；已作为已知取舍写进注释 |
| 陈旧状态负反馈死锁 | 健康 channel 被永久排除，且无报错 | 陈旧按乐观处理（洞察 3 + §4.4 规则 3），并加陈旧命中计数器 |
| 全部 channel `exhausted` | 硬失败、网关不可用 | §4.4 规则 1/2：逐级降级，最终仍返回一个 channel |
| 热路径性能回归 | 全局延迟上升 | §8 硬预算：无同步网络调用 + 性能回归验收项 |
| upstream rebase 冲突 | 长期维护成本 | 只调制 weight 不改骨架；上游文件改动收敛到一处（洞察 2） |
| 生产无 staging，改坏直接影响真实租户 | `airbotix-kids` / `jr-academy` 受影响 | feature flag **默认关闭** + 灰度放开 |

---

## 11. 成功指标（V0 上线后 30 天）

- **上游 429 率下降**：开关开启前后对比，目标降幅 ≥ 50%
- **不必要跳档次数下降**：同 tier 有余量却跳档的次数 → 目标趋近 0（若 D2 采纳）
- **选择路径 p99 延迟无回归**：符合 §8 预算
- **因"全部 exhausted"导致的硬失败 = 0**（§4.4 规则 2 生效）
- **因陈旧状态永久跳过 channel 的事件 = 0**（洞察 3 生效）
- **`unknown` 态 channel 的选中分布与开关关闭时一致**（严格向后兼容）
- **smart-router auto 模式的实际降本幅度提升**（本 PRD 的最终目的，见 §1.3）

---

## 12. Out of scope（V1+ Roadmap 备忘）

- 消费级订阅凭证池（待 **D1**）
- L2 → L1 配额反馈通道（待 **D5**，跨进程契约变更）
- 预测式 / 学习型负载预估
- per-user 公平性与请求排队
- 跨 region 容量摊平（Bedrock / Vertex 多区域）
- 配额状态历史落库 + 趋势分析看板（待 **D4**）
- 按配额压力自动启用备用 channel（自动扩缩池容量）
- 承诺用量（committed-use）额度的单独建模——与按 tier 的 RPM/TPM 是不同约束形状
