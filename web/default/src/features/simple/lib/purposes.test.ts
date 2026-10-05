// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { SIMPLE_PURPOSE_IDS } from '@/features/keys/lib/api-key-form'
import { clipsAffordable } from './balance'
import { findPurpose, purposeFromKeyName, SIMPLE_PURPOSES } from './purposes'

describe('Simple purposes', () => {
  it('only uses purpose ids the key form (and backend) understands', () => {
    for (const p of SIMPLE_PURPOSES) {
      expect(SIMPLE_PURPOSE_IDS).toContain(p.id)
    }
  })

  it('reads the purpose back from an auto-named Simple key', () => {
    expect(purposeFromKeyName('my-video-key')).toBe('video')
    expect(purposeFromKeyName('my-coding-key')).toBe('coding')
    expect(purposeFromKeyName('my-all-key')).toBeUndefined()
    expect(purposeFromKeyName('prod key')).toBeUndefined()
    expect(purposeFromKeyName(undefined)).toBeUndefined()
  })

  it('finds known purposes only', () => {
    expect(findPurpose('image')?.id).toBe('image')
    expect(findPurpose('nope')).toBeUndefined()
  })
})

describe('clipsAffordable', () => {
  it('counts whole reference clips', () => {
    // $10 at 500000 quota per USD, $0.48 per clip → 20 clips
    expect(clipsAffordable(5_000_000, 500_000)).toBe(20)
  })

  it('returns null when not even one clip is covered', () => {
    expect(clipsAffordable(0, 500_000)).toBeNull()
    expect(clipsAffordable(100_000, 500_000)).toBeNull()
    expect(clipsAffordable(undefined, 500_000)).toBeNull()
  })
})
