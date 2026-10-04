/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChannelsPrimaryButtons } from '../channels-primary-buttons'
import { QuickImportProvidersDialog } from './quick-import-providers-dialog'

const createChannel = vi.hoisted(() => vi.fn())
const setOpen = vi.hoisted(() => vi.fn())
vi.mock('../../api', () => ({ createChannel }))
vi.mock('../channels-provider', () => ({
  useChannels: () => ({
    setOpen,
    enableTagMode: false,
    setEnableTagMode: vi.fn(),
    idSort: false,
    setIdSort: vi.fn(),
    upstream: { syncing: false, refresh: vi.fn() },
  }),
}))
vi.mock('../../lib', () => ({
  handleDeleteAllDisabled: vi.fn(),
  handleFixAbilities: vi.fn(),
  handleTestAllChannels: vi.fn(),
  handleUpdateAllBalances: vi.fn(),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (text: string) => text }),
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('MiniMax quick import', () => {
  it('opens provider presets from the primary creation action', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ChannelsPrimaryButtons />
      </QueryClientProvider>
    )
    await userEvent.click(
      screen.getByRole('button', { name: /Create Channel/ })
    )
    expect(setOpen).toHaveBeenCalledWith('quick-import-providers')
  })

  it('keeps manual creation available without importing a channel', async () => {
    const onManualCreate = vi.fn()
    render(
      <QueryClientProvider client={new QueryClient()}>
        <QuickImportProvidersDialog
          open
          onOpenChange={() => {}}
          onManualCreate={onManualCreate}
        />
      </QueryClientProvider>
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Manual configuration' })
    )
    expect(onManualCreate).toHaveBeenCalledOnce()
    expect(createChannel).not.toHaveBeenCalled()
  })

  it('imports only the selected provider as a disabled, keyless skeleton', async () => {
    createChannel.mockResolvedValue({ success: true })
    const onOpenChange = vi.fn()
    render(
      <QueryClientProvider client={new QueryClient()}>
        <QuickImportProvidersDialog open onOpenChange={onOpenChange} />
      </QueryClientProvider>
    )
    await userEvent.click(
      screen.getByRole('checkbox', { name: /MiniMax · 对话/ })
    )
    await userEvent.click(screen.getByRole('button', { name: /Import/ }))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(createChannel).toHaveBeenCalledTimes(1)
    expect(createChannel).toHaveBeenCalledWith({
      mode: 'single',
      channel: {
        name: 'MiniMax · 对话',
        type: 1,
        key: 'REPLACE_WITH_YOUR_KEY',
        base_url: 'https://api.minimax.io',
        models: 'MiniMax-M3,MiniMax-M2.7,MiniMax-M2',
        test_model: 'MiniMax-M2',
        group: 'default',
        status: 2,
      },
    })
  })
})

