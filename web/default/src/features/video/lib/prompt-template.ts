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
 * One model the video flow can write instructions for. The page renders no
 * model menu — the menu lives inside the prompt (`menuLine`, prices from the
 * Video First Wave PRD §4 = the gateway's seeded price table) and users switch
 * by telling the AI. Both strings carry their own languages instead of going
 * through i18next, because they are embedded in the prompt text.
 */
export interface VideoModelOption {
  id: string
  /** The verification-run line inside the prompt: params + expected cost. */
  testRun: Record<PromptLanguage, string>
  /** The alternatives-menu line written into the project-memory rules. */
  menuLine: Record<PromptLanguage, string>
}

// 🔴 Ordered cheapest first. A key is only granted the video models the
// account actually has enabled (internal/keypurpose), so the caller filters
// this list down to what the bound key may call and the first survivor becomes
// the default — never a hardcoded one the key would get a 403 on.
export const VIDEO_MODELS: VideoModelOption[] = [
  {
    id: 'MiniMax-H3',
    testRun: {
      zh: '6 秒、768P（约 $0.48）',
      en: '6 seconds, 768P (≈ $0.48)',
    },
    menuLine: {
      zh: 'MiniMax-H3：4–15 秒，768P（$0.08/秒）或 2K（$0.13/秒），自带音效',
      en: 'MiniMax-H3: 4–15 s, 768P ($0.08/s) or 2K ($0.13/s), sound included',
    },
  },
  {
    id: 'doubao-seedance-2-0-260128',
    testRun: {
      zh: '默认档（约 $1.0）',
      en: 'default settings (≈ $1.0)',
    },
    menuLine: {
      zh: 'doubao-seedance-2-0-260128：5 秒 1080p，约 $1.0/条',
      en: 'doubao-seedance-2-0-260128: 5 s 1080p, ≈ $1.0/clip',
    },
  },
  {
    id: 'doubao-seedance-2-5-260628',
    testRun: {
      zh: '默认档（约 $5.4，此模型单条较贵）',
      en: 'default settings (≈ $5.4 — this model is pricey per clip)',
    },
    menuLine: {
      zh: 'doubao-seedance-2-5-260628：最长 30 秒、4K、同步音轨，约 $5.4/条',
      en: 'doubao-seedance-2-5-260628: up to 30 s, 4K, synced audio, ≈ $5.4/clip',
    },
  },
]

export const DEFAULT_VIDEO_MODEL = VIDEO_MODELS[0]

/**
 * Build the paste-prompt: one block of text a student pastes into Claude Code
 * or Codex, which then configures itself and offers a first clip (the test
 * run costs real money, so the agent must ask before spending).
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
  /**
   * The models the bound key may actually call, cheapest first. The first one
   * becomes the default and the test run; the rest are the in-prompt menu.
   * Empty falls back to the full list, which only happens with no key bound.
   */
  models: VideoModelOption[]
  language: PromptLanguage
}): string {
  const { scriptUrl, language } = options
  const models = options.models.length > 0 ? options.models : VIDEO_MODELS
  return language === 'zh'
    ? buildZhPrompt(scriptUrl, models)
    : buildEnPrompt(scriptUrl, models)
}

