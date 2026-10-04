/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { ChannelTestDialog } from './channel-test-dialog'
import { QuickImportProvidersDialog } from './quick-import-providers-dialog'

const createChannel = vi.hoisted(() => vi.fn())
vi.mock('../../api', () => ({ createChannel }))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (text: string) => text }),
}))
vi.mock('../channels-provider', () => ({
  useChannels: () => ({
    currentRow: {
      id: 1,
      name: 'MiniMax H3',
      type: 35,
      models: 'MiniMax-H3',
      test_model: 'MiniMax-H3',
    },
  }),
}))
vi.mock('../../lib', () => ({
  formatResponseTime: () => '10ms',
  handleTestChannel: vi.fn(),
}))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

it('imports separate video, image and speech channels with native MiniMax routing', async () => {
  createChannel.mockResolvedValue({ success: true })
  const done = vi.fn()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <QuickImportProvidersDialog open onOpenChange={done} />
    </QueryClientProvider>
  )
  for (const name of [
    'MiniMax · 海螺视频',
    'MiniMax · 画图',
    'MiniMax · 语音合成',
  ]) {
    await userEvent.click(
      screen.getByRole('checkbox', { name: new RegExp(name) })
    )
  }
  await userEvent.click(screen.getByRole('button', { name: /Import/ }))
  await waitFor(() => expect(done).toHaveBeenCalledWith(false))
  expect(createChannel).toHaveBeenCalledTimes(3)
  const channels = createChannel.mock.calls.map(([request]) => request.channel)
  for (const channel of channels) {
    expect(channel).toMatchObject({
      type: 35,
      base_url: 'https://api.minimax.io',
      status: 2,
      key: 'REPLACE_WITH_YOUR_KEY',
    })
  }
  expect(channels[0].models).toBe(
    'MiniMax-H3,MiniMax-Hailuo-2.3,MiniMax-Hailuo-2.3-Fast,MiniMax-Hailuo-02'
  )
  expect(channels[0].test_model).toBe('MiniMax-H3')
  expect(channels[1].models).toBe('image-01')
  expect(channels[2].models).toContain('speech-2.8-hd')
  expect(channels[2].test_model).toBe('speech-2.8-turbo')
})

it('labels H3 connection tests as read-only without generation or chat controls', () => {
  render(<ChannelTestDialog open onOpenChange={() => {}} />)
  expect(
    screen.getByText(/without generating a paid video/)
  ).toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: 'Endpoint Type' })).toBeDisabled()
  expect(screen.getByRole('switch')).toHaveAttribute('aria-disabled', 'true')
})
