// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import {
  buildVideoPrompt,
  DEFAULT_VIDEO_MODEL,
  type PromptLanguage,
} from '@/features/video/lib/prompt-template'
import type { SimplePurposeId } from './purposes'

const GUIDE_INDEX = 'https://deeprouter.co/llms.txt'

type Lang = Record<PromptLanguage, string>

interface PurposeRecipe {
  /** Section heading written into the agent's project memory. */
  heading: Lang
  /** How to call the gateway for this purpose, appended to the rules. */
  method: Lang
  /** The one cheap run the agent performs right away to prove it works. */
  verify: Lang
}

/**
 * Non-video purposes. Every recipe tells the agent to discover first
 * (`GET /v1/models` with this key) and to pick the endpoint from each model's
 * `supported_endpoint_types` — never to guess support from a model name.
 */
const RECIPES: Record<Exclude<SimplePurposeId, 'video'>, PurposeRecipe> = {
  image: {
    heading: {
      zh: '生成图片（DeepRouter）',
      en: 'Image generation (DeepRouter)',
    },
    method: {
      zh: `2. 生成：POST {DEEPROUTER_BASE_URL}/v1/images/generations
   Body: {"model":"<第 1 步里支持 image-generation 的模型>","prompt":"<用户的描述>"}
3. 保存：响应 data[0] 里是 url 就下载，是 b64_json 就解码，存为 images/<日期时间>.png。
4. 交付：用系统默认程序打开图片（Windows 用 start，macOS 用 open，Linux 用 xdg-open）；无论是否打开成功，都输出文件的绝对路径。`,
      en: `2. Generate: POST {DEEPROUTER_BASE_URL}/v1/images/generations
   Body: {"model":"<a model from step 1 that lists image-generation>","prompt":"<the user's description>"}
3. Save: data[0] holds either a url (download it) or b64_json (decode it); save as images/<datetime>.png.
4. Deliver: open it with the system default app (start on Windows, open on macOS, xdg-open on Linux); whether or not that works, print the file's absolute path.`,
    },
    verify: {
      zh: '生成一张测试图片（比如"海边日出，水彩风格"），完成后打开图片或输出文件路径。',
      en: 'Generate one test picture (e.g. "sunrise over the sea, watercolour"), then open it or print its path.',
    },
  },
  voice: {
    heading: { zh: '配音（DeepRouter）', en: 'Voice-over (DeepRouter)' },
    method: {
      zh: `2. 合成：POST {DEEPROUTER_BASE_URL}/v1/audio/speech
   Body: {"model":"<第 1 步里支持 audio-speech 的模型>","input":"<要读的文字>","response_format":"mp3"}
   模型名以 gpt- 或 tts- 开头时还要加 "voice":"alloy"。
3. 保存：响应体就是音频，存为 audio/<日期时间>.mp3。
4. 交付：用系统默认播放器打开（Windows 用 start，macOS 用 open，Linux 用 xdg-open）；无论是否打开成功，都输出文件的绝对路径。`,
      en: `2. Synthesise: POST {DEEPROUTER_BASE_URL}/v1/audio/speech
   Body: {"model":"<a model from step 1 that lists audio-speech>","input":"<the text to read>","response_format":"mp3"}
   For models whose name starts with gpt- or tts-, also send "voice":"alloy".
3. Save: the response body is the audio; save it as audio/<datetime>.mp3.
4. Deliver: open it with the default player (start on Windows, open on macOS, xdg-open on Linux); whether or not that works, print the file's absolute path.`,
    },
    verify: {
      zh: '把"你好，欢迎使用 DeepRouter。"合成一段测试音频，完成后播放或输出文件路径。',
      en: 'Synthesise "Hello, and welcome to DeepRouter." as a test clip, then play it or print its path.',
    },
  },
  chat: {
    heading: {
      zh: '对话与写作（DeepRouter）',
      en: 'Chat & writing (DeepRouter)',
    },
    method: {
      zh: `2. 调用：POST {DEEPROUTER_BASE_URL}/v1/chat/completions（OpenAI 兼容）
   Body: {"model":"deeprouter-auto","messages":[{"role":"user","content":"<内容>"}]}
   deeprouter-auto 会按任务自动挑选合适的模型；用户点名某个模型时，从第 1 步的列表里选。`,
      en: `2. Call: POST {DEEPROUTER_BASE_URL}/v1/chat/completions (OpenAI-compatible)
   Body: {"model":"deeprouter-auto","messages":[{"role":"user","content":"<content>"}]}
   deeprouter-auto picks a suitable model per task; when the user names a model, take it from the step 1 list.`,
    },
    verify: {
      zh: '发一条"用一句话介绍你自己"，把回复原样告诉用户。',
      en: 'Send "Introduce yourself in one sentence" and show the user the reply.',
    },
  },
  coding: {
    heading: { zh: '用 DeepRouter 写代码', en: 'Coding with DeepRouter' },
    method: {
      zh: `2. 若用户希望你（当前 AI 工具）本身改用 DeepRouter 的模型：按对应官方指南操作——
   Claude Code：https://deeprouter.co/docs/integrations/claude-code.zh.md
   Codex：https://deeprouter.co/docs/integrations/codex.zh.md
   改全局配置前必须先征得用户同意，并说明改了哪个文件。
3. 若项目代码需要调用 AI：用 OpenAI 兼容接口 POST {DEEPROUTER_BASE_URL}/v1/chat/completions，model 用 "deeprouter-auto"。`,
      en: `2. If the user wants you (this AI tool) to run on DeepRouter's models: follow the official guide —
   Claude Code: https://deeprouter.co/docs/integrations/claude-code.md
   Codex: https://deeprouter.co/docs/integrations/codex.md
   Ask the user before changing any global config, and say which file you changed.
3. If the project's code needs to call AI: use the OpenAI-compatible POST {DEEPROUTER_BASE_URL}/v1/chat/completions with model "deeprouter-auto".`,
    },
    verify: {
      zh: '用 deeprouter-auto 发一条"写一个把两个数相加的 Python 函数"，把返回的代码给用户看。',
      en: 'Send "Write a Python function that adds two numbers" with deeprouter-auto and show the user the code.',
    },
  },
}

