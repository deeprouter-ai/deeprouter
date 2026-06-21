# Changelog

DeepRouter gateway 变更记录。规则见 `AGENTS.md` Rule 10。

## 2026-06-20

- 重做 `/welcome` 为「目标导向」单屏：H1 从 persona 提问改为结果「你的账号已经开好了」，展示免费额度 + 密钥（可复制 / 无 handoff 时退化为去密钥页），给出「3 步用起来」+ 主 CTA「确认能用」(→ `/keys/test` 自检) 与次 CTA「充值」(→ `/wallet`)，persona 降级为底部可选项不再阻塞。所有 CTA 离开前都先保存 persona，避免被 `PersonaPickerHost` 弹回 /welcome 死循环。casual 文案零开发者术语 + 中英 i18n 落齐（`features/welcome/index.tsx`、`routes/welcome.tsx`、`i18n/locales/{en,zh}.json`）。PRD：`docs/tasks/welcome-goal-first-prd.md`
- 新增 `AGENTS.md` Rule 10（每次改动记 CHANGELOG）+ Rule 11（每个任务开工前先写/更新 `docs/tasks/*-prd.md`，带 spec→ship status）
- 新增 `CHANGELOG.md`：建立变更记录文件
- 新增站内 Docs/集成文档区（`web/default/src/features/docs/` + 路由 `/docs`、`/docs/$slug`）：渲染 `public/docs/integrations/*.md` 的 23 篇工具接入指南（Claude Code、Cursor、Cherry Studio、SDK 等），分类侧边栏 + 索引网格 + 运行时 fetch markdown。首页导航恢复 Docs 入口（`use-top-nav-links.ts`，受 `HeaderNavModules.docs` 控制）。新文件版权头用 `Copyright (C) 2026 DeepRouter`（非上游 QuantumNous——原创文件不挂上游版权；copyright 脚本按第三方版权跳过保留）
- 订正 `CLAUDE.md` §0 的定位描述：支付是**多币种（USD/AUD via Airwallex/CNY 微信支付宝），价格以美金计价（USD-denominated）**——不再误述为"只收/只按人民币"；并明确产品核心是**手把手教小白配置好、用起来 + 讲清每个模型用来干嘛（写作/代码/翻译/图像/语音）**
