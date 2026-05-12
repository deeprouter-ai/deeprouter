# DeepRouter — Local Development Guide

> 本文档不是 upstream NewAPI 的一部分（避免 rebase 冲突）。
> Airbotix-specific intent 见 `AIRBOTIX.md`，业务 PRD 见 `~/Documents/sites/jr-academy-ai/deeprouter-brand/DeepRouter-PRD.md`。

---

## TL;DR — 5 分钟跑通本地

```bash
git clone https://github.com/deeprouter-ai/deeprouter
cd deeprouter
docker compose up -d            # 启动 new-api + postgres + redis
open http://localhost:3000      # 访问 Admin UI
# 注册第一个账号 → 自动成为 root admin
```

完成上面 4 步，DeepRouter 已经是一个**可用的 OpenAI 兼容 gateway**（裸机版，无 Airbotix 改造）。下一步是加 4 个自有字段 + policy 中间件，详见 §5。

---

## 1. 先决条件

- macOS / Linux（Windows 用 WSL2）
- Docker Desktop ≥ 4.30，docker compose v2
- Go 1.22+（仅当要编译我们的自有代码时）
- (可选) `bun` 用于前端 hot-reload（见 §4）

---

## 2. 启动 / 停止 / 重置

```bash
# 启动（守护态）
docker compose up -d

# 看日志
docker compose logs -f new-api

# 停止（保留数据）
docker compose down

# 重置（清空 PG / Redis 所有数据，回到初始）
docker compose down -v
```

服务端口：
| 服务 | 端口 | 说明 |
|---|---|---|
| `new-api` (Go API + 内置 Admin UI) | `:3000` | 主要入口 |
| `postgres` | （仅 docker 网络内）| 默认不暴露 |
| `redis` | （仅 docker 网络内）| 同上 |

如果要直连 Postgres 调试：取消 `docker-compose.yml` 里 postgres 服务 `ports` 那段注释。

---

## 3. 首次安装：创建 root admin + 测试一个 LLM 请求

### 3.1 创建 root admin

第一次访问 `http://localhost:3000` 注册的用户**自动是 root admin**。

```
浏览器 → http://localhost:3000
  → 顶部 "Register" → 邮箱 + 密码
  → 注册完登录
```

### 3.2 配置上游 provider（Channel）

```
Admin UI → 渠道 (Channels) → 添加新渠道
  类型: OpenAI（或 Anthropic / DeepSeek / 豆包等）
  名称: openai-test
  Key: sk-xxxxx（你的真实 OpenAI key）
  模型: gpt-4o-mini, gpt-3.5-turbo
  → 保存
```

### 3.3 创建一个测试 API key (Token)

```
Admin UI → 令牌 (Tokens) → 添加新令牌
  名称: dev-test
  无限额度
  → 复制生成的 sk-xxxxx
```

### 3.4 测试 OpenAI 兼容接口

```bash
TOKEN=sk-xxxxxx   # 上一步复制的 token

curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hello in 5 words"}]
  }'
```

如果返回 OpenAI 标准 chat completion 响应，说明 **DeepRouter 裸机版本已经工作**。

---

## 4. 开发工作流（hot reload）

跑 dev compose 文件（用本地源码构建，而非 docker hub 镜像）：

```bash
docker compose -f docker-compose.dev.yml up -d
cd web && bun install && bun run dev
# 前端 dev 服务器 http://localhost:3001（API 自动代理到 :3000）
```

修改 Go 后端代码后重建：

```bash
docker compose -f docker-compose.dev.yml up -d --build new-api
```

---

## 5. Airbotix-specific 改造 — Week 3-4 工作目标

按 PRD §6.4-pre，我们需要给 NewAPI 的 `User` 表加 4 个字段。**改造目录约定**（避免污染 upstream）：

```
deeprouter/
├─ internal/policy/     ← 新建。kid-safe 注入、prompt/output 过滤
├─ internal/billing/    ← 新建。计费 webhook 调用 + 重试 + 死信队列
├─ internal/kids/       ← 新建。kids_mode 硬约束执行（strip metadata / ZDR / 模型白名单）
├─ model/user.go        ← 上游文件，加 4 个字段（migration 在 model/db.go）
└─ web/default/...      ← 上游 admin UI，加 4 个字段的编辑入口
```

要加的字段：

```go
// model/user.go 扩展
type User struct {
    // ... 上游已有字段
    KidsMode             bool   `gorm:"default:false"`
    PolicyProfile        string `gorm:"size:32;default:'passthrough'"`  // kid-safe | adult | passthrough
    BillingWebhookURL    string `gorm:"size:255"`
    CustomPricingID      string `gorm:"size:64"`
}
```

测试：在 admin UI 给一个 user 开 `kids_mode = true`，调用 OpenAI 端点时验证：
- ✅ 请求自动加 `store: false`
- ✅ 输出 NSFW 等过滤启用
- ✅ 请求 metadata 里没有 user_id

---

## 6. 验证 12 周里程碑

| Week | 验收 |
|---|---|
| W2 | 本文档 §3.4 的 curl 跑通；admin UI 可正常配置 channel |
| W4 | User 表新字段 + admin UI 入口 + e2e 测试通过 |
| **W6** | `/v1/chat/completions` 跨协议转换 + 流式 SSE 全部通过（**unblocks Team B**） |
| W8 | 豆包 / DeepSeek / Qwen 接入 + 多 channel pool burst 压测通过 |
| W10 | policy 中间件 + billing webhook + Airbotix `/internal/deeprouter/billing` receiver 联调通过 |
| W12 | JR Academy 灰度 ≥ 1M token/day 通过 |

---

## 7. 跟上游 NewAPI 同步

```bash
git fetch upstream
git log upstream/main..HEAD --oneline    # 看我们和上游的差异
git cherry-pick <commit>                  # 拣一个上游 bugfix
# 或合并整段：
git merge upstream/main                   # 走 PR review
```

每月做一次 cherry-pick / merge 是 hygienic minimum。

---

## 8. Troubleshooting

| 现象 | 原因 / 解决 |
|---|---|
| `:3000` 端口被占 | 改 `docker-compose.yml` 的 ports 映射，或停掉占端口的进程 |
| `pg_data` permission denied | `docker compose down -v && docker compose up -d` 重置 |
| Admin UI 注册后没看到 admin 菜单 | 当前账号不是第一个；用 `docker compose down -v` 清库重来 |
| 上游 commit cherry-pick 冲突 | 大部分集中在 `web/` 和 `controller/` —— 我们的 `internal/` 不会冲突 |
| Channel 添加成功但调用 404 | 看 Channel 的"模型"字段是否包含请求的 model name |

---

## 9. 相关文档

- `AIRBOTIX.md` — fork 意图 + 我们要做的改造 + tenant 设计
- `~/Documents/sites/jr-academy-ai/deeprouter-brand/DeepRouter-PRD.md` — 完整工程 PRD（§5 架构 / §6 策略 / §7 计费 / §8 12 周计划）
- `~/Documents/sites/kidsinai/planning/PROJECT.md` — 跨 org 主计划
- 上游 `README.md` — NewAPI 原始文档（中英多语）
