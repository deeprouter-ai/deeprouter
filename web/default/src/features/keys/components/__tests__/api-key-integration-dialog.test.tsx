/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: the setup guide's "For your AI assistant" handoff and its key
// masking. A bare sk- key looks like any OpenAI key; handed one, Codex had no
// idea what DeepRouter was or where its docs live (boss, 2026-10-05), so the
// handoff sends the docs and base URL along with the key. The guide shows the
// key masked everywhere (screen sharing), while every copy takes the real one.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render as testingRender,
  screen,
  waitFor,
} from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiKeyIntegrationDialog } from '../api-key-integration-dialog'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, unknown>) =>
      key.replace(/\{\{(\w+)\}\}/g, (_, name) => String(vars?.[name] ?? '')),
  }),
}))

vi.mock('../media-key-setup', () => ({ MediaKeySetup: () => null }))

const FULL_KEY = 'sk-abcdEFGHijklMNOPqrstUVWXyz0123456789abcdEFGHwxyz'
const MASKED_KEY = 'sk-abcd**********wxyz'

function render(apiKey: string | null, purpose: string) {
  return testingRender(
    <QueryClientProvider client={new QueryClient()}>
      <ApiKeyIntegrationDialog
        open
        onClose={() => {}}
        apiKey={apiKey}
        purpose={purpose}
      />
    </QueryClientProvider>
  )
}

/** The copy button sitting next to the element with this text. */
function copyButtonBeside(text: string | RegExp) {
  return screen.getByText(text).parentElement!.querySelector('button')!
}

describe('ApiKeyIntegrationDialog AI handoff and key masking', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    writeText.mockClear()
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    })
  })

  it('step 1 hands the key to an AI together with the docs', async () => {
    render(FULL_KEY, 'chat')

    expect(screen.getByText('For your AI assistant')).toBeInTheDocument()
    const shown = screen.getByText(/^I use DeepRouter/).textContent ?? ''
    expect(shown).toMatch(/https?:\/\/[^ ]+\/llms\.txt/)
    expect(shown).toMatch(/\/v1\/models/)
    expect(shown).toContain(MASKED_KEY)
    expect(shown).not.toContain('{{')

    fireEvent.click(copyButtonBeside(/^I use DeepRouter/))
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1))
    expect(writeText.mock.calls[0][0]).toContain(`API key: ${FULL_KEY}.`)
  })

  it('never puts the full key on screen, yet every copy takes the real one', async () => {
    render(FULL_KEY, 'chat')

    // Step 1: masked in the key field and the handoff, tooltips included.
    expect(document.body.innerHTML).not.toContain(FULL_KEY)
    expect(screen.getByText(MASKED_KEY)).toBeInTheDocument()
    fireEvent.click(copyButtonBeside(MASKED_KEY))
    await waitFor(() => expect(writeText).toHaveBeenLastCalledWith(FULL_KEY))

    // Step 2: the snippets embed the key too.
    fireEvent.click(screen.getByRole('button', { name: /Next/ }))
    const pre = await screen.findByText(new RegExp(MASKED_KEY.replace(/\*/g, '\\*')))
    expect(document.body.innerHTML).not.toContain(FULL_KEY)
    fireEvent.click(pre.closest('pre')!.parentElement!.querySelector('button')!)
    await waitFor(() =>
      expect(writeText.mock.lastCall?.[0]).toContain(FULL_KEY)
    )
  })

  it('image keys get it too: they have no paste-prompt of their own', () => {
    render(FULL_KEY, 'image')

    expect(screen.getByText('For your AI assistant')).toBeInTheDocument()
    expect(document.body.innerHTML).not.toContain(FULL_KEY)
  })

  it('skips video keys and the keyless guide', () => {
    // Video keys belong to the video page's one-time paste-prompt; a guide
    // opened without a key has nothing real to hand over.
    const { unmount } = render(FULL_KEY, 'video')
    expect(screen.queryByText('For your AI assistant')).not.toBeInTheDocument()
    unmount()

    render(null, 'chat')
    expect(screen.queryByText('For your AI assistant')).not.toBeInTheDocument()
  })
})
