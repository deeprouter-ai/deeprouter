/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { PurposeSummary } from '../../types'
import { ApiKeysMutateDrawer } from '../api-keys-mutate-drawer'

// The UI language the drawer sees; tests flip it between renders.
const lang = vi.hoisted(() => ({ current: 'zh' }))
const mockGetApiKeyPurposes = vi.hoisted(() => vi.fn())

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: lang.current },
  }),
}))

vi.mock('../../api', () => ({
  createApiKey: vi.fn(),
  updateApiKey: vi.fn(),
  getApiKey: vi.fn(),
  getApiKeyPurposes: mockGetApiKeyPurposes,
}))

vi.mock('@/lib/api', () => ({
  getUserModels: () => Promise.resolve({ success: true, data: [] }),
  getUserGroups: () => Promise.resolve({ success: true, data: {} }),
}))

vi.mock('@/hooks/use-admin', () => ({ useIsAdmin: () => false }))
vi.mock('@/hooks/use-status', () => ({ useStatus: () => ({ status: {} }) }))
vi.mock('../api-keys-provider', () => ({
  useApiKeys: () => ({ triggerRefresh: vi.fn(), setOpen: vi.fn() }),
}))

// Only the cards' text matters here: render each option's label.
vi.mock('../api-key-purpose-picker', () => ({
  ApiKeyPurposePicker: ({ options }: { options: PurposeSummary[] }) => (
    <ul>
      {options.map((o) => (
        <li key={o.id}>{o.label}</li>
      ))}
    </ul>
  ),
}))
vi.mock('../api-key-brand-filter', () => ({ ApiKeyBrandFilter: () => null }))
vi.mock('../api-key-price-tier', () => ({ ApiKeyPriceTier: () => null }))
vi.mock('../api-key-group-combobox', () => ({
  ApiKeyGroupCombobox: () => null,
}))
vi.mock('../api-key-success-dialog', () => ({
  ApiKeySuccessDialog: () => null,
}))
vi.mock('@/components/datetime-picker', () => ({ DateTimePicker: () => null }))
vi.mock('@/components/multi-select', () => ({ MultiSelect: () => null }))

/** What the backend returns: one card, localised by the caller's language. */
function purposesFor(language: string) {
  return {
    success: true,
    data: {
      purposes: [
        {
          id: 'chat',
          label: language === 'zh' ? '聊天 / 写作' : 'Chat / Writing',
          icon: '',
          desc: '',
          human_estimate: '',
          price_range: '',
          recommended_brand: '',
          available_brands: [],
        },
      ],
      price_tiers: [],
      default_price_tier: 'standard',
    },
  }
}

describe('ApiKeysMutateDrawer — localised purpose cards', () => {
  it('refetches the cards in the new UI language, asking for it explicitly', async () => {
    // Measured 2026-10-05, twice. First the query was keyed by nothing, so the
    // cached Chinese cards survived a switch to English. Keying by language
    // was not enough either: the drawer stays mounted while closed, so the
    // switch refetches at once — before the new language reaches the saved
    // setting — and the backend answered in the old one. Modelled here: with
    // no explicit language, the backend replies in the stale setting (zh).
    mockGetApiKeyPurposes.mockImplementation((requested?: string) =>
      Promise.resolve(purposesFor(requested ?? 'zh'))
    )
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const wrap = (node: ReactNode) => (
      <QueryClientProvider client={client}>{node}</QueryClientProvider>
    )

    lang.current = 'zh'
    const { rerender } = render(
      wrap(<ApiKeysMutateDrawer open onOpenChange={() => {}} />)
    )
    expect(await screen.findByText('聊天 / 写作')).toBeInTheDocument()

    lang.current = 'en'
    rerender(wrap(<ApiKeysMutateDrawer open onOpenChange={() => {}} />))
    expect(await screen.findByText('Chat / Writing')).toBeInTheDocument()
    expect(screen.queryByText('聊天 / 写作')).toBeNull()
    expect(mockGetApiKeyPurposes).toHaveBeenCalledTimes(2)
    expect(mockGetApiKeyPurposes).toHaveBeenLastCalledWith('en')
  })
})
