/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3, D15).
// An organization key is listed on its holder's own "API keys" page — that is
// where they run one-click setup from — but nothing on that page may change it
// or fetch its value. The backend refuses those calls; these tests pin that the
// page does not offer them in the first place, and that a personal key next to
// it keeps everything it had.
import type { ReactNode } from 'react'
import type { Row } from '@tanstack/react-table'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchActiveChatKey } from '@/features/chat/hooks/use-active-chat-key'
import { isOrgKey } from '../../lib/org-key'
import type { ApiKey } from '../../types'
import { ApiKeyCell } from '../api-keys-cells'
import { ApiKeysSetupCard } from '../api-keys-setup-card'
import { DataTableRowActions } from '../data-table-row-actions'

const mocks = vi.hoisted(() => ({
  provider: {} as Record<string, unknown>,
  getConnectTools: vi.fn(),
  getApiKeys: vi.fn(),
  fetchTokenKey: vi.fn(),
  updateApiKeyStatus: vi.fn(),
}))

vi.mock('../api-keys-provider', () => ({
  useApiKeys: () => mocks.provider,
}))
vi.mock('../../api', () => ({
  getConnectTools: mocks.getConnectTools,
  getApiKeys: mocks.getApiKeys,
  fetchTokenKey: mocks.fetchTokenKey,
  updateApiKeyStatus: mocks.updateApiKeyStatus,
}))
vi.mock('@/features/chat/hooks/use-chat-presets', () => ({
  useChatPresets: () => ({
    chatPresets: [
      {
        id: 'cherry',
        name: 'Cherry Studio',
        url: 'cherrystudio://providers/api-keys?v=1&data={cherryConfig}',
        type: 'cherry',
      },
    ],
    serverAddress: 'https://gateway.example',
  }),
}))
// The terminal block mints a one-time token on mount and has tests of its own;
// here it only has to be there, for the key it was given.
vi.mock('../api-keys-one-click-card', () => ({
  ApiKeysOneClickSection: ({ apiKey }: { apiKey: ApiKey }) => (
    <div data-testid='one-click'>{apiKey.name}</div>
  ),
}))
vi.mock('../api-keys-ask-ai-section', () => ({
  ApiKeysAskAiSection: () => null,
}))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children?: ReactNode }) => <a>{children}</a>,
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: { id: 1 } } }),
}))
const stableT = (key: string) => key
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: stableT, i18n: { language: 'en' } }),
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}))

/** A key as `GET /api/token` lists it; a personal one unless told otherwise. */
function apiKey(over: Partial<ApiKey> = {}): ApiKey {
  return {
    id: 7,
    name: 'my key',
    key: 'abcd**********wxyz',
    status: 1,
    remain_quota: 500000,
    used_quota: 0,
    unlimited_quota: false,
    expired_time: -1,
    created_time: 1790000000,
    accessed_time: 1790000000,
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
    org_id: 0,
    ...over,
  }
}

const personalKey = apiKey()
const orgKey = apiKey({ id: 8, name: 'from the company', org_id: 5 })

const resolveRealKey = vi.fn()

/** What the page's provider hands its components, for the given keys. */
function provide(setupKey: ApiKey | null, setupKeys: ApiKey[] = []) {
  mocks.provider = {
    setOpen: vi.fn(),
    setCurrentRow: vi.fn(),
    triggerRefresh: vi.fn(),
    setResolvedKey: vi.fn(),
    resolveRealKey,
    resolvedKeys: {},
    loadingKeys: {},
    copiedKeyId: null,
    markKeyCopied: vi.fn(),
    setupKeys,
    setupKeyId: setupKey?.id ?? null,
    setSetupKeyId: vi.fn(),
    setupKey,
  }
}

/** A table row for the given key, as far as the row menu reads it. */
function rowOf(key: ApiKey): Row<ApiKey> {
  return { original: key } as Row<ApiKey>
}

beforeEach(() => {
  vi.clearAllMocks()
  provide(null)
  resolveRealKey.mockResolvedValue('REALVALUE')
  mocks.getConnectTools.mockResolvedValue({
    success: true,
    data: [{ id: 'claude-code', name: 'Claude Code' }],
  })
})

