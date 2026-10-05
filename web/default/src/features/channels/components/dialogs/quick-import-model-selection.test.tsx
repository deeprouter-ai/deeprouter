/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { QuickImportProvidersDialog } from './quick-import-providers-dialog'

const createChannel = vi.hoisted(() => vi.fn())
vi.mock('../../api', () => ({ createChannel }))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (text: string) => text }),
}))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('Quick import model selection', () => {
  const setup = () => {
    const queryClient = new QueryClient()
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    render(
      <QueryClientProvider client={queryClient}>
        <QuickImportProvidersDialog open onOpenChange={() => {}} />
      </QueryClientProvider>
    )
    return { invalidate }
  }

  it('offers all nine ElevenLabs models and submits only the chosen subset', async () => {
    createChannel.mockResolvedValue({ success: true })
    setup()
    await userEvent.click(screen.getByRole('checkbox', { name: /ElevenLabs/ }))
    await userEvent.click(screen.getByText(/Choose models/))
    for (const model of [
      'eleven_v4',
      'eleven_v4_turbo',
      'eleven_v3',
      'eleven_v3_conversational',
      'eleven_multilingual_v2',
      'eleven_turbo_v2_5',
      'eleven_flash_v2_5',
      'eleven_flash_v2',
      'eleven_turbo_v2',
    ]) {
      expect(screen.getByRole('checkbox', { name: model })).toBeChecked()
    }
    await userEvent.click(screen.getByRole('button', { name: 'Clear models' }))
    expect(screen.getByRole('button', { name: /Import/ })).toBeDisabled()
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Pick at least one model'
    )
    await userEvent.click(screen.getByRole('checkbox', { name: 'eleven_v3' }))
    await userEvent.click(
      screen.getByRole('checkbox', {
        name: 'eleven_multilingual_v2',
      })
    )
    await userEvent.click(screen.getByRole('button', { name: /Import/ }))
    await waitFor(() => expect(createChannel).toHaveBeenCalledOnce())
    expect(createChannel).toHaveBeenCalledWith(
      expect.objectContaining({
        channel: expect.objectContaining({
          models: 'eleven_v3,eleven_multilingual_v2',
          test_model: 'eleven_v3',
          status: 2,
        }),
      })
    )
  })

  it('keeps the cheap test model when it is among the selected models', async () => {
    createChannel.mockResolvedValue({ success: true })
    setup()
    await userEvent.click(screen.getByRole('checkbox', { name: /ElevenLabs/ }))
    await userEvent.click(screen.getByText(/Choose models/))
    await userEvent.click(screen.getByRole('button', { name: 'Clear models' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'eleven_v3' }))
    await userEvent.click(
      screen.getByRole('checkbox', { name: 'eleven_flash_v2_5' })
    )
    await userEvent.click(screen.getByRole('button', { name: /Import/ }))
    await waitFor(() => expect(createChannel).toHaveBeenCalledOnce())
    expect(createChannel.mock.calls[0][0].channel.test_model).toBe(
      'eleven_flash_v2_5'
    )
  })

  it('refreshes partial successes and retries only the failed provider', async () => {
    createChannel
      .mockResolvedValueOnce({ success: true })
      .mockResolvedValueOnce({ success: false, message: 'temporary error' })
      .mockResolvedValue({ success: true })
    const { invalidate } = setup()
    await userEvent.click(
      screen.getByRole('checkbox', { name: /OpenAI · 对话/ })
    )
    await userEvent.click(screen.getByRole('checkbox', { name: /ElevenLabs/ }))
    await userEvent.click(screen.getByRole('button', { name: /Import/ }))
    await waitFor(() => expect(createChannel).toHaveBeenCalledTimes(2))
    await waitFor(() =>
      expect(
        screen.getByRole('checkbox', { name: /OpenAI · 对话/ })
      ).not.toBeChecked()
    )
    expect(screen.getByRole('checkbox', { name: /ElevenLabs/ })).toBeChecked()
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['channels'] })
    await userEvent.click(screen.getByRole('button', { name: /Import/ }))
    await waitFor(() => expect(createChannel).toHaveBeenCalledTimes(3))
    expect(createChannel.mock.calls[2][0].channel.type).toBe(58)
  })
})