// buildZhPrompt renders the Chinese prompt (UI locale zh).
function buildZhPrompt(
  scriptUrl: string,
  models: VideoModelOption[]
): string {
  const model = models[0]
  const menu = models.map((m) => `- ${m.menuLine.zh}`).join('\n')
  // `size` is MiniMax-only; naming it for a key that cannot call MiniMax
  // invites the agent to send a field the chosen model rejects.
  const sizeNote = models.some((m) => m.id === 'MiniMax-H3')
    ? `；\"size\"（仅 MiniMax-H3，"768P" 或 "2K"）`
    : ''
  return `请严格按下面三步，为当前项目配置「用 DeepRouter 生成视频」的能力。除这些步骤外不要做其他改动。
前提：这三步必须在用户的电脑上、当前项目目录里执行。如果你无法读写用户电脑上的文件（比如你是网页版 AI，没有终端），不要执行任何步骤，直接告诉用户：请在电脑上打开 Claude Code 或 Codex 这类 AI 编程工具，把这段话粘贴进去。

【第 1 步：取得凭证】
用 GET 请求这个地址（30 分钟内有效，只能成功取用一次）：
${scriptUrl}?format=env
响应是几行 KEY='VALUE' 形式的纯文本，读出 DR_BASE_URL（服务器地址）和 DR_API_KEY（调用密钥）的值。
如果响应里出现 DR_ERROR、或找不到这两个变量，说明链接已失效：停下来，请用户回视频生成页重新复制一份（每次复制都是新链接），不要反复重试旧地址。
把两个值写入本项目根目录的 .env 文件：
DEEPROUTER_BASE_URL=<DR_BASE_URL 的值>
DEEPROUTER_API_KEY=<DR_API_KEY 的值>
并确认 .env 已在 .gitignore 里（没有就加上）。密钥明文不得出现在任何输出、日志或记忆文件中。

【第 2 步：写入项目记忆】
把下面整段规则原样写进项目记忆文件——Claude Code 写 CLAUDE.md，Codex 写 AGENTS.md（不存在就创建，存在就追加）：

## 生成视频（DeepRouter）
当用户要求生成视频时，按以下流程执行。凭证从 .env 读取 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY，所有请求都带 Header「Authorization: Bearer {DEEPROUTER_API_KEY}」。完整接口说明：https://deeprouter.co/llms.txt（自建网关则在 {DEEPROUTER_BASE_URL}/llms.txt）——遇到下面没写到的参数、状态、报错，先读它。
0. 先确认：GET {DEEPROUTER_BASE_URL}/v1/models，要用的模型必须在返回列表里，且它的 supported_endpoint_types 含 video-generation；不在列表里就告诉用户这把密钥没有开通该模型，不要换别的接口去试。
1. 提交：POST {DEEPROUTER_BASE_URL}/v1/video/generations
   Body: {"model":"<模型>","prompt":"<用户的描述，翻成英文效果更好>"}
   可选字段：\"duration\"（秒）${sizeNote}。响应里的 id 即任务号。
2. 轮询：每 10 秒 GET {DEEPROUTER_BASE_URL}/v1/videos/{任务号}，
   status 为 "completed" 即完成；为 "failed" 则把 error.message 告诉用户并停止。
3. 下载：GET {DEEPROUTER_BASE_URL}/v1/videos/{任务号}/content，存为 videos/<日期时间>.mp4。
4. 交付：尝试用系统默认播放器打开（Windows 用 start，macOS 用 open，Linux 用 xdg-open）；无论是否打开成功，都必须输出文件的绝对路径。
本密钥可用的模型与大约花费（生成前先告诉用户本次预计花多少；不在此列的模型本密钥调不了，不要试）：
${menu}
默认用 ${model.id}。
若用户要求移除视频配置：删除 .env 中的 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY 两行（.env 仅含这两行时可整个删掉），并删除本段规则。

【第 3 步：验证一次（可选，这一步会花钱）】
先问用户要不要立刻生成一条测试视频来验证配置——参数用 ${model.testRun.zh}，把预计花费说清楚。用户同意才执行：按上面的流程，用模型 ${model.id}，提示词自拟（比如"海边日出，海浪拍打礁石"），完成后打开播放器或输出文件路径。用户不想花这笔钱就跳过，配置本身已经完成，不受影响。

最后向用户报告：配置写进了哪些文件、测试视频存在哪里（如果生成了），并讲清楚：这段话只需要在最开始粘贴这一次，以后在这个项目里直接说「生成视频」就行，不需要再跑这段配置。`
}

