# Changelog

DeepRouter gateway 变更记录。规则见 `AGENTS.md` Rule 10。

## 2026-06-21

- fix(DR-64): `resolve()` 現在在 DB 取回 Skill 後驗證 `skill.Status`，draft / archived / deprecated 的 Skill 回傳 `SKILL_NOT_PUBLISHED` (HTTP 403)，阻止非 published Skill 被執行 (`internal/skill/relay/resolver.go`)
- fix(DR-64): `resolve()` 現在驗證 `active_version_id != nil`，防止 DR-88 下游 nil deref；無 active version 的 Skill 同樣回傳 `SKILL_NOT_PUBLISHED` (`internal/skill/relay/resolver.go`)
- test(DR-64): `defaultSkill()` helper 補上 `ActiveVersionID`；新增 4 個測試覆蓋 draft/archived/deprecated/nil-active-version 路徑 (`internal/skill/relay/resolver_test.go`, `relay/compatible_handler_skill_test.go`)

## 2026-06-20

- 新增 DR-64 relay entry：接受 `deeprouter.skill_id`，從 auth context 解析用戶身份，返回 `SkillRelayContext` 供下游 DR-67/DR-88 使用 (`internal/skill/relay/`, `relay/compatible_handler.go`, `dto/deeprouter_extension.go`)
- 新增 `enums.EntryPointSkillPackage = "skill_package"` 供 relay 和 download 路徑共用 (`internal/skill/enums/`)
- 新增 `SkillRelayContext.EntryPoint` 字段，relay 入口設置 entry_point 供 DR-88 analytics 使用 (`internal/skill/relay/context.go`)
- 新增 `AGENTS.md` Rule 10（每次改动记 CHANGELOG）+ Rule 11（每个任务开工前先写/更新 `docs/tasks/*-prd.md`，带 spec→ship status）
- 新增 `CHANGELOG.md`：建立变更记录文件
- 新增站内 Docs/集成文档区（`web/default/src/features/docs/` + 路由 `/docs`、`/docs/$slug`）：渲染 `public/docs/integrations/*.md` 的 23 篇工具接入指南（Claude Code、Cursor、Cherry Studio、SDK 等），分类侧边栏 + 索引网格 + 运行时 fetch markdown。首页导航恢复 Docs 入口（`use-top-nav-links.ts`，受 `HeaderNavModules.docs` 控制）。新文件版权头用 `Copyright (C) 2026 DeepRouter`（非上游 QuantumNous——原创文件不挂上游版权；copyright 脚本按第三方版权跳过保留）
- 订正 `CLAUDE.md` §0 的定位描述：支付是**多币种（USD/AUD via Airwallex/CNY 微信支付宝），价格以美金计价（USD-denominated）**——不再误述为"只收/只按人民币"；并明确产品核心是**手把手教小白配置好、用起来 + 讲清每个模型用来干嘛（写作/代码/翻译/图像/语音）**
