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
  descriptionKey:
    "Anthropic's terminal coding agent — for developers. Routes via DeepRouter's Anthropic-compatible endpoint.",
  icon: 'terminal',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Use DeepRouter with Claude Code

> **The official Claude app (desktop / phone / claude.ai) cannot be used with DeepRouter.** It has no setting to change the server address, so it can only talk to Anthropic directly. There is no workaround — it's not a missing step.
>
> **If you don't write code or use a terminal**, don't use Claude Code below. Instead install a desktop app that lets you paste a server address — **[Cherry Studio](/onboarding/cherry-studio)** or **[Chatbox](/onboarding/chatbox)** — where you can still pick Claude models. That's the path built for you.

Claude Code is a **command-line tool for developers**. It speaks the Anthropic Messages API, and DeepRouter exposes an Anthropic-compatible endpoint at \`${vars.baseUrl.replace(/\/v1\/?$/, '')}/v1/messages\`.

### 1. Install Claude Code
\`\`\`bash
npm install -g @anthropic-ai/claude-code
\`\`\`

### 2. Configure environment variables
Set these in your shell rc file (\`~/.zshrc\`, \`~/.bashrc\`, etc), then reload your shell:

\`\`\`bash
export ANTHROPIC_BASE_URL="${vars.baseUrl.replace(/\/v1\/?$/, '')}"
export ANTHROPIC_AUTH_TOKEN="<your-deeprouter-key>"
export ANTHROPIC_MODEL="${vars.modelName}"
export ANTHROPIC_SMALL_FAST_MODEL="${vars.modelName}"
\`\`\`

> Use \`ANTHROPIC_AUTH_TOKEN\` (not \`ANTHROPIC_API_KEY\`) — your DeepRouter key is a gateway token, not a native Anthropic key.

### 3. Run claude
\`\`\`bash
cd your-project
claude
\`\`\`

DeepRouter picks the right Claude model for each request, so you don't need to set one manually.

### Troubleshooting
- **"Invalid API key"** — verify \`ANTHROPIC_AUTH_TOKEN\` is set in the same shell session (run \`echo $ANTHROPIC_AUTH_TOKEN\`).
- **"Rate limit / quota exceeded"** — top up at [Wallet](/wallet).
`,
    zh: `## 用 Claude Code 走 DeepRouter

> **官方 Claude App（桌面 / 手机 / claude.ai）无法连 DeepRouter。** 它没有"修改服务器地址"的设置项，只能直连 Anthropic 官方。这不是少了哪一步——而是这条路本身不存在，没有任何变通办法。
>
> **如果你不写代码、不用命令行**，不要用下面的 Claude Code。请改装一个能填服务器地址的桌面客户端——**[Cherry Studio](/onboarding/cherry-studio)** 或 **[Chatbox](/onboarding/chatbox)**——在里面同样能选 Claude 模型。那才是为你准备的路径。

Claude Code 是**给开发者用的命令行工具**。它用的是 Anthropic Messages API，DeepRouter 暴露了 Anthropic 兼容端点：\`${vars.baseUrl.replace(/\/v1\/?$/, '')}/v1/messages\`。

### 1. 安装 Claude Code
\`\`\`bash
npm install -g @anthropic-ai/claude-code
\`\`\`

### 2. 配置环境变量
加进 shell rc 文件（\`~/.zshrc\` / \`~/.bashrc\` 等），然后重新加载 shell：

\`\`\`bash
export ANTHROPIC_BASE_URL="${vars.baseUrl.replace(/\/v1\/?$/, '')}"
export ANTHROPIC_AUTH_TOKEN="<你的 deeprouter key>"
export ANTHROPIC_MODEL="${vars.modelName}"
export ANTHROPIC_SMALL_FAST_MODEL="${vars.modelName}"
\`\`\`

> 用 \`ANTHROPIC_AUTH_TOKEN\`（不是 \`ANTHROPIC_API_KEY\`）——你的 DeepRouter key 是网关令牌，不是 Anthropic 原生 key。

### 3. 跑 claude
\`\`\`bash
cd your-project
claude
\`\`\`

DeepRouter 会为每次请求自动选合适的 Claude 模型，你不需要手动指定。

### 常见问题
- **"Invalid API key"** — 确保 \`ANTHROPIC_AUTH_TOKEN\` 在当前 shell session 里生效（跑 \`echo $ANTHROPIC_AUTH_TOKEN\` 检查）。
- **"Rate limit / quota exceeded"** — 去 [钱包](/wallet) 充值。
`,
  }),
}
