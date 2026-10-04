/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: the channel row menu's Delete action. Regression for 2026-10-04:
// the item was wired to `onSelect`, a prop Base UI's Menu.Item does not have,
// so clicking Delete silently did nothing — no confirm dialog, no deletion.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Row } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Channel } from '../../types'
import { DataTableRowActions } from '../data-table-row-actions'

const { mockHandleDeleteChannel, mockSetOpen, mockSetCurrentRow } = vi.hoisted(
  () => ({
    mockHandleDeleteChannel: vi.fn(),
    mockSetOpen: vi.fn(),
    mockSetCurrentRow: vi.fn(),
  })
)

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('../../lib', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  handleDeleteChannel: mockHandleDeleteChannel,
}))

vi.mock('../channels-provider', () => ({
  useChannels: () => ({
    setOpen: mockSetOpen,
    setCurrentRow: mockSetCurrentRow,
    upstream: { openModal: vi.fn(), detectChannelUpdates: vi.fn() },
  }),
}))

// Minimal channel: a type outside MODEL_FETCHABLE_TYPES and no multi-key
// info, so the menu renders without its conditional extras.
const CHANNEL = {
  id: 42,
  name: 'MiniMax intl video',
  type: 35,
  status: 1,
  models: 'MiniMax-H3',
  group: 'default',
} as Channel

function renderRow() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <DataTableRowActions row={{ original: CHANNEL } as Row<Channel>} />
    </QueryClientProvider>
  )
}

// Base UI's menu mounts asynchronously — poll for it (findBy*) rather than
// assuming it is already in the DOM the instant the click resolves.
async function openMenu() {
  const buttons = screen.getAllByRole('button')
  await userEvent.click(buttons[buttons.length - 1])
  await screen.findByRole('menu')
}

describe('Channels row actions — Delete', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('opens the confirm dialog when Delete is clicked', async () => {
    renderRow()
    await openMenu()

    await userEvent.click(
      await screen.findByRole('menuitem', { name: /Delete/i })
    )

    expect(await screen.findByText('Delete Channel')).toBeInTheDocument()
    // Opening the dialog must not already delete anything.
    expect(mockHandleDeleteChannel).not.toHaveBeenCalled()
  })

  it('deletes the channel once the dialog is confirmed', async () => {
    renderRow()
    await openMenu()
    await userEvent.click(
      await screen.findByRole('menuitem', { name: /Delete/i })
    )
    await screen.findByText('Delete Channel')

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(mockHandleDeleteChannel).toHaveBeenCalledWith(
      CHANNEL.id,
      expect.anything()
    )
  })
})
