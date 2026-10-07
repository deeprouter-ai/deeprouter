// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { consoleModeFor, homePathFor, readPersona } from './mode'

describe('consoleModeFor', () => {
  it('puts dev and team personas in Advanced', () => {
    expect(consoleModeFor({ setting: { persona: 'dev' } })).toBe('advanced')
    expect(consoleModeFor({ setting: '{"persona":"team"}' })).toBe('advanced')
  })

  it('defaults everyone else to Simple', () => {
    expect(consoleModeFor({ setting: { persona: 'casual' } })).toBe('simple')
    expect(consoleModeFor({ setting: '{"persona":"unset"}' })).toBe('simple')
    expect(consoleModeFor({ setting: undefined })).toBe('simple')
    expect(consoleModeFor({ setting: 'not json' })).toBe('simple')
    expect(consoleModeFor(null)).toBe('simple')
    expect(consoleModeFor({ setting: '{"persona":"unset"}', org_id: 0 })).toBe(
      'simple'
    )
  })

  // Enterprise Org D42: the members of an organization work in the Advanced
  // console until they choose otherwise.
  it('defaults a member of an organization who has not chosen to Advanced', () => {
    expect(consoleModeFor({ setting: '{"persona":"unset"}', org_id: 5 })).toBe(
      'advanced'
    )
    expect(consoleModeFor({ setting: undefined, org_id: 5 })).toBe('advanced')
    expect(consoleModeFor({ setting: 'not json', org_id: 5 })).toBe('advanced')
  })

  it('leaves a member who chose Simple in Simple', () => {
    expect(consoleModeFor({ setting: { persona: 'casual' }, org_id: 5 })).toBe(
      'simple'
    )
  })
})

describe('homePathFor', () => {
  it('sends each mode to its own home', () => {
    expect(homePathFor({ setting: { persona: 'casual' } })).toBe('/simple')
    expect(homePathFor({ setting: { persona: 'dev' } })).toBe('/dashboard')
  })

  it('sends a member of an organization who has not chosen to the Advanced home', () => {
    expect(homePathFor({ setting: { persona: 'unset' }, org_id: 5 })).toBe(
      '/dashboard'
    )
    expect(homePathFor({ setting: { persona: 'unset' } })).toBe('/simple')
  })
})

describe('readPersona', () => {
  it('reads objects and JSON strings, ignores junk', () => {
    expect(readPersona({ persona: 'dev' })).toBe('dev')
    expect(readPersona('{"persona":"casual"}')).toBe('casual')
    expect(readPersona('[]')).toBeUndefined()
    expect(readPersona(42)).toBeUndefined()
  })
})
