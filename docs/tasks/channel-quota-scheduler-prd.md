---
title: "PRD — Channel Quota-Aware Scheduling (L2 凭证池调度)"
date: 2026-09-26
status: spec
owner: "@yurunyu"
priority: high
due: ""
branch: "yurunyu/api-pool"
repo: "deeprouter"
---

# PRD — Channel Quota-Aware Scheduling（L2 凭证池调度）

> **范围**: 给 L2 渠道选择（`model/channel_cache.go:GetRandomSatisfiedChannel`）加一层
> **按真实剩余配额调度**的能力——数据源是上游响应头，不是推断。
> **不覆盖**: 消费级订阅凭证池（见 §3，那是独立的供给决策，不在本 PRD 范围）。
> **依赖**: 无新增外部依赖。触及 `model/channel_cache.go`、`relay/` 完成路径、
> `controller/channel.go`（只读展示）。
> **落地位置**: 全部在 `deeprouter/`（分支 `yurunyu/api-pool`）。**不涉及 `smart-router/`** ——
> L2 渠道路由按定义属于网关侧，且 smart-router 按设计不接触 API key
> （`smart-router/CLAUDE.md` §1），跨进程契约也不应为此加宽（`rules/solid.md` 接口隔离）。
> 部署上无新增产物：现有 `new-api` 二进制内部变更，Redis 已在栈内，走现有 CI。
> **前置阅读**: `../ARCHITECTURE.md`（relay 分层）、`../../CLAUDE.md`（两层路由模型）、
> `AGENTS.md` Rule 2（三库兼容）。

---

## 1. 问题

`GetRandomSatisfiedChannel(group, model, retry)` 目前是**完全无状态**的：

```
group2model2channels[group][model] → 一组 channel id
  → 按 priority 分层（降序）
  → 层内按 weight 加权随机
  → 重试第 N 次跳到第 N 档 priority
```

它不知道任何一个 channel 当前**还剩多少配额**。四个具体后果：

**① 撞墙式发现限流。** 一个已经打满 RPM/TPM 的 channel 仍会被等概率选中 →
429 → 进重试 → 白付一次往返延迟。上游明明在响应头里告诉过我们它快满了，
我们没有读。

**② 重试会不必要地跳档涨价。** 重试第 N 次跳到第 N 档 priority，
而 priority 分层通常就是成本分层——同层还有余量的 channel 没被尝试，
就先跳到了更贵的一档。

**③ 突发流量无法整形。** 网关流量天然突发。随机选择意味着一次突发会均匀
打在所有 channel 上，而不是优先打在余量最多的那个上。

**④ 这会抵消 smart-router 的成本优化。** 这条最关键：
L1（smart-router）挑出"最便宜够用的模型"，L2 如果在那个模型上 429 了
并跳档到更贵的渠道，**L1 省下来的钱在 L2 被吐回去**。
配额感知的 L2 是让 smart-router 的节省真正落地的前提。

另外 `DeepRouter-BP.md` §9 已经把"上游模型方限制（OpenAI 封 IP/账户）"
列为中高风险，对策写的是"多上游冗余 + 多账号池，降低单点依赖"——
**本 PRD 就是那条对策缺失的工程实现**。

---

## 2. 方案

一层 per-channel 配额状态，写入来自上游响应头，读取发生在选择时。
**核心设计原则：最小侵入 + 严格向后兼容。**

### 2.1 数据采集（写路径）

在 relay 完成路径（与计费同一位置，`PostTextConsumeQuota` 附近）解析上游响应头。

两家主流格式：

| 上游 | 请求数 | Token 数 | 429 |
|---|---|---|---|
| Anthropic | `anthropic-ratelimit-requests-remaining` / `-limit` / `-reset` | `anthropic-ratelimit-tokens-remaining` / `-limit` / `-reset` | `retry-after` |
| OpenAI 及兼容 | `x-ratelimit-remaining-requests` / `-limit-` / `-reset-` | `x-ratelimit-remaining-tokens` / `-limit-` / `-reset-` | `retry-after` |