// buildEnPrompt renders the English prompt (any non-zh UI locale).
function buildEnPrompt(
  scriptUrl: string,
  models: VideoModelOption[]
): string {
  const model = models[0]
  const menu = models.map((m) => `- ${m.menuLine.en}`).join('\n')
  const sizeNote = models.some((m) => m.id === 'MiniMax-H3')
    ? `; \"size\" (MiniMax-H3 only, "768P" or "2K")`
    : ''
  return `Follow these three steps exactly to set up "video generation via DeepRouter" for the current project. Do not make any other changes.
Precondition: these steps must run on the user's computer, inside the current project directory. If you cannot read or write files on the user's machine (for example, you are a web-based AI with no terminal), do not run any step — tell the user to open an AI coding tool on their computer (Claude Code or Codex) and paste this text there.

[Step 1: Get the credential]
Send a GET request to this URL (valid for 30 minutes, works once):
${scriptUrl}?format=env
The response is a few lines of plain text in KEY='VALUE' form. Read the values of DR_BASE_URL (the server address) and DR_API_KEY (the API key).
If the response contains DR_ERROR, or those two variables are missing, the link is dead: stop and ask the user to go back to the video page and copy a fresh prompt (every copy carries a new link). Do not retry the old URL.
Write the two values into a .env file at the project root:
DEEPROUTER_BASE_URL=<value of DR_BASE_URL>
DEEPROUTER_API_KEY=<value of DR_API_KEY>
Make sure .env is listed in .gitignore (add it if missing). The key must never appear in plain text in any output, log, or memory file.

[Step 2: Write the project memory]
Write the following rules verbatim into the project memory file — CLAUDE.md for Claude Code, AGENTS.md for Codex (create it if missing, append if it exists):

## Video generation (DeepRouter)
When the user asks to generate a video, follow this flow. Read DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY from .env; every request carries the header "Authorization: Bearer {DEEPROUTER_API_KEY}". Full API reference: https://deeprouter.co/llms.txt (on a self-hosted gateway, {DEEPROUTER_BASE_URL}/llms.txt) — read it first for any parameter, status or error not covered below.
0. Check first: GET {DEEPROUTER_BASE_URL}/v1/models — the model you will use must be listed and its supported_endpoint_types must include video-generation; if it is not listed, tell the user this key has no access to it instead of trying another endpoint.
1. Submit: POST {DEEPROUTER_BASE_URL}/v1/video/generations
   Body: {"model":"<model>","prompt":"<the user's description>"}
   Optional fields: \"duration\" (seconds)${sizeNote}. The id in the response is the task id.
2. Poll: GET {DEEPROUTER_BASE_URL}/v1/videos/{task id} every 10 seconds.
   Status "completed" means done; on "failed", tell the user what error.message says and stop.
3. Download: GET {DEEPROUTER_BASE_URL}/v1/videos/{task id}/content and save it as videos/<timestamp>.mp4.
4. Deliver: try to open it with the system default player (start on Windows, open on macOS, xdg-open on Linux); whether or not that works, always print the file's absolute path.
Models this key can use and their approximate cost (tell the user the expected cost before generating; anything not listed here this key cannot call, so do not try):
${menu}
Default model: ${model.id}.
If the user asks to remove the video setup: delete the DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY lines from .env (delete the whole file if those are its only lines), and delete this section of rules.

[Step 3: Verify once (optional — this step costs money)]
First ask the user whether to generate one test clip now to verify the setup — parameters: ${model.testRun.en} — and spell out the expected cost. Only proceed if they agree: use the flow above with model ${model.id}, pick any prompt you like (e.g. "sunrise over the sea, waves hitting the rocks"), and when it finishes, open the player or print the file path. If they would rather not spend it, skip this step — the setup is already complete.

Finally, report to the user: which files were configured, where the test clip is saved (if one was made), and make this clear: this text only needs to be pasted once, at the very beginning — from now on they can simply say "generate a video" in this project; there is no need to run this setup again.`
}
