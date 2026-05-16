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

export const CHATBOX: Tutorial = {
  slug: 'chatbox',
  label: 'Chatbox',
  descriptionKey: 'Cross-platform chat client (web + desktop).',
  icon: 'chat',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Connect DeepRouter to Chatbox

[Chatbox](https://chatboxai.app) is an open-source AI chat client — works on web, desktop, mobile.

### 1. Open Chatbox
Use the web app at [web.chatboxai.app](https://web.chatboxai.app) or install the desktop app.

### 2. Add the AI provider
- Click **Settings** (top-right)
- **Model Provider** → choose **OpenAI API**

### 3. Fill these fields

| Field | Value |
|---|---|
| API Key | the key from DeepRouter (starts with \`sk-\`) |
| API Host | \`${vars.baseUrl}\` |
| Model | \`${vars.modelName}\` |

### 4. Save and chat.

### Troubleshooting
- **"401 Unauthorized"** — re-copy the API key. Avoid trailing spaces.
- **Model not responding** — try \`gpt-4o-mini\` or another model in the dropdown.
`,
    zh: `## 在 Chatbox 里接入 DeepRouter

[Chatbox](https://chatboxai.app) 是开源 AI 客户端 —— 网页 / 桌面 / 移动端都有。

### 1. 打开 Chatbox
用网页版 [web.chatboxai.app](https://web.chatboxai.app) 或装桌面端。

### 2. 添加 AI 服务商
- 点右上 **设置**
- **模型提供方** 选 **OpenAI API**

### 3. 填以下字段

| 字段 | 内容 |
|---|---|
| API 密钥 | DeepRouter 创建的 Key（\`sk-\` 开头） |
| API 地址 | \`${vars.baseUrl}\` |
| 模型 | \`${vars.modelName}\` |

### 4. 保存，开聊。

### 常见问题
- **"401 未授权"** — 重新复制 Key，注意前后不要带空格。
- **模型无响应** — 下拉里换 \`gpt-4o-mini\` 试试。
`,
  }),
}
