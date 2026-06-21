# Changelog

DeepRouter gateway 变更记录。规则见 `AGENTS.md` Rule 10。

## 2026-06-20

- 新增 CI 门禁 `Unit Tests`（`.github/workflows/unit-test.yml`）：在**每个 PR**上运行（无 path 过滤，可设为 required status check），跑 gofmt + go vet + Airbotix 自有单测层 + 上游邻接的聚焦 `-run` 用例（与 `airbotix-internal.yml` 同口径，避免全量 `go test ./...` 拉起需要 MySQL/Redis 的上游用例）
- 新增 Unit Tests 规则到伞层公用 rule book（`../rules/unit-tests.md`，由伞层 `CLAUDE.md` 用 `@rules/` 导入）：后端行为改动必须带单测、`Unit Tests` CI 必须绿，禁止 `t.Skip`/删测刷绿。**不动 `AGENTS.md`（Codex 的 rule book）**
- 新增 `AGENTS.md` Rule 10（每次改动记 CHANGELOG）+ Rule 11（每个任务开工前先写/更新 `docs/tasks/*-prd.md`，带 spec→ship status）
- 新增 `CHANGELOG.md`：建立变更记录文件
- 新增站内 Docs/集成文档区（`web/default/src/features/docs/` + 路由 `/docs`、`/docs/$slug`）：渲染 `public/docs/integrations/*.md` 的 23 篇工具接入指南（Claude Code、Cursor、Cherry Studio、SDK 等），分类侧边栏 + 索引网格 + 运行时 fetch markdown。首页导航恢复 Docs 入口（`use-top-nav-links.ts`，受 `HeaderNavModules.docs` 控制）。新文件版权头用 `Copyright (C) 2026 DeepRouter`（非上游 QuantumNous——原创文件不挂上游版权；copyright 脚本按第三方版权跳过保留）
- Resources 文档区加固：路由 `/docs` → `/resources`（与导航标签一致；`routes/resources/`、导航 href、站内 `<Link>`、`doc-markdown.tsx` 内链解析全部改齐；markdown 静态资源仍在 `/docs/integrations`）；DESIGN.md §0–5 合规（侧栏 active 改 `bg-info/10 text-info` 双模式保 AI-blue、卡片 `rounded-xl bg-card`、`prose-pre:bg-card`、抬升 h3）；补 9 张 GUIDE.md 品牌占位图（修复运行时断图）；GUIDE.md 图片嵌入规范化 + 移除指向 `images/README.md` 的外链页脚；删除未被渲染的 `public/docs/integrations/README.md`
- 订正 `CLAUDE.md` §0 的定位描述：支付是**多币种（USD/AUD via Airwallex/CNY 微信支付宝），价格以美金计价（USD-denominated）**——不再误述为"只收/只按人民币"；并明确产品核心是**手把手教小白配置好、用起来 + 讲清每个模型用来干嘛（写作/代码/翻译/图像/语音）**
