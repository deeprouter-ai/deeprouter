/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/features/keys/types'
import { VideoPage } from '../index'

const mockGetApiKeys = vi.hoisted(() => vi.fn())
const mockIssueConnectToken = vi.hoisted(() => vi.fn())

vi.mock('@/features/keys/api', () => ({
  getApiKeys: mockGetApiKeys,
  issueConnectToken: mockIssueConnectToken,
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

// The themed Select stands in as a native one: what this file tests is which
// key the token is minted for, not how the popup is drawn.
vi.mock('@/components/ui/select', () => ({
  Select: ({
    items,
    value,
    onValueChange,
  }: {
    items: { value: string; label: string }[]
    value: string
    onValueChange: (v: string) => void
  }) => (
    <select
      aria-label='Key to set up'
      value={value}
      onChange={(e) => onValueChange(e.target.value)}
    >
      {items.map((item) => (
        <option key={item.value} value={item.value}>
          {item.label}
        </option>
      ))}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: () => null,
  SelectGroup: () => null,
  SelectItem: () => null,
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

describe('VideoPage — which key the prompt configures', () => {
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

  it('states the key name when there is only one, without a control', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([key({ id: 7, name: 'my key' })])
    )

    render(<VideoPage />)

    expect(await screen.findByText('my key')).toBeInTheDocument()
    expect(screen.queryByLabelText('Key to set up')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(7, ['claude-code'])
    )
  })

  it('offers a picker with several keys and re-mints for the one chosen', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({ id: 11, name: 'newest key' }),
        key({ id: 12, name: 'older key' }),
      ])
    )

    render(<VideoPage />)

    const select =
      await screen.findByLabelText<HTMLSelectElement>('Key to set up')
    // Newest-first list, newest is the default — and the prompt must carry its token.
    expect(select.value).toBe('11')
    expect(await screen.findByText(/tok_key11/)).toBeInTheDocument()

    await userEvent.selectOptions(select, '12')

    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenLastCalledWith(12, [
        'claude-code',
      ])
    )
    expect(await screen.findByText(/tok_key12/)).toBeInTheDocument()
  })

  it('never offers a disabled key', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({ id: 21, name: 'disabled key', status: 2 }),
        key({ id: 22, name: 'live key' }),
      ])
    )

    render(<VideoPage />)

    // One enabled key left, so no picker — and the one named is the enabled one.
    expect(await screen.findByText('live key')).toBeInTheDocument()
    expect(screen.queryByText('disabled key')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(22, ['claude-code'])
    )
  })

  it('warns when the chosen key is restricted to some models', async () => {
    mockGetApiKeys.mockResolvedValue(
      keysResponse([
        key({ id: 31, name: 'limited key', model_limits_enabled: true }),
      ])
    )

    render(<VideoPage />)

    expect(
      await screen.findByText(/This key is limited to certain models/)
    ).toBeInTheDocument()
  })
})
