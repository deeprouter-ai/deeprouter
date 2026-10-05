/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: one-click PRD §11 / §7 Track C — the downloadable installers on the
// key page. Two things here are worth a test rather than an eyeball:
// the Windows security warning has to be readable BEFORE the click (that is the
// acceptance item), and the download has to mint a FRESH token rather than reuse
// the one issued when the page loaded.
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey, ConnectTool } from '../../types'
import { ApiKeysOneClickSection } from '../api-keys-one-click-card'

const mockIssueConnectToken = vi.hoisted(() => vi.fn())

vi.mock('../../api', () => ({
  issueConnectToken: mockIssueConnectToken,
}))

// `t` must be a STABLE reference, as the real react-i18next's is: the
// component keeps it in the mint effect's deps, and a fresh function per
// render would re-fire the effect after its own setState — double-minting in
// the test what mints once in the app.
const stableT = (key: string) => key
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: stableT }),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}))

const TOOLS: ConnectTool[] = [
  { id: 'claude-code', name: 'Claude Code' },
  { id: 'codex', name: 'Codex' },
]

const KEY = { id: 7, name: 'my key' } as ApiKey

/** The component reads navigator.platform once, at mount. */
function onPlatform(platform: string) {
  Object.defineProperty(navigator, 'platform', {
    value: platform,
    configurable: true,
  })
}

/** Every anchor the component clicked, in order. */
function captureDownloads() {
  const clicks: { href: string; download: string }[] = []
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
    this: HTMLAnchorElement
  ) {
    clicks.push({ href: this.href, download: this.download })
  })
  return clicks
}

/** Stub fetch + blob URLs for the .pkg road; returns the fetch mock. */
function stubPkgFetch(status: number) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    blob: () => Promise.resolve(new Blob(['signed-bytes'])),
  })
  vi.stubGlobal('fetch', fetchMock)
  URL.createObjectURL = vi.fn(() => 'blob:mock-pkg')
  URL.revokeObjectURL = vi.fn()
  return fetchMock
}

