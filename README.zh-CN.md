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
