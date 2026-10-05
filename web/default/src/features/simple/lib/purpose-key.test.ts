// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/features/keys/types'

const getApiKeys = vi.fn()
const createApiKey = vi.fn()
vi.mock('@/features/keys/api', () => ({
  getApiKeys: (...args: unknown[]) => getApiKeys(...args),
  createApiKey: (...args: unknown[]) => createApiKey(...args),
}))

const { ensurePurposeKey, pickPurposeKey } = await import('./purpose-key')

const key = (over: Partial<ApiKey>): ApiKey =>
  ({ id: 1, name: 'k', status: 1, simple_purpose: '', ...over }) as ApiKey

describe('pickPurposeKey', () => {
  it('takes the newest enabled key of that purpose', () => {
    const keys = [
      key({ id: 3, simple_purpose: 'video', status: 2 }),
      key({ id: 2, simple_purpose: 'video' }),
      key({ id: 1, simple_purpose: 'video' }),
    ]
    expect(pickPurposeKey(keys, 'video')?.id).toBe(2)
    expect(pickPurposeKey(keys, 'image')).toBeNull()
  })
})

describe('ensurePurposeKey', () => {
  beforeEach(() => {
    getApiKeys.mockReset()
    createApiKey.mockReset()
  })

  it('reuses an existing key without creating one', async () => {
    getApiKeys.mockResolvedValue({
      data: { items: [key({ id: 7, simple_purpose: 'image' })] },
    })
    expect((await ensurePurposeKey('image', false)).id).toBe(7)
    expect(createApiKey).not.toHaveBeenCalled()
  })

  it('creates a Simple key for the purpose the first time', async () => {
    getApiKeys.mockResolvedValue({ data: { items: [] } })
    createApiKey.mockResolvedValue({
      success: true,
      data: key({ id: 9, simple_purpose: 'voice' }),
    })
    expect((await ensurePurposeKey('voice', true)).id).toBe(9)
    const payload = createApiKey.mock.calls[0][0]
    expect(payload.simple_purpose).toBe('voice')
    expect(payload.name).toBe('my-voice-key')
    expect(payload.model_limits).toBe('')
    expect(payload.group).toBe('auto')
  })

  it('surfaces the server message when creation is refused', async () => {
    getApiKeys.mockResolvedValue({ data: { items: [] } })
    createApiKey.mockResolvedValue({
      success: false,
      message: 'no video models available',
    })
    await expect(ensurePurposeKey('video', false)).rejects.toThrow(
      'no video models available'
    )
  })

  it('reads the key back when create returns no row', async () => {
    getApiKeys
      .mockResolvedValueOnce({ data: { items: [] } })
      .mockResolvedValueOnce({
        data: { items: [key({ id: 11, simple_purpose: 'chat' })] },
      })
    createApiKey.mockResolvedValue({ success: true })
    expect((await ensurePurposeKey('chat', false)).id).toBe(11)
  })
})
