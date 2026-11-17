// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SmartRoutingSettingsCard } from '../smart-routing-settings-card'

// Smart Routing V2 (meta-repo docs/smart-routing-v2-prd.md, D4 / D6): the admin
// switch that lets deeprouter-auto ask Jev, and the TypeSafe key it needs. The
// switch must start off; both must save under the exact option keys the
// router catalog is computed from; the saved key is never shown, only its
// presence.

const mockMutateAsync = vi.hoisted(() => vi.fn())
const mockGetStatus = vi.hoisted(() => vi.fn())

vi.mock('@/features/system-settings/hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync: mockMutateAsync, isPending: false }),
}))
vi.mock('@/features/system-settings/models/smart-routing-api', () => ({
  getSmartRoutingStatus: mockGetStatus,
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const ENABLED_KEY = 'smart_router_setting.jev_enabled'
const API_KEY_KEY = 'smart_router_setting.jev_api_key'

/** Renders the card with the status endpoint answering `configured`. */
function renderCard(configured: boolean, enabled = false) {
  mockGetStatus.mockResolvedValue({
    success: true,
    data: { jev_api_key_configured: configured },
  })
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <SmartRoutingSettingsCard
        defaultValues={{
          smart_router_setting: { jev_enabled: enabled, jev_api_key: '' },
        }}
      />
    </QueryClientProvider>
  )
}

describe('SmartRoutingSettingsCard', () => {
  beforeEach(() => {
    mockMutateAsync.mockReset()
    mockMutateAsync.mockResolvedValue(undefined)
    mockGetStatus.mockReset()
  })

  it('starts off and says no key is saved yet', async () => {
    renderCard(false)
    expect(
      screen.getByText('Use Jev to judge the request type when picking a model')
    ).toBeTruthy()
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe(
      'false'
    )
    expect(await screen.findByText('No key saved yet.')).toBeTruthy()
    expect(screen.queryByText('Clear the saved key')).toBeNull()
  })

  it('saves the switch under its option key', async () => {
    const user = userEvent.setup()
    renderCard(false)

    await user.click(screen.getByRole('switch'))
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(mockMutateAsync).toHaveBeenCalledWith({
        key: ENABLED_KEY,
        value: true,
      })
    )
    expect(mockMutateAsync).toHaveBeenCalledTimes(1)
  })

  it('saves a typed key under its option key and empties the field', async () => {
    const user = userEvent.setup()
    renderCard(false)

    const input = screen.getByLabelText('TypeSafe API key')
    await user.type(input, 'k-new')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(mockMutateAsync).toHaveBeenCalledWith({
        key: API_KEY_KEY,
        value: 'k-new',
      })
    )
    expect(mockMutateAsync).toHaveBeenCalledTimes(1)
    await waitFor(() => expect((input as HTMLInputElement).value).toBe(''))
  })

  it('shows that a key is saved, without the key, and can clear it', async () => {
    const user = userEvent.setup()
    renderCard(true, true)

    expect(
      await screen.findByText('A key is saved. It is never shown here.')
    ).toBeTruthy()
    expect(
      (screen.getByLabelText('TypeSafe API key') as HTMLInputElement).value
    ).toBe('')

    await user.click(screen.getByText('Clear the saved key'))
    await waitFor(() =>
      expect(mockMutateAsync).toHaveBeenCalledWith({
        key: API_KEY_KEY,
        value: '',
      })
    )
  })

  it('sends nothing when saved without a change', async () => {
    const user = userEvent.setup()
    renderCard(true, true)

    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(mockMutateAsync).not.toHaveBeenCalled()
  })
})