> ⚠️ 上表头名需在实现时**对真实上游响应核实一遍**（`CLAUDE.md` §0 Rule 3：
> 每个用到的值都要对活上游验证过）。

**不改 37 个适配器。** 走 Go 惯用的可选接口断言：

```go
// 默认提取器覆盖 Anthropic 式 + OpenAI 式两种头；
// 形状不同的 provider 自行实现这个接口即可，其余零改动。
type RateLimitReporter interface {
    ExtractRateLimit(http.Header) *RateLimitSnapshot
}
```

`RateLimitSnapshot` 字段：`RequestsRemaining`、`RequestsLimit`、`TokensRemaining`、
`TokensLimit`、`ResetAt`、`ObservedAt`、`CooldownUntil`（429 时由 `retry-after` 推出）。

### 2.2 状态存储

- **权威存储 Redis**：`channel_rl:{channel_id}` → hash，TTL = `ResetAt` + 余量
- **热路径读内存**：与现有 `channelSyncLock` / `channelsIDM` 同一层的内存副本

**为什么要 Redis 而不是纯内存**：今天是单机，但水平扩容是既定路径
（`smart-router/docs/PRD.md` §5.2）。配额状态若不共享，每个实例只有局部视图，
会集体超发。

**已知取舍**：内存读 + Redis 写意味着跨实例有秒级滞后。可接受——
限流本身是软约束，轻微超发由 429 路径兜住。**此取舍需写进实现注释。**

### 2.3 调度（读路径）

**不新增选择算法，只调制权重。** 现有"priority 分层 + 层内加权随机"结构完全保留，
在随机抽取前对 weight 做一次乘性调整：

| 状态 | 判定 | weight 调整 |
|---|---|---|
| `healthy` | 剩余比例 > 阈值（默认 20%） | ×1.0（不变） |
| `throttled` | 剩余比例 ≤ 阈值 | 按剩余比例线性衰减 |
| `exhausted` | 剩余 ≈ 0，或处于 429 冷却期内 | ×0（跳过） |
| `unknown` | 无任何配额数据 | **×1.0（等同现状）** |

选这个设计是因为**对 fork 的上游冲突面最小**——不改选择算法骨架，
只在既有 weight 上乘一个系数（`AIRBOTIX.md` 的 cherry-pick 成本考虑）。

### 2.4 降级规则（硬要求）

`../../CLAUDE.md` 把优雅降级列为硬约束，本功能必须遵守：

1. 同一 tier 全部 `exhausted` → **降到下一 tier**，不返回 nil
2. **所有** tier 全部 `exhausted` → 仍返回一个 channel（选冷却期最早到期的那个）。
   **宁可试一次失败，不可无尝试地硬失败**
3. 状态陈旧（`ObservedAt` 早于 N 秒且无 `ResetAt` 信息）→ **按乐观处理**，
   视为 `healthy`。绝不允许陈旧的悲观状态把一个 channel 永久排除在外
4. `now > ResetAt` → 视为窗口已滚动、配额已恢复

### 2.5 可观测性

- Admin 渠道列表页增加每个 channel 的当前配额状态（只读）
- 计数器：因 `exhausted` 被跳过的选择次数、跳档次数
- **不在热路径加日志**——每请求一条结构化日志已是既定预算上限

### 2.6 配置

| 开关 | 默认 | 作用 |
|---|---|---|
| `CHANNEL_QUOTA_SCHEDULING_ENABLED` | **false** | 总开关。默认关闭，灰度放开 |
| `CHANNEL_QUOTA_THROTTLE_THRESHOLD` | `0.2` | `throttled` 判定阈值 |
| `CHANNEL_QUOTA_STALE_SECONDS` | `60` | 状态陈旧判定窗口 |

