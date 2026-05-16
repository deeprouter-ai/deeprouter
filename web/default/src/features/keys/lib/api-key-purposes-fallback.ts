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
import type {
  ApiKeyPurposesResponse,
  PriceTierSummary,
  PurposeSummary,
} from '../types'

// Hardcoded mirror of setting/alias_setting/seed/aliases.yaml. Used by the
// Create API Key drawer when the backend hasn't shipped the new endpoint
// yet (binary needs rebuild for /api/user/self/api-key-purposes to exist).
// Once the backend is live this falls through; in production the YAML is
// the authoritative source.

const ZH = (() => {
  if (typeof navigator === 'undefined') return false
  return navigator.language?.toLowerCase().startsWith('zh')
})()

function pick<T>(en: T, zh: T): T {
  return ZH ? zh : en
}

export const FALLBACK_PURPOSES: PurposeSummary[] = [
  {
    id: 'chat',
    label: pick('Chat / Writing', '聊天 / 写作'),
    icon: '💬',
    desc: pick('Translation, creation, dialogue', '翻译、创作、对话'),
    human_estimate: pick('≈ ¥1 for 100 chats', '约 ¥1 聊 100 句'),
    price_range: '¥0.01 – 0.10 / 1K tokens',
    recommended_brand: 'claude',
    available_brands: ['claude', 'openai', 'gemini', 'deepseek'],
  },
  {
    id: 'coding',
    label: pick('Coding', '编程 / Coding'),
    icon: '💻',
    desc: pick(
      'Code completion, AI pair programming, refactor',
      '代码补全、AI 编程、重构'
    ),
    human_estimate: pick('≈ ¥1 for 50 code edits', '约 ¥1 改 50 段代码'),
    price_range: '¥0.02 – 0.15 / 1K tokens',
    recommended_brand: 'claude',
    available_brands: ['claude', 'openai', 'deepseek'],
  },
  {
    id: 'image',
    label: pick('Image generation', '图像生成'),
    icon: '🎨',
    desc: pick('Text-to-image, image edit, image variation', '文生图、改图、变体'),
    human_estimate: pick('≈ ¥10 for 20 images', '约 ¥10 生成 20 张'),
    price_range: '¥0.3 – 1.0 / image',
    recommended_brand: 'openai',
    available_brands: ['openai'],
  },
  {
    id: 'video',
    label: pick('Video generation', '视频生成'),
    icon: '🎬',
    desc: pick('Text-to-video, video edit', '文生视频、改视频'),
    human_estimate: pick('≈ ¥50 for 10 short clips', '约 ¥50 生成 10 段短视频'),
    price_range: '¥3 – 10 / clip',
    recommended_brand: 'openai',
    available_brands: [],
  },
  {
    id: 'voice',
    label: pick('Voice / TTS / Transcription', '语音 / TTS / 转写'),
    icon: '🎙️',
    desc: pick('Transcription, voice cloning, text-to-speech', '转写、配音、克隆'),
    human_estimate: pick('≈ ¥10 for 200 minutes', '约 ¥10 转写 200 分钟'),
    price_range: '¥0.05 / minute',
    recommended_brand: 'openai',
    available_brands: [],
  },
  {
    id: 'all',
    label: pick('Everything (Auto)', '全部 (Auto)'),
    icon: '⚡',
    desc: pick(
      'Auto-route by task. Set a price cap below.',
      '按任务自动路由，可设置价格上限'
    ),
    human_estimate: pick('Billed per actual model', '按实际使用模型计费'),
    price_range: 'Variable',
    recommended_brand: '',
    available_brands: ['claude', 'openai', 'gemini', 'deepseek'],
  },
]

export const FALLBACK_PRICE_TIERS: PriceTierSummary[] = [
  {
    id: 'economy',
    label: pick('Economy', '经济档'),
    desc: pick(
      'Cheap & fast only, never Opus / o1 / Ultra',
      '只走便宜模型，绝不上 Opus/o1'
    ),
    price_range: '¥0.001 – 0.02 / 1K',
    is_default: false,
    requires_confirm: false,
  },
  {
    id: 'standard',
    label: pick('Standard', '标准档'),
    desc: pick(
      'Default. Covers most tasks, avoids ultra-premium models.',
      '默认，覆盖大部分场景，避免顶配'
    ),
    price_range: '¥0.001 – 0.10 / 1K',
    is_default: true,
    requires_confirm: false,
  },
  {
    id: 'premium',
    label: pick('Premium', '高级档'),
    desc: pick(
      'Includes Claude Opus and GPT-4 family',
      '含 Claude Opus、GPT-4 系列'
    ),
    price_range: '¥0.001 – 0.30 / 1K',
    is_default: false,
    requires_confirm: false,
  },
  {
    id: 'ultra',
    label: pick('Ultra', '顶配档'),
    desc: pick(
      'No cap. Includes o1 / Opus / Gemini Ultra. Confirm required.',
      '无上限，含 o1 / Opus / Gemini Ultra，需确认'
    ),
    price_range: 'Uncapped',
    is_default: false,
    requires_confirm: true,
  },
]

export const FALLBACK_API_KEY_PURPOSES: ApiKeyPurposesResponse = {
  purposes: FALLBACK_PURPOSES,
  price_tiers: FALLBACK_PRICE_TIERS,
  default_price_tier: 'standard',
}