/**
 * The text a Simple user copies and pastes into their AI tool. Same three
 * layers as the video paste-prompt (credential → project memory → one
 * verification run): the one-time token URL travels, never the key, and the
 * rules land in CLAUDE.md / AGENTS.md so later a plain request just works.
 * Video reuses the existing video prompt verbatim.
 */
export function buildPurposePrompt(options: {
  purpose: SimplePurposeId
  /** Full redeem URL (`{base}/i/{token}`) minted for the current user. */
  scriptUrl: string
  language: PromptLanguage
}): string {
  const { purpose, scriptUrl, language } = options
  if (purpose === 'video') {
    return buildVideoPrompt({ scriptUrl, model: DEFAULT_VIDEO_MODEL, language })
  }
  const recipe = RECIPES[purpose]
  return language === 'zh'
    ? zhPrompt(scriptUrl, recipe)
    : enPrompt(scriptUrl, recipe)
}

function zhPrompt(scriptUrl: string, r: PurposeRecipe): string {
  return `请严格按下面三步，为当前项目配置「${r.heading.zh}」的能力。除这些步骤外不要做其他改动。

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

## ${r.heading.zh}
凭证从 .env 读取 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY，所有请求都带 Header「Authorization: Bearer {DEEPROUTER_API_KEY}」。完整接口说明：${GUIDE_INDEX}
1. 先发现：GET {DEEPROUTER_BASE_URL}/v1/models，只使用返回列表里的模型；按每个模型的 supported_endpoint_types 选接口，不要凭模型名猜。
${r.method.zh}
若用户要求移除这项配置：删除 .env 中的 DEEPROUTER_BASE_URL 与 DEEPROUTER_API_KEY 两行，并删除本段规则。

【第 3 步：立刻验证一次】
${r.verify.zh}

三步都完成后向用户报告：配置写进了哪些文件、验证结果如何。`
}

function enPrompt(scriptUrl: string, r: PurposeRecipe): string {
  return `Follow these three steps exactly to set up "${r.heading.en}" for the current project. Do not make any other changes.

[Step 1: Get the credential]
Send a GET request to this URL (valid for 30 minutes, single use):
${scriptUrl}
The response is a script text. Read two variable values out of it (both quoted): DR_BASE_URL (the server address) and DR_API_KEY (the API key).
Write them into a .env file at the project root:
DEEPROUTER_BASE_URL=<value of DR_BASE_URL>
DEEPROUTER_API_KEY=<value of DR_API_KEY>
Make sure .env is listed in .gitignore (add it if missing). The key must never appear in plain text in any output, log, or memory file.

[Step 2: Write the project memory]
Copy the rules below verbatim into the project memory file — CLAUDE.md for Claude Code, AGENTS.md for Codex (create it if missing, append if it exists):

## ${r.heading.en}
Read DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY from .env; every request carries the header "Authorization: Bearer {DEEPROUTER_API_KEY}". Full API reference: ${GUIDE_INDEX}
1. Discover first: GET {DEEPROUTER_BASE_URL}/v1/models and use only models it lists; choose the endpoint from each model's supported_endpoint_types — never guess from the model name.
${r.method.en}
If the user asks to remove this setup: delete the DEEPROUTER_BASE_URL and DEEPROUTER_API_KEY lines from .env and delete this section.

[Step 3: Verify once, right now]
${r.verify.en}

When all three steps are done, tell the user which files you changed and how the verification went.`
}
