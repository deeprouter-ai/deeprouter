import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: the purpose split in the post-create dialog (Video First Wave
// AC-G). A video-purpose key's next step is the video page's paste-prompt;
// showing it the chat story (Base URL / model name / self-check) is exactly
// the void the boss fell into ("搞不懂怎么用", 2026-10-04).
import { render as testingRender, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ApiKeySuccessDialog } from '../api-key-success-dialog'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('../api-key-integration-dialog', () => ({
  ApiKeyIntegrationDialog: () => null,
}))

function render(element: React.ReactNode) {
  return testingRender(
    <QueryClientProvider client={new QueryClient()}>
      {element}
    </QueryClientProvider>
  )
}

vi.mock('../media-key-setup', () => ({ MediaKeySetup: () => null }))

describe('ApiKeySuccessDialog purpose split', () => {
  it('routes a video-purpose key to the video page, not the chat story', () => {
    render(
      <ApiKeySuccessDialog
        open
        onClose={() => {}}
        apiKey='sk-test'
        purpose='video'
      />
    )

    const go = screen.getByText('Go to Make videos →').closest('a')
    expect(go).toHaveAttribute('href', '/video')
    // The chat trio must be gone: Base URL, model name, self-check.
    expect(screen.queryByText('Base URL')).not.toBeInTheDocument()
    expect(screen.queryByText('Model name')).not.toBeInTheDocument()
    expect(screen.queryByText('Test this key →')).not.toBeInTheDocument()
    // The key itself is still shown once and copyable.
    expect(screen.getByText('API key')).toBeInTheDocument()
  })

  it('keeps the full chat guidance for non-video purposes', () => {
    render(
      <ApiKeySuccessDialog
        open
        onClose={() => {}}
        apiKey='sk-test'
        purpose='chat'
      />
    )

    expect(screen.getByText('Base URL')).toBeInTheDocument()
    expect(screen.getByText('Model name')).toBeInTheDocument()
    expect(screen.getByText('Test this key →').closest('a')).toHaveAttribute(
      'href',
      '/keys/test'
    )
    expect(screen.queryByText('Go to Make videos →')).not.toBeInTheDocument()
  })
})
