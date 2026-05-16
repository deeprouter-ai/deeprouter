/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { Tutorial, TutorialContent, TutorialVars } from './registry'

export const CURSOR: Tutorial = {
  slug: 'cursor',
  label: 'Cursor',
  descriptionKey: 'AI-powered code editor — use DeepRouter for completions and chat.',
  icon: 'cursor',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Connect DeepRouter to Cursor

[Cursor](https://cursor.sh) supports custom OpenAI-compatible backends.

### 1. Open Cursor → Settings (⌘,)

### 2. Models tab
- Scroll to **OpenAI API Key**
- Paste your DeepRouter key
- Click **Override OpenAI Base URL** → enter \`${vars.baseUrl}\`
- Click **Verify**

### 3. Add your model name
- In the **Model Names** field, add: \`${vars.modelName}\` (and any others you want, e.g. \`gpt-4o\`, \`claude-sonnet-4-7\`)
- Click **Add Model**

### 4. Use it
\`Cmd+L\` or \`Cmd+K\` → DeepRouter routes the call.

### Notes
- Cursor's auto-complete (Cmd+→) uses its own internal model — **not** DeepRouter. Only Chat (⌘L) and Edit (⌘K) route through your custom backend.
- For agent mode and tab-completion, use the DeepRouter key inside Cursor's "Composer" agent settings instead.
`,
    zh: `## 在 Cursor 里接入 DeepRouter

[Cursor](https://cursor.sh) 支持自定义 OpenAI 兼容后端。

### 1. 打开 Cursor → 设置（⌘,）

### 2. Models 标签页
- 滚到 **OpenAI API Key**
- 粘贴 DeepRouter 创建的 Key
- 点 **Override OpenAI Base URL** → 填 \`${vars.baseUrl}\`
- 点 **Verify**

### 3. 添加模型名
- 在 **Model Names** 字段里加 \`${vars.modelName}\`（也可以多加几个 \`gpt-4o\`、\`claude-sonnet-4-7\`）
- 点 **Add Model**

### 4. 使用
\`Cmd+L\` 或 \`Cmd+K\` → DeepRouter 路由调用。

### 注意
- Cursor 的代码自动补全（Cmd+→）用自家内部模型 —— **不走** DeepRouter。只有 Chat（⌘L）和 Edit（⌘K）走自定义后端。
- 想 agent 模式 / tab 补全也走 DeepRouter？要在 Cursor 的 "Composer" agent 设置里再配一遍。
`,
  }),
}
