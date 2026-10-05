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
/** Which language the paste-prompt itself is written in (follows the UI locale). */
export type PromptLanguage = 'zh' | 'en'

/**
 * One entry of the model menu on the video page; also drives the model line
 * and the test-run cost inside the paste-prompt.
 *
 * Prices are the per-clip figures from the Video First Wave PRD §4 — the same
 * numbers P1 seeded into the gateway's price table (margin pending PRD Q3, so
 * recalibrate here when that lands). `traits` / `price` are English i18n
 * source strings; zh lives in the locale file like every other UI string.
 * `testRun` is embedded into the prompt text, so it carries both languages
 * itself instead of going through i18next.
 */
export interface VideoModelOption {
  id: string
  name: string
  traits: string
  price: string
  /** The verification-run line inside the prompt: params + expected cost. */
  testRun: Record<PromptLanguage, string>
}

export const VIDEO_MODELS: VideoModelOption[] = [
  {
    id: 'MiniMax-H3',
    name: 'MiniMax-H3',
    traits: '4–15 s · up to 2K · sound included',
    price: '6 s ≈ $0.48 (768P) / $0.78 (2K)',
    testRun: {
      zh: '6 秒、768P（约 $0.48）',
      en: '6 seconds, 768P (≈ $0.48)',
    },
  },
  {
    id: 'doubao-seedance-2-5-260628',
    name: 'Seedance 2.5',
    traits: 'up to 30 s · 4K · synced sound',
    price: '≈ $5.4 / clip',
    testRun: {
      zh: '默认档（约 $5.4，此模型单条较贵）',
      en: 'default settings (≈ $5.4 — this model is pricey per clip)',
    },
  },
  {
    id: 'doubao-seedance-2-0-260128',
    name: 'Seedance 2.0',
    traits: '5 s · 1080p',
    price: '≈ $1.0 / clip',
    testRun: {
      zh: '默认档（约 $1.0）',
      en: 'default settings (≈ $1.0)',
    },
  },
]

export const DEFAULT_VIDEO_MODEL = VIDEO_MODELS[0]

/**
 * Build the paste-prompt: one block of text a student pastes into Claude Code
 * or Codex, which then configures itself and runs a first clip.
 *
 * Three layers per the PRD §5 B1 design — credential (one-time token URL,
 * never the key itself), method (submit → poll → download → open player /
 * print path), memory (rules written into CLAUDE.md / AGENTS.md so a plain
 * "generate a video" works from then on). The text is addressed to the AI
 * tool, not the student, so API terms are fine here; the page around it stays
 * jargon-free. Written in the UI's language so an English-mode user does not
 * paste a block of Chinese they cannot read.
 */
export function buildVideoPrompt(options: {
  /** Full redeem URL (`{base}/i/{token}`) minted for the current user. */
  scriptUrl: string
  model: VideoModelOption
  language: PromptLanguage
}): string {
  const { scriptUrl, model, language } = options
  return language === 'zh'
    ? buildZhPrompt(scriptUrl, model)
    : buildEnPrompt(scriptUrl, model)
}

// buildZhPrompt renders the Chinese prompt (UI locale zh).
function buildZhPrompt(scriptUrl: string, model: VideoModelOption): string {
  return `请严格按下面三步，为当前项目配置「用 DeepRouter 生成视频」的能力。除这些步骤外不要做其他改动。

【第 1 步：取得凭证】
用 GET 请求这个地址（30 分钟内有效，只能用一次）：
${scriptUrl}
返回的是一段脚本文本，从中读出两个变量的值（都在引号里）：DR_BASE_URL（服务器地址）和 DR_API_KEY（调用密钥）。
把它们写入本项目根目录的 .env 文件：
DEEPROUTER_BASE_URL=<DR_BASE_URL 的值>
DEEPROUTER_API_KEY=<DR_API_KEY 的值>
并确认 .env 已在 .gitignore 里（没有就加上）。密钥明文不得出现在任何输出、日志或记忆文件中。

【第 2 步：写入项目记忆】
把下面整段规则原样写进项目记忆文件——Claude Code 写 CLAUDE.md，Codex 写 AGENTS.md（不存在就创建，存在就追加）：

## 生成视频（DeepRouter）
当用户要求生成视频时，按以下流程执行。凭证从 .env 读取 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY，所有请求都带 Header「Authorization: Bearer {DEEPROUTER_API_KEY}」。完整接口说明：https://deeprouter.co/llms.txt
0. 先确认：GET {DEEPROUTER_BASE_URL}/v1/models，要用的模型必须在返回列表里，且它的 supported_endpoint_types 含 video-generation；不在列表里就告诉用户这把密钥没有开通该模型，不要换别的接口去试。
1. 提交：POST {DEEPROUTER_BASE_URL}/v1/video/generations
   Body: {"model":"<模型>","prompt":"<用户的描述，翻成英文效果更好>"}
   可选字段：\"duration\"（秒）；\"size\"（仅 MiniMax-H3，"768P" 或 "2K"）。响应里的 id 即任务号。
2. 轮询：每 10 秒 GET {DEEPROUTER_BASE_URL}/v1/video/generations/{任务号}，
   status 为 "completed" 即完成；为 "failed" 则把错误信息告诉用户并停止。
3. 下载：GET {DEEPROUTER_BASE_URL}/v1/videos/{任务号}/content，存为 videos/<日期时间>.mp4。
4. 交付：尝试用系统默认播放器打开（Windows 用 start，macOS 用 open，Linux 用 xdg-open）；无论是否打开成功，都必须输出文件的绝对路径。
可选模型与大约花费（生成前先告诉用户本次预计花多少）：
- MiniMax-H3：4–15 秒，768P（$0.08/秒）或 2K（$0.13/秒），自带音效
- doubao-seedance-2-5-260628：最长 30 秒、4K、同步音轨，约 $5.4/条
- doubao-seedance-2-0-260128：5 秒 1080p，约 $1.0/条
默认用 ${model.id}。
若用户要求移除视频配置：删除 .env 中的 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY 两行（.env 仅含这两行时可整个删掉），并删除本段规则。

【第 3 步：立刻验证一次】
按上面的流程，用模型 ${model.id} 生成一条测试视频，参数用 ${model.testRun.zh}，提示词自拟（比如"海边日出，海浪拍打礁石"），完成后打开播放器或输出文件路径。

三步都完成后向用户报告：配置写进了哪些文件、测试视频存在哪里，并提醒：以后在这个项目里直接说「生成视频」就可以了。`
}

