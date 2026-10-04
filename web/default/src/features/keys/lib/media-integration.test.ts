// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { modelNameForPurpose } from './integration'
import { mediaEndpoint, mediaExample } from './media-integration'

describe('media integration', () => {
  it('never supplies a chat alias for a media purpose', () => {
    for (const purpose of ['video', 'image', 'voice'])
      expect(modelNameForPurpose(purpose)).toBe('')
    expect(modelNameForPurpose('coding')).toBe('deeprouter-auto')
  })
  it('uses a concrete image model and a speech output file', () => {
    expect(
      mediaEndpoint('image', {
        id: 'new-image',
        supported_endpoint_types: ['openai'],
      })
    ).toBeNull()
    expect(
      mediaExample(
        'https://example.test/v1',
        '/images/generations',
        'new-image'
      )
    ).toContain('"model":"new-image"')
    expect(
      mediaExample('https://example.test/v1', '/audio/speech', 'eleven_fixture')
    ).toContain('--output speech.mp3')
    for (const endpoint of [
      '/images/generations',
      '/audio/speech',
      '/videos',
      '/audio/transcriptions',
    ]) {
      expect(
        mediaExample('https://example.test/v1', endpoint, 'fixture')
      ).not.toMatch(/\n\+ /)
    }
  })
})
