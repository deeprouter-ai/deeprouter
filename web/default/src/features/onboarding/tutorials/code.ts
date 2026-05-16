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

export const CODE: Tutorial = {
  slug: 'code',
  label: 'Python / Node code',
  descriptionKey: 'Call DeepRouter directly from your application.',
  icon: 'code',
  content: (vars: TutorialVars): TutorialContent => ({
    en: `## Call DeepRouter from code

The endpoint is OpenAI-compatible, so use the official OpenAI client library.

### Python
\`\`\`python
from openai import OpenAI

client = OpenAI(
    api_key="<your-deeprouter-key>",
    base_url="${vars.baseUrl}",
)

resp = client.chat.completions.create(
    model="${vars.modelName}",
    messages=[{"role": "user", "content": "Hello"}],
)
print(resp.choices[0].message.content)
\`\`\`

### Node.js
\`\`\`js
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: '<your-deeprouter-key>',
  baseURL: '${vars.baseUrl}',
})

const resp = await client.chat.completions.create({
  model: '${vars.modelName}',
  messages: [{ role: 'user', content: 'Hello' }],
})
console.log(resp.choices[0].message.content)
\`\`\`

### cURL
\`\`\`bash
curl ${vars.baseUrl}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <your-deeprouter-key>" \\
  -d '{
    "model": "${vars.modelName}",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
\`\`\`

### Streaming
Add \`stream: true\` to the request. Both Python and Node SDKs support \`for chunk in stream\` patterns.

### Common errors
- **401** — bad API key. Re-copy from DeepRouter.
- **404** — bad base URL. Should end with \`/v1\` (no trailing slash).
- **429** — rate limit. Wait or upgrade your plan.
- **402** — out of quota. Top up at [Wallet](/wallet).
`,
    zh: `## 在代码里调用 DeepRouter

端点是 OpenAI 兼容的，直接用官方 OpenAI 客户端库即可。

### Python
\`\`\`python
from openai import OpenAI

client = OpenAI(
    api_key="<你的 deeprouter key>",
    base_url="${vars.baseUrl}",
)

resp = client.chat.completions.create(
    model="${vars.modelName}",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)
\`\`\`

### Node.js
\`\`\`js
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: '<你的 deeprouter key>',
  baseURL: '${vars.baseUrl}',
})

const resp = await client.chat.completions.create({
  model: '${vars.modelName}',
  messages: [{ role: 'user', content: '你好' }],
})
console.log(resp.choices[0].message.content)
\`\`\`

### cURL
\`\`\`bash
curl ${vars.baseUrl}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <你的 deeprouter key>" \\
  -d '{
    "model": "${vars.modelName}",
    "messages": [{"role": "user", "content": "你好"}]
  }'
\`\`\`

### 流式返回
请求里加 \`stream: true\`。Python 和 Node SDK 都支持 \`for chunk in stream\` 这种用法。

### 常见错误
- **401** — Key 错误。从 DeepRouter 重新复制。
- **404** — base URL 错。应该以 \`/v1\` 结尾（不带斜杠）。
- **429** — 限流。等一会或升级套餐。
- **402** — 余额不足。去 [钱包](/wallet) 充值。
`,
  }),
}
