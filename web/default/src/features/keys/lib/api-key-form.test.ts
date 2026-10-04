import { describe, expect, it } from 'vitest'
import type { ApiKey } from '../types'
import { detectAdvancedMode } from './api-key-form'

const key = (fields: Partial<ApiKey>) => fields as ApiKey

describe('Simple key editing mode', () => {
  it('keeps automatically derived purpose grants in Simple mode', () => {
    for (const simple_purpose of [
      'chat',
      'coding',
      'video',
      'image',
      'voice',
      'all',
    ] as const) {
      expect(
        detectAdvancedMode(
          key({
            simple_purpose,
            model_limits_enabled: true,
            model_limits: 'fixture-model',
            group: '',
          }),
          ''
        )
      ).toBe(false)
    }
  })
  it('preserves manual model grants and other advanced restrictions', () => {
    expect(
      detectAdvancedMode(
        key({ model_limits_enabled: true, model_limits: 'fixture-model' }),
        ''
      )
    ).toBe(true)
    for (const fields of [
      { allow_ips: '127.0.0.1' },
      { rpm_limit: 5 },
      { group: 'other' },
    ]) {
      expect(
        detectAdvancedMode(key({ simple_purpose: 'video', ...fields }), '')
      ).toBe(true)
    }
  })
})
