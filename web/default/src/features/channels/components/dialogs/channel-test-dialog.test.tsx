import { render, screen, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChannelTestDialog } from './channel-test-dialog'

const fixture = vi.hoisted(() => ({
  channel: {
    id: 1,
    name: 'Seedance fixture',
    type: 54,
    models: 'doubao-seedance-2-5-260628',
    test_model: '',
  },
  success: true,
}))
vi.mock('../channels-provider', () => ({
  useChannels: () => ({ currentRow: fixture.channel }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (text: string) => text }),
}))
vi.mock('../../lib', () => ({
  formatResponseTime: () => '10 ms',
  handleTestChannel: async (
    _id: number,
    _options: unknown,
    done: (success: boolean, time?: number, error?: string) => void
  ) => {
    done(
      fixture.success,
      10,
      fixture.success ? undefined : 'AuthenticationError'
    )
  },
}))

afterEach(() => {
  cleanup()
  fixture.channel.type = 54
  fixture.channel.models = 'doubao-seedance-2-5-260628'
  fixture.success = true
})

describe('Seedance connection-test scope', () => {
  it('labels successful connectivity without claiming generation and disables irrelevant controls', async () => {
    fixture.success = true
    render(<ChannelTestDialog open onOpenChange={() => {}} />)
    expect(screen.getByText(/Checks the video task API/)).toBeInTheDocument()
    expect(screen.getByRole('switch')).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('combobox', { name: 'Endpoint Type' })
    ).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: /^Test$/ }))
    await waitFor(() =>
      expect(
        screen.getByText('Connected — model generation not tested')
      ).toBeInTheDocument()
    )
    expect(
      screen.queryByText('Success', { exact: true })
    ).not.toBeInTheDocument()
  })

  it('keeps provider authentication failures visible', async () => {
    fixture.success = false
    render(<ChannelTestDialog open onOpenChange={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: /^Test$/ }))
    await waitFor(() =>
      expect(screen.getByText('AuthenticationError')).toBeInTheDocument()
    )
    expect(screen.getByText('Failed')).toBeInTheDocument()
  })
})

describe('Mixed MiniMax channel test scope', () => {
  it('shows video connectivity separately from chat success', async () => {
    fixture.channel.type = 35
    fixture.channel.models = 'MiniMax-M3,MiniMax-H3'
    render(<ChannelTestDialog open onOpenChange={() => {}} />)
    const buttons = screen.getAllByRole('button', { name: /^Test$/ })
    await userEvent.click(buttons[0])
    await userEvent.click(screen.getAllByRole('button', { name: /^Test$/ })[1])
    await waitFor(() => {
      expect(screen.getByText('Success', { exact: true })).toBeInTheDocument()
      expect(
        screen.getByText('Connected — model generation not tested')
      ).toBeInTheDocument()
    })
  })
})
