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

export const CLAUDE_CODE: Tutorial = {
  slug: 'claude-code',
  label: 'Claude Code',
  descriptionKey: "Anthropic's CLI agent — route via DeepRouter using Anthropic-format keys.",
  icon: 'terminal',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Use DeepRouter with Claude Code

Claude Code talks the Anthropic Messages API. DeepRouter exposes an Anthropic-compatible endpoint at \`${vars.baseUrl.replace(/\/v1\/?$/, '')}/v1/messages\`.

### 1. Install Claude Code
\`\`\`bash
npm install -g @anthropic-ai/claude-code
\`\`\`

### 2. Configure environment variables
Set these in your shell rc file (\`~/.zshrc\`, \`~/.bashrc\`, etc):

\`\`\`bash
export ANTHROPIC_API_KEY="<your-deeprouter-key>"
export ANTHROPIC_BASE_URL="${vars.baseUrl.replace(/\/v1\/?$/, '')}"
\`\`\`

Reload your shell.

### 3. Run claude
\`\`\`bash
cd your-project
claude
\`\`\`

### 4. Pick a model
At the prompt, \`/model\` then pick \`claude-sonnet-4-7\` or \`${vars.modelName}\`.

### Troubleshooting
- **"Invalid API key"** — verify the env var is set in the same shell session.
- **"Rate limit / quota exceeded"** — top up at [Wallet](/wallet).
`,
    zh: `## 用 Claude Code 走 DeepRouter

Claude Code 用的是 Anthropic Messages API。DeepRouter 暴露了 Anthropic 兼容端点：\`${vars.baseUrl.replace(/\/v1\/?$/, '')}/v1/messages\`。

### 1. 安装 Claude Code
\`\`\`bash
npm install -g @anthropic-ai/claude-code
\`\`\`

### 2. 配置环境变量
加进 shell rc 文件（\`~/.zshrc\` / \`~/.bashrc\` 等）：

\`\`\`bash
export ANTHROPIC_API_KEY="<你的 deeprouter key>"
export ANTHROPIC_BASE_URL="${vars.baseUrl.replace(/\/v1\/?$/, '')}"
\`\`\`

重新加载 shell。

### 3. 跑 claude
\`\`\`bash
cd your-project
claude
\`\`\`

### 4. 选模型
进入交互后 \`/model\`，选 \`claude-sonnet-4-7\` 或 \`${vars.modelName}\`。

### 常见问题
- **"Invalid API key"** — 确保环境变量在当前 shell session 里生效。
- **"Rate limit / quota exceeded"** — 去 [钱包](/wallet) 充值。
`,
  }),
}
