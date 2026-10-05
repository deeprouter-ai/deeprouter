// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import {
  buildVideoPrompt,
  DEFAULT_VIDEO_MODEL,
} from '@/features/video/lib/prompt-template'
import { buildPurposePrompt } from './prompt'
import { SIMPLE_PURPOSES } from './purposes'

const URL = 'https://deeprouter.co/i/one-time-token'

describe('buildPurposePrompt', () => {
  it('reuses the video paste-prompt verbatim for video', () => {
    for (const language of ['zh', 'en'] as const) {
      expect(
        buildPurposePrompt({ purpose: 'video', scriptUrl: URL, language })
      ).toBe(
        buildVideoPrompt({
          scriptUrl: URL,
          model: DEFAULT_VIDEO_MODEL,
          language,
        })
      )
    }
  })

  it.each(['image', 'voice', 'chat', 'coding'] as const)(
    '%s: carries the one-time URL, discovery first, the guide index',
    (purpose) => {
      for (const language of ['zh', 'en'] as const) {
        const text = buildPurposePrompt({ purpose, scriptUrl: URL, language })
        expect(text).toContain(URL)
        expect(text).toContain('/v1/models')
        expect(text).toContain('supported_endpoint_types')
        expect(text).toContain('https://deeprouter.co/llms.txt')
        expect(text).toContain('DEEPROUTER_API_KEY')
        // the credential travels as a token URL, never as a literal key
        expect(text).not.toMatch(/sk-[A-Za-z0-9]{8,}/)
      }
    }
  )

  it('points each media purpose at its own endpoint', () => {
    const en = (purpose: 'image' | 'voice') =>
      buildPurposePrompt({ purpose, scriptUrl: URL, language: 'en' })
    expect(en('image')).toContain('/v1/images/generations')
    expect(en('voice')).toContain('/v1/audio/speech')
    expect(en('voice')).not.toContain('/v1/images/generations')
  })

  it('video uses the models the key actually holds', () => {
    const text = buildPurposePrompt({
      purpose: 'video',
      scriptUrl: URL,
      language: 'en',
      apiKey: {
        model_limits_enabled: true,
        model_limits: 'doubao-seedance-2-0-260128',
      },
    })
    expect(text).toContain('Default model: doubao-seedance-2-0-260128')
    expect(text).not.toContain('MiniMax-H3')
  })

  it('writes a prompt for every purpose on the home grid', () => {
    for (const p of SIMPLE_PURPOSES) {
      expect(
        buildPurposePrompt({ purpose: p.id, scriptUrl: URL, language: 'zh' })
      ).toContain(URL)
    }
  })
})
