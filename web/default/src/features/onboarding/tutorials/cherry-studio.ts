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

export const CHERRY_STUDIO: Tutorial = {
  slug: 'cherry-studio',
  label: 'Cherry Studio',
  descriptionKey: 'Friendly desktop client — recommended for chat and writing.',
  icon: 'cherry',
  recommended: true,
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Connect DeepRouter to Cherry Studio

Cherry Studio is a free desktop AI client — the easiest way to start using DeepRouter without writing code.

### 1. Install Cherry Studio
Download from [cherry-ai.com](https://cherry-ai.com) and install. Available for macOS, Windows and Linux.

### 2. Add DeepRouter as a provider
- Open Cherry Studio
- Click the **Settings** ⚙️ icon (bottom-left)
- Go to **Model Providers** → click **+ Add provider**
- Pick **OpenAI-compatible** as the provider type

### 3. Fill in these fields

| Field | Value |
|---|---|
| Provider Name | DeepRouter |
| API Key | the key you just created (starts with \`sk-\`) |
| API URL | \`${vars.baseUrl}\` |
| Model | \`${vars.modelName}\` |

### 4. Start chatting
Pick **DeepRouter** in the model dropdown and say hi. That's it.

### Troubleshooting
- **"Connection refused"** — double-check the API URL. It should end with \`/v1\`.
- **"Invalid model"** — try \`gpt-4o\` or \`claude-sonnet-4-7\` instead of \`${vars.modelName}\`. The virtual alias is best for Simple-mode keys.
- **"Insufficient balance"** — top up at the [Wallet](/wallet) page.

> Screenshots coming soon. If you get stuck, ask on our community forum.
`,
    zh: `## 在 Cherry Studio 里接入 DeepRouter

Cherry Studio 是一款免费桌面端 AI 客户端 —— 最适合非技术用户起步，不用写代码就能用上 DeepRouter。

### 1. 安装 Cherry Studio
从 [cherry-ai.com](https://cherry-ai.com) 下载安装。macOS / Windows / Linux 都有。

### 2. 添加 DeepRouter 模型供应商
- 打开 Cherry Studio
- 点左下角 **设置** ⚙️ 图标
- 进入 **模型服务** → 点 **+ 添加**
- 类型选 **OpenAI 兼容**

### 3. 填以下字段

| 字段 | 内容 |
|---|---|
| 服务商名称 | DeepRouter |
| API 密钥 | 你刚刚创建的 Key（\`sk-\` 开头） |
| API 地址 | \`${vars.baseUrl}\` |
| 默认模型 | \`${vars.modelName}\` |

### 4. 开聊
在模型下拉里选 **DeepRouter**，发"你好"。完事。

### 常见问题
- **"连接失败"** — 检查 API 地址，确保以 \`/v1\` 结尾。
- **"模型无效"** — 试试 \`gpt-4o\` 或 \`claude-sonnet-4-7\`，虚拟别名适合 Simple Key。
- **"余额不足"** — 去 [钱包](/wallet) 充值。

> 截图教程整理中。卡住了可以来社区问。
`,
  }),
}
