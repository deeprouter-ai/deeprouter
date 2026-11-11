/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '../../types'
import { ApiKeysProvider, useApiKeys } from '../api-keys-provider'

const mockGetApiKeys = vi.hoisted(() => vi.fn())

vi.mock('../../api', () => ({
  getApiKeys: mockGetApiKeys,
  fetchTokenKey: vi.fn(),
  fetchTokenKeysBatch: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

/** Builds an enabled key with only the fields under test overridden. */
function key(
  overrides: Partial<ApiKey> & { id: number; name: string }
): ApiKey {
  return {
    key: '',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 0,
    accessed_time: 0,
    group: '',
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    simple_purpose: '',
    simple_brand: '',
    simple_price_tier: '',
    rpm_limit: 0,
    tpm_limit: 0,
    monthly_limit: 0,
    ...overrides,
  } as ApiKey
}

/** Renders what the setup card would read from the provider. */
function Probe() {
  const { setupKeys, setupKey } = useApiKeys()
  return (
    <>
      <p data-testid='setup-key'>{setupKey?.name ?? 'none'}</p>
      <p data-testid='candidates'>{setupKeys.map((k) => k.name).join(',')}</p>
    </>
  )
}

function renderProvider(items: ApiKey[]) {
  mockGetApiKeys.mockResolvedValue({
    success: true,
    data: { items, total: items.length, page: 1, page_size: 50 },
  })
  render(
    <ApiKeysProvider>
      <Probe />
    </ApiKeysProvider>
  )
}

describe('ApiKeysProvider — which key the one-click setup card configures', () => {
  beforeEach(() => vi.clearAllMocks())

  it('skips a newest media key and defaults to the next usable one', async () => {
    // The measured regression (2026-10-05): the video page's one-click create
    // made a video key the newest, the card hid itself because its default
    // pick was a media key, and the picker that could switch away went with it.
    renderProvider([
      key({ id: 3, name: 'my-video-key', simple_purpose: 'video' }),
      key({ id: 2, name: 'my-coding-key', simple_purpose: 'coding' }),
      key({ id: 1, name: 'older-chat-key' }),
    ])

    await waitFor(() =>
      expect(screen.getByTestId('setup-key')).toHaveTextContent('my-coding-key')
    )
    expect(screen.getByTestId('candidates')).toHaveTextContent(
      'my-coding-key,older-chat-key'
    )
    expect(screen.getByTestId('candidates')).not.toHaveTextContent('my-video-key')
  })

  it('offers no candidate when every key is a media key', async () => {
    // Nothing a coding tool can use: the card hides, same as having no key.
    renderProvider([
      key({ id: 3, name: 'video', simple_purpose: 'video' }),
      key({ id: 2, name: 'image', simple_purpose: 'image' }),
      key({ id: 1, name: 'voice', simple_purpose: 'voice' }),
    ])

    await waitFor(() => expect(mockGetApiKeys).toHaveBeenCalled())
    await waitFor(() =>
      expect(screen.getByTestId('setup-key')).toHaveTextContent('none')
    )
    expect(screen.getByTestId('candidates')).toBeEmptyDOMElement()
  })

  it('still skips disabled keys when picking the default', async () => {
    // The pre-existing rule must survive the filter: a disabled key configures
    // a tool that then answers 401 days later.
    renderProvider([
      key({ id: 2, name: 'disabled-chat', status: 2 }),
      key({ id: 1, name: 'live-chat' }),
    ])

    await waitFor(() =>
      expect(screen.getByTestId('setup-key')).toHaveTextContent('live-chat')
    )
  })
})
