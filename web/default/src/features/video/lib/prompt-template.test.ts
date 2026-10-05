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
import { describe, expect, it } from 'vitest'
import { MODEL_PRESETS } from '@/features/models/lib/model-presets'
import {
  buildVideoPrompt,
  DEFAULT_VIDEO_MODEL,
  VIDEO_MODELS,
  type PromptLanguage,
} from './prompt-template'

const SCRIPT_URL = 'https://deeprouter.example/i/tok_abc123'
const LANGUAGES: PromptLanguage[] = ['zh', 'en']

function build(language: PromptLanguage, model = DEFAULT_VIDEO_MODEL) {
  return buildVideoPrompt({ scriptUrl: SCRIPT_URL, model, language })
}

describe('buildVideoPrompt (both languages)', () => {
  it('credential layer: embeds the one-time URL, never a key', () => {
    for (const language of LANGUAGES) {
      const prompt = build(language)
      // Exactly once — the agent must not be offered two places to fetch from.
      expect(prompt.split(SCRIPT_URL).length - 1).toBe(1)
      // The variables the agent reads out of the redeemed script text are the
      // ones internal/connect injects (posixValues): renaming either side
      // silently breaks every prompt already printed in tutorials.
      expect(prompt).toContain('DR_BASE_URL')
      expect(prompt).toContain('DR_API_KEY')
      // AC (Q10 token branch): no plaintext key in page or clipboard.
      expect(prompt).not.toMatch(/sk-[A-Za-z0-9]/)
      // The key lands in .env, which must be gitignored.
      expect(prompt).toContain('DEEPROUTER_API_KEY')
      expect(prompt).toContain('.gitignore')
    }
  })

  it('method layer: teaches submit → poll → download → open/print-path', () => {
    for (const language of LANGUAGES) {
      const prompt = build(language)
      expect(prompt).toContain('/v1/video/generations')
      // Discover before calling, and know where the full reference lives.
      expect(prompt).toContain('/v1/models')
      expect(prompt).toContain('supported_endpoint_types')
      expect(prompt).toContain('https://deeprouter.co/llms.txt')
      expect(prompt).toContain('/v1/videos/')
      // Poll terminal states must match dto.VideoStatus* on the gateway.
      expect(prompt).toContain('"completed"')
      expect(prompt).toContain('"failed"')
      // Player fallback: open on each OS, and always print the absolute path.
      expect(prompt).toContain('start')
      expect(prompt).toContain('xdg-open')
    }
    expect(build('zh')).toContain('绝对路径')
    expect(build('en')).toContain('absolute path')
  })

  it('memory layer: writes rules into both tools’ memory files', () => {
    for (const language of LANGUAGES) {
      const prompt = build(language)
      expect(prompt).toContain('CLAUDE.md')
      expect(prompt).toContain('AGENTS.md')
    }
    expect(build('zh')).toContain('生成视频')
    expect(build('en')).toContain('generate a video')
    // The rules teach their own removal, so "delete the video setup" works in
    // a fresh session without guessing what was written where.
    expect(build('zh')).toContain('若用户要求移除视频配置')
    expect(build('en')).toContain('asks to remove the video setup')
  })

  it('uses the chosen model for the default and the test run', () => {
    for (const language of LANGUAGES) {
      for (const model of VIDEO_MODELS) {
        const p = buildVideoPrompt({ scriptUrl: SCRIPT_URL, model, language })
        if (language === 'zh') {
          expect(p).toContain(`默认用 ${model.id}`)
        } else {
          expect(p).toContain(`Default model: ${model.id}`)
        }
        expect(p).toContain(model.testRun[language])
      }
    }
  })

  it('lists every menu model inside the prompt with a price', () => {
    // The in-prompt alternatives table and the page's model menu must not
    // drift apart: an agent asked to switch models should only pick ones the
    // page also prices.
    for (const language of LANGUAGES) {
      const prompt = build(language)
      for (const model of VIDEO_MODELS) {
        expect(prompt).toContain(model.id)
      }
    }
  })

  it('every model option carries a test-run line in both languages', () => {
    for (const model of VIDEO_MODELS) {
      expect(model.testRun.zh.length).toBeGreaterThan(0)
      expect(model.testRun.en.length).toBeGreaterThan(0)
    }
  })

  it('every video-page model has a Quick Import metadata preset', () => {
    // The admin Models page backfills metadata cards via Quick Import; a
    // model the page sells but the preset table lacks means the operator
    // cannot one-click its card (Video First Wave AC-F — MiniMax-H3 and
    // seedance 2.5 were missing on 2026-10-04).
    for (const model of VIDEO_MODELS) {
      expect(
        MODEL_PRESETS.some((preset) => preset.model_name === model.id),
        `${model.id} missing from MODEL_PRESETS`
      ).toBe(true)
    }
  })
})
