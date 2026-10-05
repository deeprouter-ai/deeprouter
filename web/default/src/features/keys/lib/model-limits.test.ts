/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { describe, expect, it } from 'vitest'
import { keyPermitsModel } from './model-limits'

function key(limits: string | null, enabled = true) {
  return { model_limits_enabled: enabled, model_limits: limits }
}

describe('keyPermitsModel', () => {
  it('permits everything when limits are disabled', () => {
    expect(keyPermitsModel(key(null, false), 'MiniMax-H3')).toBe(true)
    expect(keyPermitsModel(key('gpt-4o', false), 'MiniMax-H3')).toBe(true)
  })

  it('matches exact entries', () => {
    expect(keyPermitsModel(key('MiniMax-H3,gpt-4o'), 'MiniMax-H3')).toBe(true)
    expect(keyPermitsModel(key('gpt-4o'), 'MiniMax-H3')).toBe(false)
  })

  it('matches trailing-* entries as prefixes, like the gateway does', () => {
    // The video purpose whitelist (P5) relies on exactly this semantics.
    expect(
      keyPermitsModel(key('doubao-seedance-*'), 'doubao-seedance-2-5-260628')
    ).toBe(true)
    expect(keyPermitsModel(key('doubao-seedance-*'), 'doubao-seedream-4')).toBe(
      false
    )
    expect(keyPermitsModel(key('*'), 'anything-at-all')).toBe(true)
  })

  it('tolerates spaces and empty segments in the stored CSV', () => {
    expect(keyPermitsModel(key(' MiniMax-H3 , ,gpt-4o'), 'MiniMax-H3')).toBe(
      true
    )
    expect(keyPermitsModel(key(''), 'MiniMax-H3')).toBe(false)
  })
})