// buildEnPrompt renders the English prompt (any non-zh UI locale).
function buildEnPrompt(scriptUrl: string, model: VideoModelOption): string {
  return `Follow these three steps exactly to set up "video generation via DeepRouter" for the current project. Do not make any other changes.

[Step 1: Get the credential]
Send a GET request to this URL (valid for 30 minutes, single use):
${scriptUrl}
The response is a script text. Read two variable values out of it (both quoted): DR_BASE_URL (the server address) and DR_API_KEY (the API key).
Write them into a .env file at the project root:
DEEPROUTER_BASE_URL=<value of DR_BASE_URL>
DEEPROUTER_API_KEY=<value of DR_API_KEY>
Make sure .env is listed in .gitignore (add it if missing). The key must never appear in plain text in any output, log, or memory file.

[Step 2: Write the project memory]
Write the following rules verbatim into the project memory file — CLAUDE.md for Claude Code, AGENTS.md for Codex (create it if missing, append if it exists):

## Video generation (DeepRouter)
When the user asks to generate a video, follow this flow. Read DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY from .env; every request carries the header "Authorization: Bearer {DEEPROUTER_API_KEY}". Full API reference: https://deeprouter.co/llms.txt
0. Check first: GET {DEEPROUTER_BASE_URL}/v1/models — the model you will use must be listed and its supported_endpoint_types must include video-generation; if it is not listed, tell the user this key has no access to it instead of trying another endpoint.
1. Submit: POST {DEEPROUTER_BASE_URL}/v1/video/generations
   Body: {"model":"<model>","prompt":"<the user's description>"}
   Optional fields: \"duration\" (seconds); \"size\" (MiniMax-H3 only, "768P" or "2K"). The id in the response is the task id.
2. Poll: GET {DEEPROUTER_BASE_URL}/v1/video/generations/{task id} every 10 seconds.
   Status "completed" means done; on "failed", tell the user the error message and stop.
3. Download: GET {DEEPROUTER_BASE_URL}/v1/videos/{task id}/content and save it as videos/<timestamp>.mp4.
4. Deliver: try to open it with the system default player (start on Windows, open on macOS, xdg-open on Linux); whether or not that works, always print the file's absolute path.
Available models and approximate cost (tell the user the expected cost before generating):
- MiniMax-H3: 4–15 s, 768P ($0.08/s) or 2K ($0.13/s), sound included
- doubao-seedance-2-5-260628: up to 30 s, 4K, synced audio, ≈ $5.4/clip
- doubao-seedance-2-0-260128: 5 s 1080p, ≈ $1.0/clip
Default model: ${model.id}.
If the user asks to remove the video setup: delete the DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY lines from .env (delete the whole file if those are its only lines), and delete this section of rules.

[Step 3: Verify right now]
Using the flow above, generate one test clip with model ${model.id}, parameters: ${model.testRun.en}. Pick any prompt you like (e.g. "sunrise over the sea, waves hitting the rocks"). When it finishes, open the player or print the file path.

When all three steps are done, report to the user: which files were configured, where the test clip is saved, and that from now on they can simply say "generate a video" in this project.`
}