describe('telling an organization key from a personal one', () => {
  it('goes by the organization the key belongs to', () => {
    expect(isOrgKey(orgKey)).toBe(true)
    expect(isOrgKey(personalKey)).toBe(false)
    // The backend leaves the field out for a personal key.
    expect(isOrgKey({})).toBe(false)
    expect(isOrgKey(null)).toBe(false)
    expect(isOrgKey(undefined)).toBe(false)
  })
})

describe('the key column', () => {
  it('shows an organization key masked, with nothing to press', async () => {
    render(<ApiKeyCell apiKey={orgKey} />)

    expect(screen.getByText('sk-abcd**********wxyz')).toBeInTheDocument()
    expect(screen.getByText('Organization key')).toBeInTheDocument()
    expect(screen.queryByRole('button')).toBeNull()
    await userEvent.click(screen.getByText('sk-abcd**********wxyz'))
    expect(resolveRealKey).not.toHaveBeenCalled()
  })

  it('still reveals and copies a personal key', async () => {
    render(<ApiKeyCell apiKey={personalKey} />)

    expect(screen.queryByText('Organization key')).toBeNull()
    const [reveal, copy] = screen.getAllByRole('button')
    expect(reveal).toHaveTextContent('sk-abcd**********wxyz')
    await userEvent.click(copy)
    await waitFor(() => expect(resolveRealKey).toHaveBeenCalledWith(7))
  })
})

describe('the row menu', () => {
  it('has no action for an organization key, and says who has', () => {
    render(<DataTableRowActions row={rowOf(orgKey)} />)

    expect(screen.getByText('Managed by your organization')).toBeInTheDocument()
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('still has every action for a personal key', async () => {
    mocks.updateApiKeyStatus.mockResolvedValue({ success: true })
    render(<DataTableRowActions row={rowOf(personalKey)} />)

    expect(screen.queryByText('Managed by your organization')).toBeNull()
    expect(screen.getByText('Open menu')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))
    await waitFor(() =>
      expect(mocks.updateApiKeyStatus).toHaveBeenCalledWith(7, 2)
    )
  })
})

describe('the setup box', () => {
  it('leaves an organization key one-click setup and takes the app buttons away', async () => {
    // The app buttons work by handing the app the key's value, which this
    // page cannot have. One-click setup is the road that is left (PRD D15).
    provide(orgKey, [orgKey])
    render(<ApiKeysSetupCard />)

    expect(await screen.findByTestId('one-click')).toHaveTextContent(
      'from the company'
    )
    expect(screen.queryByRole('button', { name: /Cherry Studio/ })).toBeNull()
  })

  it('keeps the app buttons for a personal key', async () => {
    provide(personalKey, [personalKey])
    render(<ApiKeysSetupCard />)

    expect(await screen.findByTestId('one-click')).toHaveTextContent('my key')
    expect(
      screen.getByRole('button', { name: /Cherry Studio/ })
    ).toBeInTheDocument()
  })

  it('is not there at all for an organization key when no tool can be set up', async () => {
    // Without the terminal tools the app buttons were all the box had.
    mocks.getConnectTools.mockResolvedValue({ success: true, data: [] })
    provide(orgKey, [orgKey])
    const { container } = render(<ApiKeysSetupCard />)

    await waitFor(() => expect(mocks.getConnectTools).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})

describe('the key a chat link is given', () => {
  it('is a personal one, even when an organization key comes first', async () => {
    mocks.getApiKeys.mockResolvedValue({
      success: true,
      data: { items: [orgKey, personalKey] },
    })
    mocks.fetchTokenKey.mockResolvedValue({
      success: true,
      data: { key: 'REALVALUE' },
    })

    expect(await fetchActiveChatKey()).toBe('sk-REALVALUE')
    expect(mocks.fetchTokenKey).toHaveBeenCalledWith(7)
    expect(mocks.fetchTokenKey).toHaveBeenCalledTimes(1)
  })

  it('is none when organization keys are all there is', async () => {
    mocks.getApiKeys.mockResolvedValue({
      success: true,
      data: { items: [orgKey] },
    })

    await expect(fetchActiveChatKey()).rejects.toThrow(
      'No enabled API keys found'
    )
    expect(mocks.fetchTokenKey).not.toHaveBeenCalled()
  })
})
