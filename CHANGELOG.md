# Changelog

DeepRouter gateway 变更记录。规则见 `AGENTS.md` Rule 10。

## 2026-06-20

- 删除面向终端用户的 playground 入口：`/playground` 路由改为重定向到 `/dashboard`，移除侧边栏「Chat / Playground」导航组、dashboard 快捷操作与「Send a request」步骤、onboarding banner「Try Playground」与教程页「Try in Playground」按钮。playground 是上游 new-api 的开发者调试台，DeepRouter 是账号+钱包工具不做 chat，终端用户不该使用（feature 代码保留以便日后开发者模式复用）（`web/default/src/routes/_authenticated/playground`、`hooks/use-sidebar-data.ts`、`features/dashboard/.../overview-dashboard.tsx`、`onboarding-status-banner.tsx`、`features/onboarding/index.tsx`）
- 新增 `AGENTS.md` Rule 10（每次改动记 CHANGELOG）+ Rule 11（每个任务开工前先写/更新 `docs/tasks/*-prd.md`，带 spec→ship status）
- 新增 `CHANGELOG.md`：建立变更记录文件
