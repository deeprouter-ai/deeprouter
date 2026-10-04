import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { KeyModelDiscovery } from './key-model-discovery'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (text: string) => text }),
}))
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
it('discovers a key’s actual directory only when opened without leaking its key into examples', async () => {
  const fetchMock = vi
    .fn()
    .mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          { id: 'fixture-model', supported_endpoint_types: ['audio-speech'] },
        ],
      }),
    })
  vi.stubGlobal('fetch', fetchMock)
  render(
    <QueryClientProvider client={new QueryClient()}>
      <KeyModelDiscovery apiKey='fixture-private-key' />
    </QueryClientProvider>
  )
  expect(fetchMock).not.toHaveBeenCalled()
  await userEvent.click(screen.getByText('Where can I find this key’s models?'))
  await screen.findByText('fixture-model')
  expect(fetchMock.mock.calls[0][0]).toMatch(/\/v1\/models$/)
  expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe(
    'Bearer fixture-private-key'
  )
  expect(screen.getByText(/curl /).textContent).toContain('$DEEPROUTER_API_KEY')
  expect(screen.getByText(/curl /).textContent).not.toContain(
    'fixture-private-key'
  )
})