**默认关闭是刻意的**：这动的是生产热路径，而生产上跑着 `airbotix-kids`
和 `jr-academy` 的真实流量，没有 staging 环境。

---

## 3. 不做什么

- ❌ **消费级订阅 / session token 凭证池。** 订阅**没有任何接口可查剩余额度**，
  调度只能做成"自己数 + 拿 429 当 ground truth 修正"的推断式——永远在估、
  靠撞墙校准、且每次撞墙都是一次用户可见失败。这是**供给来源决策**，
  不是调度器的技术选项，需 Lightman 拍板后另立 PRD
- ❌ **粘性会话（sticky session）。** 与本功能目标直接冲突（粘性要锁定账号，
  配额调度要迁移流量），也与 smart-router 的动态选型冲突。三者不能同时满足
- ❌ **预测式 / 学习型负载预估。** V0 只对真实响应头做反应式调度
- ❌ **选择时做跨 provider 成本优化。** 那是 L1（smart-router）的职责，
  不在 L2 重复实现
- ❌ **改动 priority 分层语义**
- ❌ **per-user 公平性 / 排队**

---

## 4. 验收

- [ ] **无 rate-limit 头的 channel，行为与现状逐位一致**（回归测试覆盖）
- [ ] `CHANNEL_QUOTA_SCHEDULING_ENABLED=false` 时，选择结果与现状等价
- [ ] 某 channel 剩余配额降至阈值以下后，其被选中频率可统计地下降
- [ ] channel 进入 429 冷却后，在 `CooldownUntil` 前不被选中；到期自动恢复
- [ ] 同一 tier 全部 exhausted → 降到下一 tier，不返回 nil
- [ ] 所有 tier 全部 exhausted → 仍返回一个 channel，不硬失败
- [ ] 状态陈旧时按乐观处理，不会导致 channel 被永久跳过
- [ ] **选择路径无新增同步网络调用**（Redis 写在响应后，不在选择路径上）
- [ ] Anthropic 式与 OpenAI 式两种响应头都能正确解析（对真实上游验证，非构造数据）
- [ ] Admin 能看到每个 channel 的当前配额状态
- [ ] 三库（SQLite / MySQL / PostgreSQL）均可运行（若引入 schema 变更）
- [ ] 单测覆盖（`rules/unit-tests.md`），含"全部 exhausted 时不返回 nil"这条

---

## 5. 风险 / 待议

**Q1 — 重试语义要不要改？**
现状：重试第 N 次跳到第 N 档 priority。
可选：重试时先试**同 tier 内还有余量**的 channel，再跳档。
后者更省钱（避免不必要涨价），但**改变了既有行为**，需要单独 flag 门控。
本 PRD 倾向保留现状、把它作为 follow-up，但需要拍板。

**Q2 — V0 要不要上 Redis？**
今天是单机，纯内存能省掉一层复杂度。但补 Redis 时会改到状态读写接口。
建议：V0 就走 Redis（接口一次成型），除非有明确的交付压力。

**Q3 — V0 覆盖哪些 provider 的头解析？**
建议：默认提取器覆盖 Anthropic 式 + OpenAI 式（能吃掉 37 家里的大多数），
其余留 `unknown` 状态（行为等同现状），按实际流量占比再逐个补。

**Q4 — 配额状态要不要落库？**
Redis-only：无 schema 变更，但丢历史，排查问题时没有回溯能力。
落 `channels` 表：可回溯，但引入迁移 + 三库兼容成本 + 高频写放大。
建议 Redis-only，历史数据用 ops 指标另行采集。

**Q5 — 与 smart-router 的配合边界？**
L2 若因配额把请求降到了一个**明显更贵**的 channel，L1 是否应该被告知、
从而在下一次路由时换个模型？这需要扩 `POST /route` 的响应或加反馈通道——
属于跨进程契约变更（`rules/process-boundary.md`），不在本 PRD 范围，
但值得记下来。
