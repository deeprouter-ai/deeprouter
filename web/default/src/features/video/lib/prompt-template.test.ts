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

function build(language: PromptLanguage, models = VIDEO_MODELS) {
  return buildVideoPrompt({ scriptUrl: SCRIPT_URL, models, language })
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

  it('credential fetch: uses the UA-independent env form with a failure branch', () => {
    // ?format=env is the fix for the Windows wall of 2026-10-04: the script
    // form renames the variables per shell ($DrApiKey under PowerShell), so
    // an agent fetching with irm held the key under a name it was never told.
    // The env form is identical for every client.
    for (const language of LANGUAGES) {
      const prompt = build(language)
      expect(prompt).toContain(`${SCRIPT_URL}?format=env`)
      // Dead link = DR_ERROR line; the agent must stop and send the user
      // back for a fresh copy, never retry the burned URL.
      expect(prompt).toContain('DR_ERROR')
    }
    expect(build('zh')).toContain('重新复制')
    expect(build('en')).toContain('copy a fresh prompt')
  })

  it('environment guard: refuses to run off the user’s machine', () => {
    // Pasted into a web AI with no terminal (measured on claude.ai threads,
    // 2026-10-04), the old prompt half-executed and died mid-flow; it must
    // bail out with guidance instead.
    expect(build('zh')).toContain('用户的电脑上')
    expect(build('zh')).toContain('网页版 AI')
    expect(build('en')).toContain("user's computer")
    expect(build('en')).toContain('web-based AI')
  })

  it('method layer: teaches submit → poll → download → open/print-path', () => {
    for (const language of LANGUAGES) {
      const prompt = build(language)
      expect(prompt).toContain('/v1/video/generations')
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

  it('test run is opt-in (it costs money) and the prompt says it runs once', () => {
    // The verification clip spends the user's balance, so the agent must ask
    // with the price on the table instead of just generating — and the user
    // must leave knowing this text is pasted once, not before every video.
    const zh = build('zh')
    expect(zh).toContain('可选')
    expect(zh).toContain('先问用户')
    expect(zh).toContain('只需要在最开始粘贴这一次')
    const en = build('en')
    expect(en).toContain('optional')
    expect(en).toContain('ask the user')
    expect(en).toContain('pasted once')
  })

  it('the first permitted model is the default and the test run', () => {
    for (const language of LANGUAGES) {
      for (const model of VIDEO_MODELS) {
        const p = buildVideoPrompt({
          scriptUrl: SCRIPT_URL,
          models: [model],
          language,
        })
        if (language === 'zh') {
          expect(p).toContain(`默认用 ${model.id}`)
        } else {
          expect(p).toContain(`Default model: ${model.id}`)
        }
        expect(p).toContain(model.testRun[language])
      }
    }
  })

  it('the menu lists only the models the key may call', () => {
    // A key is granted exactly the video models its account has enabled
    // (internal/keypurpose). Teaching the AI about a model this key cannot
    // call produces a 403 the moment the user switches to it by voice.
    const [cheapest, mid] = VIDEO_MODELS
    for (const language of LANGUAGES) {
      const limited = buildVideoPrompt({
        scriptUrl: SCRIPT_URL,
        models: [mid],
        language,
      })
      expect(limited).toContain(mid.id)
      expect(limited).not.toContain(cheapest.id)
      expect(limited).toContain(mid.menuLine[language])
    }
  })

  it('defaults to the cheapest permitted model, not a hardcoded one', () => {
    // VIDEO_MODELS is ordered cheapest-first and the caller passes the
    // permitted subset, so models[0] is the cheapest this key can run.
    expect(DEFAULT_VIDEO_MODEL.id).toBe('MiniMax-H3')
    const withoutH3 = VIDEO_MODELS.filter((m) => m.id !== 'MiniMax-H3')
    const p = buildVideoPrompt({
      scriptUrl: SCRIPT_URL,
      models: withoutH3,
      language: 'zh',
    })
    expect(p).toContain(`默认用 ${withoutH3[0].id}`)
    expect(p).not.toContain('MiniMax-H3')
  })

  it('lists every model inside the prompt when all are permitted', () => {
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

  it('every model option carries a test-run and menu line in both languages', () => {
    for (const model of VIDEO_MODELS) {
      expect(model.testRun.zh.length).toBeGreaterThan(0)
      expect(model.testRun.en.length).toBeGreaterThan(0)
      // The menu line names the model, so the AI can switch to it by id.
      expect(model.menuLine.zh).toContain(model.id)
      expect(model.menuLine.en).toContain(model.id)
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
