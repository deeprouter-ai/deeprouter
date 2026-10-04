// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiKeyIntegrationDialog } from './api-key-integration-dialog'
import { ApiKeyPurposePicker } from './api-key-purpose-picker'
import { MediaKeySetup } from './media-key-setup'

vi.mock('@/hooks/use-casual', () => ({ useIsCasual: () => false }))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (text: string, values?: { count?: number }) =>
      text.replace('{{count}}', String(values?.count ?? '')),
  }),
}))
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function mount(element: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>{element}</QueryClientProvider>
  )
}

describe('media key setup', () => {
  it('loads this key’s video models and offers the matching task endpoint instead of chat setup', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          {
            id: 'future-video-model',
            supported_endpoint_types: ['video-generation'],
          },
          { id: 'sora-fixture', supported_endpoint_types: ['openai-video'] },
          { id: 'chat-fixture', supported_endpoint_types: ['openai'] },
        ],
      }),
    })
    vi.stubGlobal('fetch', fetchMock)
    mount(
      <ApiKeyIntegrationDialog
        open
        onClose={() => {}}
        apiKey='fixture-key'
        purpose='video'
      />
    )
    await screen.findByText('All 2 models below are included in this key.')
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe(
      'Bearer fixture-key'
    )
    expect(screen.queryByText('deeprouter-auto')).not.toBeInTheDocument()
    expect(screen.queryByText('Test this key →')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('option', { name: 'chat-fixture' })
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByText('Developer details'))
    expect(screen.getByText(/POST .*\/video\/generations/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('combobox'))
    await userEvent.click(screen.getByRole('option', { name: 'sora-fixture' }))
    expect(screen.getByText(/POST .*\/v1\/videos$/)).toBeInTheDocument()
  })

  it('keeps catalog failures visible and lets the user retry without inventing a model', async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: [
            {
              id: 'eleven_fixture',
              supported_endpoint_types: ['audio-speech'],
            },
          ],
        }),
      })
    vi.stubGlobal('fetch', fetchMock)
    mount(<MediaKeySetup apiKey='fixture-key' purpose='voice' />)
    await screen.findByText(
      'Could not load this key’s models. Retry before configuring your tool.'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() =>
      expect(screen.getByText('eleven_fixture')).toBeInTheDocument()
    )
  })

  it('does not substitute a chat model for an empty image directory', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: [] }) })
    )
    mount(<MediaKeySetup apiKey='fixture-key' purpose='image' />)
    await screen.findByText(
      'This key has no models for this purpose. Create a new key with the matching purpose, or contact support.'
    )
    expect(screen.queryByText('Developer details')).not.toBeInTheDocument()
  })
})

it('blocks media creation choices when the catalog is empty or unavailable', async () => {
  const choose = vi.fn()
  mount(
    <ApiKeyPurposePicker
      onValueChange={choose}
      options={[
        {
          id: 'video',
          label: 'Video generation',
          icon: '',
          desc: '',
          human_estimate: '',
          price_range: '',
          recommended_brand: '',
          available_brands: [],
          available: false,
          available_models: [],
        },
      ]}
    />
  )
  const button = screen.getByRole('button', { name: /Video generation/ })
  expect(button).toBeDisabled()
  await userEvent.click(button)
  expect(choose).not.toHaveBeenCalled()
})
