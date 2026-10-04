/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import type { ReactNode } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/features/keys/types'
import { VideoPage } from '../index'

const mockGetApiKeys = vi.hoisted(() => vi.fn())
const mockIssueConnectToken = vi.hoisted(() => vi.fn())
const mockCreateApiKey = vi.hoisted(() => vi.fn())

vi.mock('@/features/keys/api', () => ({
  getApiKeys: mockGetApiKeys,
  issueConnectToken: mockIssueConnectToken,
  createApiKey: mockCreateApiKey,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { default_use_auto_group: false } }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: 'en' } }),
}))

vi.mock('@/components/layout', () => {
  const Layout = ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  )
  Layout.Title = ({ children }: { children: ReactNode }) => <h1>{children}</h1>
  Layout.Description = ({ children }: { children: ReactNode }) => (
    <p>{children}</p>
  )
  Layout.Content = ({ children }: { children: ReactNode }) => <>{children}</>
  return { SectionPageLayout: Layout }
})

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => (
    <a href='/keys'>{children}</a>
  ),
}))

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

function keysResponse(items: ApiKey[]) {
  return {
    success: true,
    data: { items, total: items.length, page: 1, page_size: 100 },
  }
}

describe('VideoPage — the video-key panel and the prompt it feeds', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockIssueConnectToken.mockImplementation((id: number) =>
      Promise.resolve({
        success: true,
        data: {
          base_url: 'https://deeprouter.example/',
          script_path: `/i/tok_key${id}`,
        },
      })
    )
  })

  it('lists the single video key, marks it in use, and mints for it', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([key({ id: 7, name: 'my key' })])
    )

    render(<VideoPage />)

    expect(await screen.findByText('my key')).toBeInTheDocument()
    expect(screen.getByText('Selected')).toBeInTheDocument()
    // One key = nothing to switch, so the switching caveat stays hidden.
    expect(screen.queryByText(/Switching rows only changes/)).toBeNull()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(7, ['claude-code'])
    )
  })

  it('selecting a row does not mint; the copy binds the selected key (429 fix)', async () => {
    // Minting is a CriticalRateLimit endpoint (20 / 20 min). Binding it to row
    // selection burned the budget in a few clicks → 429 (measured 2026-10-04).
    // So the page mints ONE preview link, selection mints nothing, and the copy
    // mints for whichever key is selected.
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({ id: 11, name: 'newest key' }),
        key({ id: 12, name: 'older key' }),
      ])
    )
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    })

    render(<VideoPage />)

    // One preview link for the default (newest) key — and only one.
    expect(await screen.findByText(/tok_key11/)).toBeInTheDocument()
    expect(mockIssueConnectToken).toHaveBeenCalledTimes(1)
    expect(
      screen.getByText(/Switching rows only changes future copies/)
    ).toBeInTheDocument()

    // Selecting another row must NOT mint.
    fireEvent.click(screen.getByRole('button', { name: 'older key' }))
    expect(mockIssueConnectToken).toHaveBeenCalledTimes(1)

    // The copy is where the selected key is bound.
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenLastCalledWith(12, ['claude-code'])
    )
    const copied = writeText.mock.calls[0][0] as string
    expect(copied).toContain('/i/tok_key12?format=env')
  })

  it('never offers a disabled key', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({ id: 21, name: 'disabled key', status: 2 }),
        key({ id: 22, name: 'live key' }),
      ])
    )

    render(<VideoPage />)

    expect(await screen.findByText('live key')).toBeInTheDocument()
    expect(screen.queryByText('disabled key')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(22, ['claude-code'])
    )
  })

  it('hides chat-limited keys: empty panel, no doomed binding', async () => {
    // AC-G regression: this key used to bind silently and the project 403'd
    // on its very first generation, deep inside the agent's flow.
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 31,
          name: 'chat key',
          model_limits_enabled: true,
          model_limits: 'gpt-4o,claude-*,deeprouter',
        }),
      ])
    )

    render(<VideoPage />)

    expect(
      await screen.findByText(/No video key yet/)
    ).toBeInTheDocument()
    expect(
      screen.getByText(/Create a video key above first/)
    ).toBeInTheDocument()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()
  })

  it('lists a video-purpose whitelisted key (it passes every video model)', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 32,
          name: 'video key',
          model_limits_enabled: true,
          model_limits: 'MiniMax-H3,doubao-seedance-*,deeprouter-video',
        }),
      ])
    )

    render(<VideoPage />)

    expect(await screen.findByText('video key')).toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(32, ['claude-code'])
    )
  })

  it('excludes a key that passes only some video models', async () => {
    // Whitelisted for H3 only: the prompt teaches switching to Seedance by
    // voice, so binding this key would break on the first switch. Strictly
    // all-or-nothing keeps the promise "a video key runs every video model".
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 51,
          name: 'h3-only key',
          model_limits_enabled: true,
          model_limits: 'MiniMax-H3',
        }),
      ])
    )

    render(<VideoPage />)

    expect(await screen.findByText(/No video key yet/)).toBeInTheDocument()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()
  })

  it('one-click create: posts the Simple video purpose, then binds the new key', async () => {
    mockGetApiKeys
      .mockResolvedValueOnce(keysResponse([]))
      .mockResolvedValue(
        keysResponse([key({ id: 99, name: 'my-video-key' })])
      )
    mockCreateApiKey.mockResolvedValue({
      success: true,
      data: { id: 99, key: 'raw-key' },
    })

    render(<VideoPage />)
    expect(await screen.findByText(/No video key yet/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Create a video key/ }))

    await waitFor(() => expect(mockCreateApiKey).toHaveBeenCalledTimes(1))
    // The payload mirrors the keys drawer: purpose set, whitelist left to the
    // backend, auto-name from the purpose.
    expect(mockCreateApiKey).toHaveBeenCalledWith(
      expect.objectContaining({
        simple_purpose: 'video',
        model_limits_enabled: false,
        model_limits: '',
        name: 'my-video-key',
      })
    )
    expect(await screen.findByText('my-video-key')).toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(99, ['claude-code'])
    )
  })

  it('re-mints the one-time link on every copy, so a second paste still works', async () => {
    // The link is one-shot: the first fetch consumes it by design, so copying
    // the same text twice used to hand out a dead URL (measured 2026-10-04 —
    // the agent's own fetch burned it and every retry hit the error stub).
    let mint = 0
    mockIssueConnectToken.mockImplementation(() => {
      mint += 1
      return Promise.resolve({
        success: true,
        data: {
          base_url: 'https://deeprouter.example/',
          script_path: `/i/tok_mint${mint}`,
        },
      })
    })
    mockGetApiKeys.mockResolvedValue(
      keysResponse([key({ id: 61, name: 'my key' })])
    )
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    })

    render(<VideoPage />)
    expect(await screen.findByText(/tok_mint1/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))

    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1))
    const copied = writeText.mock.calls[0][0] as string
    expect(copied).toContain('/i/tok_mint2?format=env')
    expect(copied).not.toContain('tok_mint1')
  })
})
