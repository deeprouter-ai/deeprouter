# DeepRouter 中文说明

DeepRouter 是基于 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 的工程化 fork。上游 `new-api` 提供了成熟的 OpenAI 兼容网关、渠道管理、额度、日志、Admin UI 和多 provider adapter。DeepRouter 在这个基础上增加了更适合真实产品落地的多租户策略层、儿童安全模式、智能模型路由和计费 webhook 架构。

主 README 使用英文，是为了方便海外招聘经理、工程师和潜在雇主快速理解这个项目的技术价值。中文说明保留在本文件，便于中文读者了解背景。

## 项目定位

DeepRouter 不是简单换名的 fork。它把 `new-api` 的通用 AI gateway 改造成一个更偏产品级的 LLM gateway：

- 对外保持 OpenAI-compatible `/v1` API。
- 对内支持多个模型供应商和多把 provider key。
- 通过 `deeprouter-auto` 虚拟模型接入 smart-router sidecar。
- 为不同租户应用不同 policy。
- 对儿童/教育场景提供硬约束安全模式。
- 为下游业务系统预留按请求计费 webhook。

## 我在 fork 上做的主要工作

| 模块 | 说明 |
|---|---|
| `internal/policy/` | 把 `kids_mode` 和 `policy_profile` 转换成统一的请求决策。 |
| `internal/kids/` | 实现模型白名单、metadata 脱敏、OpenAI ZDR、child-safe system prompt。 |
| `relay/airbotix_policy.go` | 在 relay 层、provider 转换前应用安全策略。 |
| `middleware/smart_router.go` | 支持 `deeprouter-auto` 虚拟模型，调用 smart-router sidecar 后改写成具体模型。 |
| `internal/smart_router_client/` | smart-router HTTP client，带超时、熔断和降级逻辑。 |
| `internal/billing/` | HMAC 签名计费 webhook dispatcher，支持 transient retry。 |
| `model/user.go` | 扩展用户表字段：`kids_mode`、`policy_profile`、`billing_webhook_url`、`custom_pricing_id`、`webhook_secret`。 |
| 文档 | 增加架构、部署、kids coverage matrix、开发流程和 fork 意图说明。 |

## Kids Mode

`kids_mode` 是给儿童和教育产品使用的硬约束模式。开启后，DeepRouter 会在请求发给上游模型之前执行：

1. 非白名单模型直接拒绝。
2. 删除 `user`、`kid_profile_id`、`family_id` 等身份相关 metadata。
3. 对 OpenAI / Azure OpenAI 请求强制 `store: false`。
4. 注入或替换 child-safe system prompt。

相关测试覆盖见 [docs/kids-coverage-matrix.md](./docs/kids-coverage-matrix.md)。

## 智能路由

DeepRouter 把路由拆成两层：

```text
deeprouter-auto
  -> smart-router 选择具体模型
  -> new-api channel cache 选择可用 provider key
  -> relay adapter 转换并调用上游
```

这样既能保留上游 `new-api` 的渠道调度能力，也能把模型选择逻辑放在独立 sidecar 中演进。

## Skill Marketplace（已完成 PRD 设计，待实现）

Skill Marketplace 是网关之上的下一层产品能力：一个**官方托管的 AI Skill 市场**。用户在 Marketplace 浏览、启用 Skill，并在 Playground 中调用。一个 "Skill" 由服务端托管的 `instruction_template` 加上它的权限、安全和执行配置组成。`instruction_template` 是平台 IP，不提供下载，也永不下发到客户端。

V1 的产品闭环已经完整定义：

```text
Super Admin 创建官方 Skill
  -> 发布到 Marketplace
  -> 用户浏览、查看详情、启用
  -> 用户在 Playground 选择一个已启用 Skill
  -> Relay 校验权限和安全后，在服务端注入 instruction_template
  -> 归因 usage / billing / analytics / audit
  -> Operations 监控采用率、被拦截调用、收入和安全
```

设计上的几个核心约束：

- **服务端 Prompt DRM**：`instruction_template` 只存在 `skill_versions` 表，只有 Super Admin 和 Relay 执行链路可读；不进入任何客户端 API、日志、错误、analytics 事件、计费记录或导出。模板变更审计只记录 `sha256`，不记录 prompt 原文。
- **复用两层路由**：Skill 的 `model_whitelist` 存平台模型别名 / 路由组（如 `smart-tier`、`kids-safe-tier`），而不是写死的 provider 版本号。smart-router 在路由时解析别名，provider 弃用模型时只需改一处全局别名映射，不动任何 Skill 记录。
- **使用时鉴权**：启用 ≠ 永久授权；执行时重新校验计划、订阅、quota、生命周期和 Kids 状态，并映射到稳定错误码（`SKILL_PLAN_REQUIRED`、`SKILL_QUOTA_EXCEEDED`、`SKILL_KIDS_MODE_BLOCKED` 等）。
- **V1 无状态单轮**：每次 Playground 提交都是独立请求，不向 provider 转发历史对话，因此单次成本固定可预测。
- **Kids 安全为硬约束路径**：`is_kids_session` 由服务端判定（忽略客户端字段），非 Kids-Safe Skill 在注入前就被拦截，Kids 发布需 Safety Reviewer 审批。
- **计费 append-only + analytics 默认隐私**：计费台账一旦 charged 即不可变（退款用补偿行，由 DB trigger 强制），analytics 不存 prompt、原始用户输入和 Kids 敏感内容。

完整 V1 规格已经 Sprint-ready：模块化 PRD（功能需求、UX、数据模型与 API 合约、analytics 与运营、安全与 NFR，以及 M00–M15 共 16 个模块的工作拆分）放在 [docs/skill-marketplace/](./docs/skill-marketplace/)。目前还没有写任何后端或前端代码——这是下一阶段实现的设计 source of truth。

## 本地运行

```bash
git clone https://github.com/deeprouter-ai/deeprouter.git
cd deeprouter
docker compose up -d
```

打开 `http://localhost:3000`，注册第一个账号作为 root admin，然后在 Admin UI 中添加 provider channel 和 token。

测试请求：

```bash
TOKEN=sk-your-deeprouter-token

curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Say hello in five words."}
    ]
  }'
```

## 适合给海外雇主看的重点

英文 README 更适合作为 GitHub 首页，因为它能直接说明：

- 这个 fork 和 upstream 的关系是什么。
- 你具体新增了哪些后端模块。
- 这些模块解决了什么真实产品问题。
- 哪些功能已经实现，哪些还在计划中。
- 项目架构是否清晰、可测试、可维护。

这比中英文混排更专业，也更容易让海外 reviewer 在几十秒内抓住技术亮点。

## 许可证

DeepRouter 继承上游 `QuantumNous/new-api` 的 AGPL-3.0 许可证。
