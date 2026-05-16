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

export const LOBECHAT: Tutorial = {
  slug: 'lobechat',
  label: 'LobeChat',
  descriptionKey: 'Open-source AI chat with plugins and multi-modal support.',
  icon: 'lobe',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Connect DeepRouter to LobeChat

[LobeChat](https://lobehub.com/chat) supports OpenAI-compatible endpoints and ships with a rich plugin ecosystem.

### 1. Open LobeChat
Either the hosted [lobechat.com](https://lobechat.com) or self-hosted Docker.

### 2. Settings → Language Model
Click your avatar → **Settings** → **Language Model** → **OpenAI**.

### 3. Configure

| Field | Value |
|---|---|
| API Key | DeepRouter key |
| API Proxy Address | \`${vars.baseUrl}\` |
| Custom Model Name | \`${vars.modelName}\` |

Enable the toggle for **Use Client-Side Request** if you're running self-hosted and want direct calls.

### 4. Pick **OpenAI** as the chat provider in the chat session, choose your custom model, send a message.

### Troubleshooting
- **CORS errors (self-hosted)** — enable "Use Client-Side Request" or proxy through your LobeChat backend.
`,
    zh: `## 在 LobeChat 里接入 DeepRouter

[LobeChat](https://lobehub.com/chat) 支持 OpenAI 兼容接口，自带丰富的插件生态。

### 1. 打开 LobeChat
托管版 [lobechat.com](https://lobechat.com) 或自部署 Docker。

### 2. 设置 → 语言模型
点头像 → **设置** → **语言模型** → **OpenAI**。

### 3. 填配置

| 字段 | 内容 |
|---|---|
| API Key | DeepRouter 创建的 Key |
| API 代理地址 | \`${vars.baseUrl}\` |
| 自定义模型名 | \`${vars.modelName}\` |

如果是自部署版本，建议打开"客户端发送请求"开关。

### 4. 在对话里选 **OpenAI**，选你的自定义模型，发消息。

### 常见问题
- **CORS 报错（自部署）** — 打开"客户端发送请求"，或者通过 LobeChat 后端转发。
`,
  }),
}
