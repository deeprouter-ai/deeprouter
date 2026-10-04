/*
Provider presets for the "Quick Import" dialog. Each entry maps to a single
channel that will be created in disabled state (status=2) with a placeholder
key, so the operator can enable + fill the real key without typing the rest
of the form every time.

Design goals (kept deliberately beginner-proof):
  - One preset = one provider + ONE modality (chat / image / embedding-audio).
    Mixing image + chat in one channel is the #1 source of confusion: image
    models can't be tested with a chat request and behave differently.
  - Every model listed here is an EXACT key in setting/ratio_setting (chat:
    defaultModelRatio, image: defaultModelPrice) so a fresh import never throws
    "price not configured". Input pricing has NO prefix/alias fallback, so we
    use exact, dated model names where the provider has no stable alias
    (e.g. claude-sonnet-4-5-20250929, not claude-sonnet-4-6).
  - `testModel` is a cheap, real model of the right modality so the channel
    "Test" button works out of the box.

Moonshot/Kimi, Doubao/VolcEngine and Mistral ship with BOOTSTRAP default
prices (see model_ratio.go) — approximate list prices so import doesn't error;
run a models.dev price sync to get exact rates before charging customers.

Still omitted (no clean default price / aggregator): Groq, OpenRouter. Add
those manually + sync prices if you need them.

`type` corresponds to constant/channel.go ChannelType* constants on the Go
backend. Operators can always expand a channel's model list afterwards via the
edit form or "detect upstream models".
*/

export type ProviderModality = 'chat' | 'video' | 'image' | 'embedding'