describe('ApiKeysOneClickSection — downloadable installers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    let minted = 0
    mockIssueConnectToken.mockImplementation(() => {
      minted += 1
      return Promise.resolve({
        success: true,
        data: {
          token: `TOKEN${minted}`,
          expires_in: 1800,
          base_url: 'https://deeprouter.example/',
          tools: ['claude-code', 'codex'],
          script_path: `/i/TOKEN${minted}`,
        },
      })
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  // 🔴 The acceptance item: forewarned, the "publisher unknown" dialog reads as
  // expected; met cold, a non-technical user closes it and gives up.
  it('warns about the Windows security prompt before anything is clicked', async () => {
    onPlatform('Win32')
    const clicks = captureDownloads()
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    expect(
      await screen.findByText(/Windows will warn that the publisher is unknown/)
    ).toBeInTheDocument()
    // Readable without having downloaded anything: nothing was clicked.
    expect(clicks).toHaveLength(0)
  })

  // 🔴 The lazy-mint contract (2026-09-29, after a live 429): rendering the
  // page mints NOTHING — the issuing endpoint sits behind CriticalRateLimit
  // (~20 per 20 min), and the old mint-on-load/re-mint-per-checkbox shape
  // burned that budget on renders nobody read. The command only exists once
  // the fallback is opened.
  it('mints nothing on load; opening the command fallback mints the command', async () => {
    onPlatform('Win32')
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    expect(
      await screen.findByRole('button', { name: 'Download for Windows' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Download for macOS' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Download for Linux' })
    ).toBeInTheDocument()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()

    // The command road survives, collapsed as the failure fallback.
    await userEvent.click(
      screen.getByText('Setup didn’t work? Try a terminal command')
    )
    expect(
      await screen.findByText('irm https://deeprouter.example/i/TOKEN1 | iex')
    ).toBeInTheDocument()
    expect(mockIssueConnectToken).toHaveBeenCalledTimes(1)
  })

  it('mints a fresh token per download click, none on load', async () => {
    onPlatform('Win32')
    const clicks = captureDownloads()
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    const button = await screen.findByRole('button', {
      name: 'Download for Windows',
    })
    expect(mockIssueConnectToken).not.toHaveBeenCalled()

    await userEvent.click(button)
    await waitFor(() => expect(clicks).toHaveLength(1))
    await userEvent.click(button)
    await waitFor(() => expect(clicks).toHaveLength(2))

    expect(mockIssueConnectToken).toHaveBeenCalledTimes(2)
    expect(mockIssueConnectToken).toHaveBeenLastCalledWith(7, [
      'claude-code',
      'codex',
    ])
    // Each file carries its own token.
    expect(clicks[0].href).toBe(
      'https://deeprouter.example/d/TOKEN1/deeprouter-setup.cmd'
    )
    expect(clicks[1].href).toBe(
      'https://deeprouter.example/d/TOKEN2/deeprouter-setup.cmd'
    )
  })

  // "Please try again" is exactly the wrong advice for a rate limit — the one
  // failure where retrying digs deeper gets its own words.
  it('says wait, not retry, when the mint is rate-limited', async () => {
    onPlatform('Win32')
    captureDownloads()
    mockIssueConnectToken.mockRejectedValue({ response: { status: 429 } })
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    await userEvent.click(
      await screen.findByRole('button', { name: 'Download for Windows' })
    )

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Too many requests — wait a minute, then try again.'
      )
    )
  })

  it('downloads the .sh for Linux, with no terminal line attached', async () => {
    onPlatform('Linux x86_64')
    const clicks = captureDownloads()
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    // 🔴 Removed on 2026-09-29 (@sam relaying the boss): the download road
    // promises no terminal, so it must not print a terminal line under its own
    // button. Pinned here because it reads like a helpful thing to add back.
    expect(
      screen.queryByText('bash ~/Downloads/deeprouter-setup.sh')
    ).not.toBeInTheDocument()

    await userEvent.click(
      await screen.findByRole('button', { name: 'Download for Linux' })
    )

    await waitFor(() => expect(clicks).toHaveLength(1))
    expect(clicks[0].href).toBe(
      'https://deeprouter.example/d/TOKEN1/deeprouter-setup.sh'
    )
  })

  // macOS is the signed .pkg (PRD §11.7): the token and the issuing host ride
  // in the FILENAME, so the page builds the name, fetches the bytes, and saves
  // them under it — Content-Disposition alone is not enough for a client that
  // ignores it, and the name is the data channel the postinstall reads.
  it('downloads the tokenized .pkg for macOS', async () => {
    onPlatform('MacIntel')
    const clicks = captureDownloads()
    const fetchMock = stubPkgFetch(200)
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    await userEvent.click(
      await screen.findByRole('button', { name: 'Download for macOS' })
    )

    await waitFor(() => expect(clicks).toHaveLength(1))
    expect(fetchMock).toHaveBeenCalledWith(
      'https://deeprouter.example/d/TOKEN1/deeprouter-setup-TOKEN1-deeprouter.example.pkg'
    )
    expect(clicks[0].href).toBe('blob:mock-pkg')
    expect(clicks[0].download).toBe(
      'deeprouter-setup-TOKEN1-deeprouter.example.pkg'
    )
  })

  // A deployment whose CI has not shipped a signed artifact answers 404 for
  // the pkg. The button must stay honest during that window: say so, and hand
  // over the .sh instead of navigating to a bare "not found" page.
  it('falls back to the .sh when the pkg is not live on this server', async () => {
    onPlatform('MacIntel')
    const clicks = captureDownloads()
    stubPkgFetch(404)
    render(<ApiKeysOneClickSection tools={TOOLS} apiKey={KEY} />)

    await userEvent.click(
      await screen.findByRole('button', { name: 'Download for macOS' })
    )

    await waitFor(() => expect(clicks).toHaveLength(1))
    expect(clicks[0].href).toBe(
      'https://deeprouter.example/d/TOKEN1/deeprouter-setup.sh'
    )
    expect(toast.info).toHaveBeenCalled()
  })
})
