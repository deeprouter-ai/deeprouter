/*
Curated model metadata presets for the "Quick Import" dialog on the Models
page. Each entry seeds a model row with sensible defaults; the operator
reviews + enables individually. Models are created with status=false
(disabled) so they don't surface in /v1/models until reviewed.

This is the *metadata catalog* — separate from Channels. A model only
becomes invokable once at least one enabled Channel's `models` field
references it. See docs/PRD.md §6 for the Channel/Model relationship.

Grouping mirrors how the docs/DESIGN.md feature cards organize them:
text-completion, image, video, audio, embedding.
*/

export type ModelPresetGroup =
  | 'chat'
  | 'reasoning'
  | 'image'
  | 'video'
  | 'audio'
  | 'embedding'

export interface ModelPreset {
  model_name: string
  description: string
  group: ModelPresetGroup
  tags: string[]
  sourceUrl: string // official provider model directory; not proof of key access
  apiDocsUrl: string // official API documentation
  endpoints: string // pipe-separated endpoint types: "chat" | "image" | "audio" | "embedding"
}

export const MODEL_PRESETS: ModelPreset[] = [
  // ── Chat / Text Completion (default-grade) ───────────────────────────
  {
    model_name: 'gpt-6.1-sol',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description:
      'OpenAI GPT-6.1 Sol (2026-09-29) — near-Astra coding and agents at one-fifth the price, 1M context.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'tools', 'coding'],
    endpoints: 'chat',
  },
  {
    model_name: 'gpt-6-astra',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description:
      'OpenAI GPT-6 Astra — flagship for the hardest reasoning work.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'tools'],
    endpoints: 'chat',
  },
  {
    model_name: 'gpt-5.5',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI GPT-5.5 — previous-gen reasoning + chat.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'tools'],
    endpoints: 'chat',
  },
  {
    model_name: 'gpt-4o',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI flagship multimodal chat model — vision + tools.',
    group: 'chat',
    tags: ['vision', 'tools', 'multimodal'],
    endpoints: 'chat',
  },
  {
    model_name: 'gpt-4o-mini',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI cost-efficient small model — fast, cheap, vision.',
    group: 'chat',
    tags: ['vision', 'tools', 'cheap'],
    endpoints: 'chat',
  },
  {
    model_name: 'claude-opus-4-8',
    sourceUrl:
      'https://platform.claude.com/docs/en/about-claude/models/overview',
    apiDocsUrl: 'https://platform.claude.com/docs/en/api/overview',
    description: 'Anthropic Opus 4.8 — adaptive thinking, top-tier reasoning.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'long-context'],
    endpoints: 'chat',
  },
  {
    model_name: 'claude-opus-4-7',
    sourceUrl:
      'https://platform.claude.com/docs/en/about-claude/models/overview',
    apiDocsUrl: 'https://platform.claude.com/docs/en/api/overview',
    description: 'Anthropic Opus 4.7 — adaptive thinking, top-tier reasoning.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'long-context'],
    endpoints: 'chat',
  },
  {
    model_name: 'claude-sonnet-4-6',
    sourceUrl:
      'https://platform.claude.com/docs/en/about-claude/models/overview',
    apiDocsUrl: 'https://platform.claude.com/docs/en/api/overview',
    description: 'Anthropic Sonnet 4.6 — balanced quality + cost.',
    group: 'chat',
    tags: ['tools', 'long-context'],
    endpoints: 'chat',
  },
  {
    model_name: 'claude-3-5-haiku-latest',
    sourceUrl:
      'https://platform.claude.com/docs/en/about-claude/models/overview',
    apiDocsUrl: 'https://platform.claude.com/docs/en/api/overview',
    description: 'Anthropic Haiku — fastest, cheapest Claude.',
    group: 'chat',
    tags: ['cheap', 'fast'],
    endpoints: 'chat',
  },
  {
    model_name: 'gemini-3.1-pro',
    sourceUrl: 'https://ai.google.dev/gemini-api/docs/models',
    apiDocsUrl: 'https://ai.google.dev/gemini-api/docs',
    description: 'Google Gemini 3.1 Pro — flagship long-context multimodal.',
    group: 'chat',
    tags: ['vision', 'long-context', 'multimodal'],
    endpoints: 'chat',
  },
  {
    model_name: 'gemini-2.5-pro',
    sourceUrl: 'https://ai.google.dev/gemini-api/docs/models',
    apiDocsUrl: 'https://ai.google.dev/gemini-api/docs',
    description: 'Google Gemini 2.5 Pro — long context, multimodal.',
    group: 'chat',
    tags: ['vision', 'long-context', 'multimodal'],
    endpoints: 'chat',
  },
  {
    model_name: 'gemini-2.5-flash',
    sourceUrl: 'https://ai.google.dev/gemini-api/docs/models',
    apiDocsUrl: 'https://ai.google.dev/gemini-api/docs',
    description: 'Google Gemini 2.5 Flash — speed-optimized.',
    group: 'chat',
    tags: ['fast', 'cheap'],
    endpoints: 'chat',
  },
  {
    model_name: 'deepseek-v4-pro',
    sourceUrl: 'https://api-docs.deepseek.com/quick_start/pricing',
    apiDocsUrl: 'https://api-docs.deepseek.com/',
    description: 'DeepSeek V4 Pro — flagship, 1M context.',
    group: 'chat',
    tags: ['open-source', 'coder', 'long-context'],
    endpoints: 'chat',
  },
  {
    model_name: 'deepseek-chat',
    sourceUrl: 'https://api-docs.deepseek.com/quick_start/pricing',
    apiDocsUrl: 'https://api-docs.deepseek.com/',
    description: 'DeepSeek V4 (chat alias) — open weights, strong code.',
    group: 'chat',
    tags: ['open-source', 'coder'],
    endpoints: 'chat',
  },
  {
    model_name: 'deepseek-reasoner',
    sourceUrl: 'https://api-docs.deepseek.com/quick_start/pricing',
    apiDocsUrl: 'https://api-docs.deepseek.com/',
    description: 'DeepSeek R1 — open-weights reasoning model.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'open-source'],
    endpoints: 'chat',
  },
  {
    model_name: 'qwen3-max',
    sourceUrl: 'https://www.alibabacloud.com/help/en/model-studio/models',
    apiDocsUrl: 'https://www.alibabacloud.com/help/en/model-studio/',
    description: '阿里 Qwen3-Max — current domestic flagship, strong Chinese.',
    group: 'chat',
    tags: ['chinese', 'tools'],
    endpoints: 'chat',
  },
  {
    model_name: 'qwen-max',
    sourceUrl: 'https://www.alibabacloud.com/help/en/model-studio/models',
    apiDocsUrl: 'https://www.alibabacloud.com/help/en/model-studio/',
    description: '阿里 Qwen-max — domestic flagship, strong Chinese.',
    group: 'chat',
    tags: ['chinese', 'tools'],
    endpoints: 'chat',
  },
  {
    model_name: 'moonshot-v1-32k',
    sourceUrl: 'https://platform.moonshot.ai/docs',
    apiDocsUrl: 'https://platform.moonshot.ai/docs/api/chat',
    description: 'Moonshot Kimi v1 32k — long-context Chinese chat.',
    group: 'chat',
    tags: ['chinese', 'long-context'],
    endpoints: 'chat',
  },
  {
    model_name: 'kimi-k2-0905-preview',
    sourceUrl: 'https://platform.moonshot.ai/docs',
    apiDocsUrl: 'https://platform.moonshot.ai/docs/api/chat',
    description: 'Moonshot Kimi K2 — agentic + tool use, preview.',
    group: 'chat',
    tags: ['tools', 'preview'],
    endpoints: 'chat',
  },
  {
    model_name: 'doubao-seed-1-6-thinking-250715',
    sourceUrl: 'https://www.volcengine.com/docs/82379',
    apiDocsUrl: 'https://www.volcengine.com/docs/82379',
    description: 'Doubao Seed 1.6 thinking — Volcengine reasoning model.',
    group: 'reasoning',
    tags: ['reasoning', 'thinking', 'chinese'],
    endpoints: 'chat',
  },

  // ── Image Generation ─────────────────────────────────────────────────
  {
    model_name: 'gpt-image-2.5-flare',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description:
      'OpenAI default image model (2026-09-08) — better quality and editing than gpt-image-2 at half the latency.',
    group: 'image',
    tags: ['image', 'edit', 'fast'],
    endpoints: 'image',
  },
  {
    model_name: 'gpt-image-2.5-sunburst',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description:
      'OpenAI precision image model (2026-09-08) — detailed creative work, slower generation.',
    group: 'image',
    tags: ['image', 'precision'],
    endpoints: 'image',
  },
  {
    model_name: 'gpt-image-2',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description:
      'OpenAI previous-gen image model (2026-04-21) — built-in reasoning + 4K.',
    group: 'image',
    tags: ['image', 'reasoning', '4k'],
    endpoints: 'image',
  },
  {
    model_name: 'gpt-image-1.5',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI gpt-image-1.5 — previous-gen image.',
    group: 'image',
    tags: ['image'],
    endpoints: 'image',
  },
  {
    model_name: 'flux-1.1-pro',
    sourceUrl: 'https://docs.bfl.ai/',
    apiDocsUrl: 'https://docs.bfl.ai/',
    description: 'Black Forest Labs Flux 1.1 Pro — photoreal image.',
    group: 'image',
    tags: ['image', 'photoreal'],
    endpoints: 'image',
  },
  {
    model_name: 'flux-schnell',
    sourceUrl: 'https://docs.bfl.ai/',
    apiDocsUrl: 'https://docs.bfl.ai/',
    description: 'Flux Schnell — fast/cheap open-weight image.',
    group: 'image',
    tags: ['image', 'open-source', 'fast'],
    endpoints: 'image',
  },
  {
    model_name: 'doubao-seedream-4-0-250828',
    sourceUrl: 'https://www.volcengine.com/docs/82379',
    apiDocsUrl: 'https://www.volcengine.com/docs/82379',
    description: 'Doubao Seedream 4.0 — Volcengine image generation.',
    group: 'image',
    tags: ['image', 'chinese'],
    endpoints: 'image',
  },

  // ── Video Generation ─────────────────────────────────────────────────
  {
    model_name: 'veo-3.0-generate-001',
    sourceUrl:
      'https://cloud.google.com/vertex-ai/generative-ai/docs/models/veo/3-0-generate',
    apiDocsUrl:
      'https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/veo-video-generation',
    description: 'Google Veo 3 — text-to-video.',
    group: 'video',
    tags: ['video'],
    endpoints: 'video',
  },
  {
    model_name: 'doubao-seedance-2-0-260128',
    sourceUrl: 'https://www.volcengine.com/docs/82379',
    apiDocsUrl: 'https://www.volcengine.com/docs/82379',
    description: 'Doubao Seedance 2.0 — Volcengine text-to-video.',
    group: 'video',
    tags: ['video', 'chinese'],
    endpoints: 'video',
  },
  {
    model_name: 'kling-v2-master',
    sourceUrl: 'https://app.klingai.com/global/dev/document-api',
    apiDocsUrl: 'https://app.klingai.com/global/dev/document-api',
    description: '快手可灵 Kling 2.0 — text/image-to-video.',
    group: 'video',
    tags: ['video', 'chinese'],
    endpoints: 'video',
  },

  {
    model_name: 'eleven_v4',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Expressive speech; gateway bridges Dialogue HTTP.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_v4_turbo',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Realtime speech; gateway bridges Dialogue WebSocket.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_v3',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Expressive speech synthesis.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_v3_conversational',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Conversational speech; gateway bridges Dialogue WebSocket.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_multilingual_v2',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Multilingual long-form speech synthesis.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_flash_v2_5',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Low-latency multilingual speech synthesis.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_flash_v2',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Low-latency English speech synthesis.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_turbo_v2_5',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Legacy multilingual speech; prefer Flash v2.5.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'eleven_turbo_v2',
    sourceUrl: 'https://elevenlabs.io/docs/overview/models',
    apiDocsUrl:
      'https://elevenlabs.io/docs/api-reference/text-to-speech/convert',
    description: 'Legacy English speech; prefer Flash v2.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },

  // ── Audio (TTS / STT) ────────────────────────────────────────────────
  {
    model_name: 'whisper-1',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI Whisper — speech-to-text transcription.',
    group: 'audio',
    tags: ['audio', 'transcription'],
    endpoints: 'audio',
  },
  {
    model_name: 'tts-1',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI TTS standard — text-to-speech.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'tts-1-hd',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI TTS HD — higher-quality TTS.',
    group: 'audio',
    tags: ['audio', 'tts'],
    endpoints: 'audio',
  },
  {
    model_name: 'suno_music',
    sourceUrl: 'https://suno.com/hub',
    apiDocsUrl: 'https://docs.sunoapi.org/',
    description:
      'Suno — music via a third-party proxy API; verify the configured proxy contract.',
    group: 'audio',
    tags: ['audio', 'music'],
    endpoints: 'audio',
  },

  // ── Embedding ────────────────────────────────────────────────────────
  {
    model_name: 'text-embedding-3-large',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI 3072-d embedding — top quality.',
    group: 'embedding',
    tags: ['embedding'],
    endpoints: 'embedding',
  },
  {
    model_name: 'text-embedding-3-small',
    sourceUrl: 'https://platform.openai.com/docs/models',
    apiDocsUrl: 'https://platform.openai.com/docs/api-reference',
    description: 'OpenAI 1536-d embedding — cost-efficient.',
    group: 'embedding',
    tags: ['embedding', 'cheap'],
    endpoints: 'embedding',
  },
]

export const GROUP_LABELS: Record<ModelPresetGroup, string> = {
  chat: 'Chat / Text',
  reasoning: 'Reasoning',
  image: 'Image',
  video: 'Video',
  audio: 'Audio',
  embedding: 'Embedding',
}

export const GROUP_ORDER: ModelPresetGroup[] = [
  'chat',
  'reasoning',
  'image',
  'video',
  'audio',
  'embedding',
]
