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
/**
 * What the page is told about the user's organization: a personal account
 * unless a test says otherwise.
 */
const membership = vi.hoisted(() => ({ current: { data: null as unknown } }))

vi.mock('@/features/keys/api', () => ({
  getApiKeys: mockGetApiKeys,
  issueConnectToken: mockIssueConnectToken,
  createApiKey: mockCreateApiKey,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { default_use_auto_group: false } }),
}))
vi.mock('@/features/org/hooks/use-org-membership', () => ({
  useOrgMembership: () => membership.current,
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
    membership.current = { data: null }
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
      expect(mockIssueConnectToken).toHaveBeenLastCalledWith(12, [
        'claude-code',
      ])
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

    expect(await screen.findByText(/No video key yet/)).toBeInTheDocument()
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

  it('accepts a key granted only some video models, and scopes the prompt to them', async () => {
    // The backend grants exactly the video models the account has enabled
    // (internal/keypurpose), so a one-model key is normal — not broken. It
    // used to be filtered out entirely, hiding a key the user had just made.
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

    expect(await screen.findByText('h3-only key')).toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(51, ['claude-code'])
    )
    // The prompt may only teach the model this key can actually call.
    const prompt = await screen.findByText(/MiniMax-H3/)
    expect(prompt.textContent).not.toContain('doubao-seedance')
  })

  it('defaults the prompt to the cheapest model the key may call', async () => {
    // No MiniMax channel on the account → the key carries Seedance only. The
    // prompt must not keep pointing at MiniMax-H3, which would 403.
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 52,
          name: 'seedance key',
          model_limits_enabled: true,
          model_limits: 'doubao-seedance-2-0-260128,doubao-seedance-2-5-260628',
        }),
      ])
    )

    render(<VideoPage />)

    const prompt = await screen.findByText(/doubao-seedance-2-0-260128/)
    expect(prompt.textContent).not.toContain('MiniMax-H3')
    // Cheapest of the two leads: 2-0 ($1.0) before 2-5 ($5.4).
    expect(prompt.textContent).toContain(
      'Default model: doubao-seedance-2-0-260128'
    )
  })

  it('one-click create: posts the Simple video purpose, then binds the new key', async () => {
    mockGetApiKeys
      .mockResolvedValueOnce(keysResponse([]))
      .mockResolvedValue(keysResponse([key({ id: 99, name: 'my-video-key' })]))
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
  it('opens with a plain-language primer: what a key is, why, and how', async () => {
    // The page's only intro — the layout drops the Description slot, so the
    // one this page used to pass never rendered. Pinned so a refactor cannot
    // quietly strip the part first-timers actually read.
    mockGetApiKeys.mockResolvedValue(keysResponse([]))

    render(<VideoPage />)

    expect(
      await screen.findByText('What is a key (API Key)?')
    ).toBeInTheDocument()
    expect(screen.getByText('Why do I need one?')).toBeInTheDocument()
    expect(screen.getByText('How do I make a video?')).toBeInTheDocument()
    // "Project" is jargon to this audience: the steps must say it is a folder
    // they make themselves, and that next time they reopen the same one —
    // the setup lives in it, so any other folder knows nothing.
    expect(
      screen.getByText(/That folder is your "project"/)
    ).toBeInTheDocument()
    expect(screen.getByText(/open the same folder/)).toBeInTheDocument()
  })
})

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md D16): a member of an
// organization makes no key of their own, so with no key that fits the page
// says whom to ask instead of where to create one.
describe('VideoPage — a member of an organization', () => {
  const WHOM_TO_ASK =
    /Keys are handed out by your organization — ask an administrator for one\./

  beforeEach(() => {
    vi.clearAllMocks()
    membership.current = { data: { org_id: 1 } }
  })

  it('is told whom to ask when they were handed no key at all', async () => {
    mockGetApiKeys.mockResolvedValue(keysResponse([]))

    render(<VideoPage />)

    const hint = await screen.findByText(WHOM_TO_ASK)
    expect(hint).toHaveTextContent(
      'No key has been assigned to you yet. Keys are handed out by your organization — ask an administrator for one.'
    )
    // A member makes no key of their own: no one-click button, and the primer
    // and the prompt panel do not send them to one.
    expect(
      screen.queryByRole('button', { name: /Create a video key/ })
    ).toBeNull()
    expect(screen.queryByText(/No video key yet/)).toBeNull()
    expect(screen.queryByText(/Create a video key below/)).toBeNull()
    expect(
      screen.getByText('Your organization hands you a video key (below).')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'The text to copy appears here once you have a video key.'
      )
    ).toBeInTheDocument()
    expect(mockCreateApiKey).not.toHaveBeenCalled()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()
  })

  it('is told the same when none of their keys can run video models', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 61,
          name: 'chat key',
          org_id: 1,
          model_limits_enabled: true,
          model_limits: 'gpt-4o',
        }),
      ])
    )

    render(<VideoPage />)

    const hint = await screen.findByText(WHOM_TO_ASK)
    expect(hint).toHaveTextContent(
      'None of the keys assigned to you can run video models. Keys are handed out by your organization — ask an administrator for one.'
    )
    expect(
      screen.queryByRole('button', { name: /Create a video key/ })
    ).toBeNull()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()
  })

  it('uses a key they were handed that can', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({
          id: 62,
          name: 'video key',
          org_id: 1,
          model_limits_enabled: true,
          model_limits: 'MiniMax-H3',
        }),
      ])
    )

    render(<VideoPage />)

    expect(await screen.findByText('video key')).toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(62, ['claude-code'])
    )
    expect(screen.queryByText(WHOM_TO_ASK)).toBeNull()
  })

  it('still offers a personal account the one-click key', async () => {
    membership.current = { data: null }
    mockGetApiKeys.mockResolvedValue(keysResponse([]))

    render(<VideoPage />)

    expect(await screen.findByText(/No video key yet/)).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /Create a video key/ })
    ).toBeInTheDocument()
    expect(screen.queryByText(WHOM_TO_ASK)).toBeNull()
  })
})