export type ProviderPreset = {
  id: string
  name: string
  type: number
  modality: ProviderModality
  models: string
  /** Cheap real model used by the channel "Test" button. */
  testModel?: string
  baseUrl?: string
  docsUrl?: string
  description: string
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  // ── Chat ────────────────────────────────────────────────────────────────
  {
    id: 'openai-chat',
    name: 'OpenAI · 对话',
    type: 1,
    modality: 'chat',
    models:
      'gpt-6.1-sol,gpt-6-astra,gpt-6-sol,gpt-6-luna,gpt-5.6,gpt-5.6-terra,gpt-5.6-luna,gpt-4o-mini',
    testModel: 'gpt-5.6-luna',
    docsUrl: 'https://platform.openai.com/api-keys',
    description: '对话 / GPT-6.1 Sol · GPT-6 Astra · GPT-5.6 Luna（最快）',
  },
  {
    id: 'anthropic',
    name: 'Anthropic Claude · 对话',
    type: 14,
    modality: 'chat',
    models:
      'claude-opus-5-5,claude-sonnet-5-5,claude-fable-5-1,claude-opus-5,claude-sonnet-5,claude-fable-5,claude-opus-4-8,claude-sonnet-4-6,claude-haiku-4-5-20251001',
    testModel: 'claude-haiku-4-5-20251001',
    docsUrl: 'https://console.anthropic.com/settings/keys',
    description: '对话 / Opus 5.5 · Sonnet 5.5 · Fable 5.1 · Haiku 4.5',
  },
  {
    id: 'gemini',
    name: 'Google Gemini · 对话',
    type: 24,
    modality: 'chat',
    models:
      'gemini-3.8-flash,gemini-3.7-flash,gemini-3.6-flash,gemini-3.1-pro-preview,gemini-3.1-flash-lite,gemini-2.5-pro,gemini-2.5-flash',
    testModel: 'gemini-3.7-flash',
    docsUrl: 'https://aistudio.google.com/apikey',
    description: '对话 / Gemini 3.8 Flash · 3.1 Pro · 2.5 Pro',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek · 对话',
    type: 43,
    modality: 'chat',
    // deepseek-chat / -reasoner (2026-07-24) and deepseek-v4-flash were retired.
    models: 'deepseek-flash,deepseek-v4-pro',
    testModel: 'deepseek-flash',
    docsUrl: 'https://platform.deepseek.com/api_keys',
    description: '对话 / DeepSeek V4.1 Flash · V4 Pro',
  },
  {
    id: 'minimax-chat',
    name: 'MiniMax · 对话',
    // The official OpenAI-compatible API uses /v1/chat/completions; type 35
    // still uses MiniMax's legacy /v1/text/chatcompletion_v2 endpoint.
    type: 1,
    modality: 'chat',
    models: 'MiniMax-M3,MiniMax-M2.7,MiniMax-M2',
    testModel: 'MiniMax-M2',
    baseUrl: 'https://api.minimax.io',
    docsUrl:
      'https://platform.minimax.io/user-center/basic-information/interface-key',
    description: '对话 / MiniMax-M3 · M2.7 · M2（国际站，OpenAI 兼容）',
  },

  {
    id: 'qwen',
    name: 'Qwen 通义千问 · 对话',
    type: 17,
    modality: 'chat',
    models:
      'qwen3.8-max,qwen3.8-flash,qwen3.7-max,qwen3.7-plus,qwen3.7-flash,qwen3-coder-plus,qwen-plus,qwen-flash',
    testModel: 'qwen3.7-flash',
    docsUrl: 'https://bailian.console.aliyun.com/?apiKey=1',
    description: '对话 / Qwen 3.8 Max · 3.7 Plus / Flash · Coder（阿里）',
  },
  {
    id: 'zhipu-glm',
    name: '智谱 GLM · 对话',
    type: 26,
    modality: 'chat',
    models:
      'glm-5.3,glm-5.3-flash,glm-5.3-flashx,glm-5.2,glm-5.1,glm-5,glm-4.7,glm-4.7-flash',
    testModel: 'glm-4.7-flash',
    docsUrl: 'https://open.bigmodel.cn/usercenter/apikeys',
    description: '对话 / GLM-5.3 · GLM-5 · GLM-4.7 Flash（智谱）',
  },
  {
    id: 'xai-grok',
    name: 'xAI Grok · 对话',
    type: 48,
    modality: 'chat',
    models: 'grok-4.7,grok-4.6,grok-4.5,grok-4.3,grok-4.20,grok-build-0.1',
    testModel: 'grok-4.6',
    docsUrl: 'https://console.x.ai',
    description: '对话 / Grok 4.7 · 4.6 · Grok Build（xAI）',
  },
  {
    id: 'moonshot',
    name: 'Moonshot Kimi · 对话',
    type: 25,
    modality: 'chat',
    models:
      'kimi-k3,kimi-k2.7-code,kimi-k2.7-code-highspeed,kimi-k2.6,kimi-k2.5,moonshot-v1-128k',
    testModel: 'kimi-k2.6',
    docsUrl: 'https://platform.moonshot.cn/console/api-keys',
    description: '对话 / Kimi K3 · K2.7 Code · K2.6（价格为估值，请同步核对）',
  },
  {
    id: 'doubao',
    name: 'Doubao 豆包 · 对话',
    type: 45,
    modality: 'chat',
    models:
      'doubao-seed-2-1-pro-260628,doubao-seed-2-1-turbo-260628,doubao-seed-2.0-pro,doubao-seed-2.0-lite,doubao-seed-2.0-mini,doubao-pro-32k,doubao-pro-128k',
    testModel: 'doubao-seed-2.0-mini',
    docsUrl: 'https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey',
    description:
      '对话 / 豆包 Seed 2.1 Pro · Turbo · 2.0 Mini（价格为估值，请同步核对）',
  },
  {
    id: 'mistral',
    name: 'Mistral AI · 对话',
    type: 42,
    modality: 'chat',
    models:
      'mistral-medium-2604,mistral-small-2603,mistral-large-2512,codestral-2508,mistral-medium-latest,mistral-small-latest',
    testModel: 'mistral-small-latest',
    docsUrl: 'https://console.mistral.ai/api-keys',
    description: '对话 / Medium 3.5 · Small 4 · Large 3 · Codestral',
  },
  {
    id: 'minimax-video',
    name: 'MiniMax · 海螺视频',
    type: 35,
    modality: 'video',
    models:
      'MiniMax-H3,MiniMax-Hailuo-2.3,MiniMax-Hailuo-2.3-Fast,MiniMax-Hailuo-02',
    testModel: 'MiniMax-H3',
    baseUrl: 'https://api.minimax.io',
    docsUrl:
      'https://platform.minimax.io/user-center/basic-information/interface-key',
    description:
      '视频 / H3 · 海螺 2.3 / Fast / 02（国际站；旧型号价格为估值，请核对）',
  },
  // ── Image ───────────────────────────────────────────────────────────────
  {
    id: 'minimax-image',
    name: 'MiniMax · 画图',
    type: 35,
    modality: 'image',
    models: 'image-01',
    testModel: 'image-01',
    baseUrl: 'https://api.minimax.io',
    docsUrl:
      'https://platform.minimax.io/user-center/basic-information/interface-key',
    description: '画图 / image-01（国际站）',
  },
  {
    id: 'openai-image',
    name: 'OpenAI · 画图',
    type: 1,
    modality: 'image',
    // dall-e-* was retired by OpenAI on 2026-05-12. gpt-image-2.5-flare
    // (2026-09-08) is the recommended default; all four are priced.
    models:
      'gpt-image-2.5-flare,gpt-image-2.5-sunburst,gpt-image-2,gpt-image-1',
    testModel: 'gpt-image-2',
    docsUrl: 'https://platform.openai.com/api-keys',
    description:
      '画图 / gpt-image-2.5 flare · sunburst · gpt-image-2（走 /v1/images/generations）',
  },
  // ── Embeddings & Audio ────────────────────────────────────────────────────
  {
    id: 'minimax-audio',
    name: 'MiniMax · 语音合成',
    type: 35,
    modality: 'embedding',
    models:
      'speech-2.8-hd,speech-2.8-turbo,speech-2.6-hd,speech-2.6-turbo,speech-02-hd,speech-02-turbo',
    testModel: 'speech-2.8-turbo',
    baseUrl: 'https://api.minimax.io',
    docsUrl:
      'https://platform.minimax.io/user-center/basic-information/interface-key',
    description: '语音 / Speech 2.8 · 2.6 · 02，HD / Turbo（国际站）',
  },
  {
    id: 'openai-embed',
    name: 'OpenAI · 向量 / 语音',
    type: 1,
    modality: 'embedding',
    models: 'text-embedding-3-small,text-embedding-3-large,whisper-1,tts-1',
    testModel: 'text-embedding-3-small',
    docsUrl: 'https://platform.openai.com/api-keys',
    description: '向量 / 语音 / embeddings · whisper · tts',
  },
  {
    id: 'elevenlabs',
    name: 'ElevenLabs · 语音合成',
    type: 58,
    modality: 'embedding',
    models:
      'eleven_v4,eleven_v4_turbo,eleven_v3,eleven_v3_conversational,eleven_multilingual_v2,eleven_turbo_v2_5,eleven_flash_v2_5,eleven_flash_v2,eleven_turbo_v2',
    testModel: 'eleven_flash_v2_5',
    docsUrl: 'https://elevenlabs.io/app/settings/api-keys',
    description:
      '语音 / TTS（走 /v1/audio/speech，voice=voice_id；价格为估值，请核对）',
  },
]
