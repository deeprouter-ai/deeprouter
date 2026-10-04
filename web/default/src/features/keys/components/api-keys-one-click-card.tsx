/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Check, Copy, Download, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { issueConnectToken } from '../api'
import type { ApiKey, ConnectTool } from '../types'

/** Where the script template lives, for the "read it first" link (PRD §5.3). */
const SCRIPT_SOURCE_URL =
  'https://github.com/deeprouter-ai/deeprouter/tree/main/internal/connect/templates'

/**
 * The downloadable installers (PRD §11). Served by `GET /d/<token>/<file>`;
 * the names must match `internal/connect/installer.go`, which is also what the
 * file ends up called on disk. The macOS `.pkg` has no constant here: its
 * filename is built per download — the token and the issuing host ride in the
 * NAME because the signature freezes the body (PRD §11.7).
 */
const INSTALLER_WINDOWS = 'deeprouter-setup.cmd'
const INSTALLER_UNIX = 'deeprouter-setup.sh'

/**
 * This instance's own address, for the undo commands — same pattern as
 * `data-table-row-actions.tsx`. Undo needs no token (PRD §4.6), so it must
 * not depend on one having been minted.
 */
function getServerAddress(): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return status.server_address as string
    }
  } catch {
    /* empty */
  }
  return window.location.origin
}

/**
 * POST /api/connect/token sits behind CriticalRateLimit (~20 requests per 20
 * minutes per IP). "Please try again" is exactly the wrong advice for that
 * failure — retrying digs the hole deeper — so it gets its own message.
 * Measured live on 2026-09-29: a curious user clicking around the page burned
 * the whole budget and every download after that failed with a generic toast.
 */
function isRateLimited(error: unknown): boolean {
  return (error as { response?: { status?: number } })?.response?.status === 429
}

/**
 * One-click setup section for terminal AI tools.
 *
 * A section of `ApiKeysSetupCard`, not a card of its own: the key it
 * configures is chosen once for the whole box, so it arrives as a prop rather
 * than being picked here.
 *
 * The user ticks what they have installed and then takes either of two roads:
 * download a file and open it, or copy one line into a terminal. What travels
 * in both is a one-time token, never the key: commands get copied,
 * screenshotted and pasted into group chats, downloaded files sit in Downloads
 * folders and get synced and forwarded, and a token that dies after one use or
 * thirty minutes is worth nothing to whoever finds it. The key is injected
 * server-side when the script is fetched.
 *
 * The download road exists because for the people this product is for, the
 * terminal is the wall (PRD §11, decided 2026-09-12): on Windows the file is
 * double-clicked and nothing is ever typed. It is the primary road; the
 * command road survives collapsed under "Setup didn't work? Try a terminal
 * command" (@sam, 2026-09-29) — always open, it read as a second thing the
 * user was supposed to do. Terminal people open it and everything is as it
 * was.
 *
 * The tool selection lives in the token rather than in command-line flags
 * because the two platforms are not symmetric: `curl … | sh -s -- codex` is
 * natural, while PowerShell's `irm … | iex` takes no arguments at all and would
 * need `& ([scriptblock]::Create((irm …))) -Only codex`. Putting the choice on
 * the page keeps one command shape for both, and the page is the only place
 * with room to explain what is about to happen (PRD §2.1).
 *
 * Both commands are shown, with the detected platform first. Detection is a
 * guess about the browser, not about the machine being set up: a Windows user
 * working in WSL or Git Bash has no `irm`, and someone may well be browsing on
 * one machine to configure another. Guessing wrong is not dangerous — the
 * other shell answers "command not found", the key does not leak and the token
 * survives — but with one command on screen there is nothing to fall back to.
 *
 * 🔴 Named third-party tools here are a deliberate exception to CLAUDE.md §0
 * Rule 1, which keeps client brand names off casual surfaces. Decided by @sam
 * on 2026-08-27: the checkboxes ask "which of these do you have installed?",
 * and that question cannot be asked without naming them — "your terminal AI
 * tool" is not something a user can match against what is on their machine.
 * The rule's purpose is to stop jargon leaking into surfaces where a plain
 * word would do; here there is no plain word. This block is also not gated by
 * persona for the same reason: someone who has Claude Code installed is a user
 * of it whatever persona the console assigned them.
 */
export function ApiKeysOneClickSection({
  tools,
  apiKey,
}: {
  tools: ConnectTool[]
  apiKey: ApiKey
}) {
  const { t } = useTranslation()

  // `null` means the user has not touched the list, which reads as "all of
  // them": someone who does not want to think should be able to copy and go.
  // Derived rather than seeded from an effect so a late `tools` needs no
  // second render to show the right ticks.
  const [picked, setPicked] = useState<string[] | null>(null)
  const [scriptUrl, setScriptUrl] = useState('')
  const [copied, setCopied] = useState<string | null>(null)
  const [downloading, setDownloading] = useState<string | null>(null)
  // Whether the collapsed command fallback is open — minting is gated on it.
  const [commandsOpen, setCommandsOpen] = useState(false)

  const isWindows = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /win/i.test(navigator.platform || navigator.userAgent || '')
  }, [])
  const isMac = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /mac/i.test(navigator.platform || navigator.userAgent || '')
  }, [])

  const selected = useMemo(
    () => picked ?? tools.map((tool) => tool.id),
    [picked, tools]
  )

  // Mint ONLY while the command fallback is open, re-issuing when the key or
  // the tool selection changes (the token carries both, so a stale command
  // would set up the wrong thing; older tokens just expire, PRD §4.1).
  //
  // 🔴 Not on page load, and not while collapsed. The issuing endpoint sits
  // behind CriticalRateLimit (~20 per 20 min), and the old shape — mint on
  // load, re-mint on every checkbox — spent that budget on renders nobody
  // read: a curious user clicking around burned it dry, and every download
  // after that failed (measured 2026-09-29). Now a page load costs zero
  // mints; opening the fallback or clicking a download costs one.
  useEffect(() => {
    if (!commandsOpen || !apiKey || selected.length === 0) {
      setScriptUrl('')
      return
    }
    let cancelled = false
    void (async () => {
      try {
        const res = await issueConnectToken(apiKey.id, selected)
        if (cancelled) return
        if (!res.success || !res.data) {
          setScriptUrl('')
          return
        }
        const { base_url, script_path } = res.data
        setScriptUrl(`${base_url.replace(/\/+$/, '')}${script_path}`)
      } catch (error) {
        if (cancelled) return
        setScriptUrl('')
        if (isRateLimited(error)) {
          toast.error(t('Too many requests — wait a minute, then try again.'))
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apiKey, selected, commandsOpen, t])

  const toggle = useCallback(
    (id: string) => {
      setPicked((prev) => {
        const base = prev ?? tools.map((tool) => tool.id)
        return base.includes(id) ? base.filter((x) => x !== id) : [...base, id]
      })
    },
    [tools]
  )

  /**
   * Mint a FRESH token, then hand the browser the installer built from it.
   *
   * 🔴 The re-issue is the point. The token on this page was minted when the
   * page loaded, and it lives 30 minutes; somebody who reads the whole section
   * first and downloads twenty minutes later would get a file with ten minutes
   * left in it — and nothing about the file says so. The failure that produces
   * is "I downloaded it and it says expired", which is both baffling and nearly
   * impossible to reproduce. Issuing on the click starts the clock when the
   * file reaches the user. The page's own command keeps working: old tokens are
   * left to expire rather than being revoked (PRD §4.1).
   */
  const handleDownload = useCallback(
    async (file: string) => {
      if (!apiKey || selected.length === 0) return
      setDownloading(file)
      try {
        const res = await issueConnectToken(apiKey.id, selected)
        if (!res.success || !res.data) {
          toast.error(t('Could not prepare your file. Please try again.'))
          return
        }
        const base = res.data.base_url.replace(/\/+$/, '')
        // An anchor rather than location.href: the response is an attachment,
        // so this downloads without the page navigating anywhere, and it does
        // not need the filename repeated — Content-Disposition names the file.
        const link = document.createElement('a')
        link.href = `${base}/d/${res.data.token}/${file}`
        link.rel = 'noopener'
        link.click()
      } catch (error) {
        toast.error(
          isRateLimited(error)
            ? t('Too many requests — wait a minute, then try again.')
            : t('Could not prepare your file. Please try again.')
        )
      } finally {
        setDownloading(null)
      }
    },
    [apiKey, selected, t]
  )

  /**
   * The macOS road: the signed `.pkg` (PRD §11.7). Same fresh-token rule as
   * above, but the filename is the data channel — the token and the issuing
   * host travel in the NAME, because the signature freezes the body — so the
   * page builds it and fetches the bytes itself (the saved name must be
   * right even for a client that ignores Content-Disposition).
   *
   * A deployment whose CI has not shipped a signed artifact yet answers 404
   * there; falling back to the `.sh` keeps the button honest during that
   * window instead of navigating the user to a bare "not found" page. Only
   * SIGNED artifacts ever reach production (deploy.yml) — an unsigned pkg
   * would be Gatekeeper-blocked on double-click, which is worse than dark.
   */
  const handleDownloadPkg = useCallback(async () => {
    if (!apiKey || selected.length === 0) return
    setDownloading('pkg')
    try {
      const res = await issueConnectToken(apiKey.id, selected)
      if (!res.success || !res.data) {
        toast.error(t('Could not prepare your file. Please try again.'))
        return
      }
      const base = res.data.base_url.replace(/\/+$/, '')
      // ':' cannot live in a macOS filename, so ports are spelled with '_' —
      // mirror of PkgFileName in internal/connect/installer_pkg.go.
      const host = new URL(base).host.replace(/:/g, '_')
      const name = `deeprouter-setup-${res.data.token}-${host}.pkg`
      const resp = await fetch(`${base}/d/${res.data.token}/${name}`)
      if (!resp.ok) {
        toast.info(
          t(
            'The macOS installer is not live on this server yet — downloading the script version instead.'
          )
        )
        const link = document.createElement('a')
        link.href = `${base}/d/${res.data.token}/${INSTALLER_UNIX}`
        link.rel = 'noopener'
        link.click()
        return
      }
      const blob = await resp.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = name
      link.rel = 'noopener'
      link.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      toast.error(
        isRateLimited(error)
          ? t('Too many requests — wait a minute, then try again.')
          : t('Could not prepare your file. Please try again.')
      )
    } finally {
      setDownloading(null)
    }
  }, [apiKey, selected, t])

  const handleCopy = useCallback(
    async (value: string, id: string) => {
      try {
        await navigator.clipboard.writeText(value)
        setCopied(id)
        window.setTimeout(() => setCopied(null), 1500)
      } catch {
        toast.error(t('Copy failed'))
      }
    },
    [t]
  )

  if (tools.length === 0) return null

  const selectedNames = tools
    .filter((tool) => selected.includes(tool.id))
    .map((tool) => tool.name)

  const windowsCommand = `irm ${scriptUrl} | iex`
  const posixCommand = `curl -fsSL ${scriptUrl} | sh`

  // From this instance's own address, never from a minted token: undo needs
  // no token (PRD §4.6), so it must stay visible and copyable even when
  // nothing has been minted. Both platforms, detected one first — the
  // detection describes the browser, not the machine that was set up.
  const serverBase = getServerAddress().replace(/\/+$/, '')
  const uninstallWindows = `irm ${serverBase}/uninstall | iex`
  const uninstallPosix = `curl -fsSL ${serverBase}/uninstall | sh`

  // `ready` is false only for the install rows before their token has been
  // minted (the mint starts when the fallback opens); the undo rows carry no
  // token and are always ready.
  const commandRow = (
    id: string,
    label: string,
    value: string,
    ready = !!scriptUrl
  ) => (
    <div key={id}>
      <p className='text-muted-foreground text-[11px]'>{label}</p>
      <div className='border-border bg-background mt-1 flex items-center gap-2 rounded-md border px-2 py-1.5'>
        <code className='flex-1 truncate font-mono text-[11px]' title={value}>
          {ready ? value : t('Preparing…')}
        </code>
        <Button
          type='button'
          size='sm'
          variant='ghost'
          className='h-6 px-1.5'
          onClick={() => handleCopy(value, id)}
          disabled={!ready}
          aria-label={t('Copy command')}
        >
          {copied === id ? (
            <Check className='h-3 w-3' />
          ) : (
            <Copy className='h-3 w-3' />
          )}
        </Button>
      </div>
    </div>
  )

  const windowsRow = commandRow(
    'win',
    t('Windows — PowerShell or Terminal'),
    windowsCommand
  )
  const posixRow = commandRow(
    'posix',
    t('macOS / Linux — Terminal (also WSL and Git Bash)'),
    posixCommand
  )
  const uninstallWindowsRow = commandRow(
    'un-win',
    t('Windows — PowerShell or Terminal'),
    uninstallWindows,
    true
  )
  const uninstallPosixRow = commandRow(
    'un-posix',
    t('macOS / Linux — Terminal (also WSL and Git Bash)'),
    uninstallPosix,
    true
  )

  // Which button is the filled one. The detection describes the browser, and
  // that is fine here: it only picks the emphasis, all three are always shown.
  const primaryId = isWindows ? 'cmd' : isMac ? 'pkg' : 'sh'

  const downloadButton = (id: string, label: string, onClick: () => void) => (
    <Button
      type='button'
      size='sm'
      variant={primaryId === id ? 'default' : 'outline'}
      className='h-7 text-xs'
      onClick={onClick}
      disabled={downloading !== null}
    >
      <Download className='h-3 w-3' />
      {downloading === id ? t('Preparing your file…') : label}
    </Button>
  )

  const windowsDownload = (
    <div key='dl-win'>
      {downloadButton(
        'cmd',
        t('Download for Windows'),
        () => void handleDownload(INSTALLER_WINDOWS)
      )}
      {/* 🔴 Said BEFORE the click, never after. A file from the internet makes
          Windows put up a "publisher unknown" warning, and the default reaction
          of a non-technical user to that sentence is to close it and ask
          support — or give up. Forewarned, the very same dialog reads as "just
          like it said" (PRD §7 Track C). Which dialog appears varies by machine
          (plain security warning vs SmartScreen), so the copy names both
          buttons; P7 confirms the wording on real machines. */}
      <p className='text-muted-foreground mt-1.5 flex items-start gap-1.5 text-[11px]'>
        <TriangleAlert className='mt-px h-3 w-3 shrink-0' />
        <span>
          {t(
            'Windows will warn that the publisher is unknown — click “Run” (or “More info” → “Run anyway”). You never have to type anything, and the window stays open at the end so you can read what it did.'
          )}
        </span>
      </p>
    </div>
  )

  // macOS gets the signed .pkg — a double-click walks the system's own
  // install wizard, no terminal, no warning (that is what the signature and
  // notarization buy). Switched from the combined "macOS / Linux" .sh button
  // on 2026-09-29 (@sam, boss holds the signing credentials).
  const macDownload = (
    <div key='dl-mac'>
      {downloadButton(
        'pkg',
        t('Download for macOS'),
        () => void handleDownloadPkg()
      )}
    </div>
  )

  // 🔴 No "then run this line in Terminal" under the button, by decision
  // (@sam relaying the boss, 2026-09-29): the download road promises no
  // terminal, and a terminal line printed underneath contradicts that
  // promise. The .sh must still be run as `bash <file>` — the instruction
  // lives in the file's own header comment, and Linux users mostly take the
  // copy-a-command road anyway.
  const linuxDownload = (
    <div key='dl-linux'>
      {downloadButton(
        'sh',
        t('Download for Linux'),
        () => void handleDownload(INSTALLER_UNIX)
      )}
    </div>
  )

  return (
    <section className='border-border mt-4 border-t pt-4'>
      <h4 className='text-xs font-semibold'>{t('Terminal tools')}</h4>

      <fieldset className='mt-3'>
        <legend className='text-xs font-medium'>
          {t('1. Choose what to set up')}
        </legend>
        <div className='mt-2 grid gap-2 sm:grid-cols-2'>
          {tools.map((tool) => (
            <label
              key={tool.id}
              className='border-border bg-background flex cursor-pointer items-center gap-2 rounded-md border px-2.5 py-2 text-xs'
            >
              <Checkbox
                checked={selected.includes(tool.id)}
                onCheckedChange={() => toggle(tool.id)}
                aria-label={tool.name}
              />
              <span className='font-medium'>{tool.name}</span>
            </label>
          ))}
        </div>
        <p className='text-muted-foreground mt-2 text-[11px]'>
          {t(
            'Only what you tick AND actually have installed gets configured. Anything missing is skipped and reported.'
          )}
        </p>
      </fieldset>

      <div className='mt-4'>
        <p className='text-xs font-medium'>{t('2. Download and install')}</p>

        {selected.length === 0 ? (
          <p className='border-border text-muted-foreground mt-2 rounded-md border border-dashed px-3 py-4 text-center text-xs'>
            {t('Pick at least one tool above to get your command.')}
          </p>
        ) : (
          <>
            {/* The download road first: on Windows it is the only one of the two
                that requires no terminal and no typing at all, and that is the
                whole reason this form factor exists (PRD §11). Detected platform
                first here too, same reasoning as the commands below. */}
            <div className='border-border mt-2 rounded-md border p-2.5'>
              <p className='text-xs font-medium'>
                {t('Download a file and open it')}
              </p>
              <div className='mt-2 flex flex-col gap-3'>
                {isWindows
                  ? [windowsDownload, macDownload, linuxDownload]
                  : isMac
                    ? [macDownload, windowsDownload, linuxDownload]
                    : [linuxDownload, windowsDownload, macDownload]}
              </div>
            </div>

            {/* The command road, demoted from a parallel option to a
                failure fallback (@sam, 2026-09-29): for the audience this
                block exists for, the download IS the path, and an always-open
                command block read as a second thing they were supposed to do.
                Collapsed, it stops competing — and the summary names the one
                moment it should be opened. Terminal people still see it. */}
            <details
              className='mt-3'
              onToggle={(e) => setCommandsOpen(e.currentTarget.open)}
            >
              <summary className='cursor-pointer text-xs font-semibold'>
                {t('Setup didn’t work? Try a terminal command')}
              </summary>

              {/* Detected platform first, but both are always present: the
                  detection describes the browser, not necessarily the machine
                  being set up. */}
              <div className='mt-2 space-y-2'>
                {isWindows ? [windowsRow, posixRow] : [posixRow, windowsRow]}
              </div>

              {/* `curl | sh` earns its scepticism, so say what it will touch
                  right next to the command instead of in a help page. */}
              <p className='text-muted-foreground mt-1.5 text-[11px]'>
                {t('This command will set up:')} {selectedNames.join('、')}
                {' · '}
                {t('Valid for 30 minutes')}
              </p>

              {isWindows && (
                <p className='text-muted-foreground mt-2 flex items-start gap-1.5 text-[11px]'>
                  <TriangleAlert className='mt-px h-3 w-3 shrink-0' />
                  <span>
                    {t(
                      'Press Win + X and choose Terminal or Windows PowerShell. Do not use Command Prompt (cmd) — neither command works there.'
                    )}
                  </span>
                </p>
              )}
            </details>
          </>
        )}

        {/* What to type once the script has finished. The script prints the
            same table, but terminal output scrolls away and gets closed — a
            real user typed plain `codex` an hour after installing, met the
            ChatGPT login screen, and read it as "setup failed" (2026-08-28).
            The page cannot know whether their machine already had a Codex
            config (that decides plain `codex` vs `--profile deeprouter`), so
            the Codex row teaches the check instead of the answer.
            Collapsed by default like the script block below (@sam
            2026-08-28): the bold summary is the part that must be
            unmissable, not the table itself. */}
        {selected.length > 0 && (
          <details className='mt-4'>
            <summary className='cursor-pointer text-xs font-semibold'>
              {t('After it finishes — how to open each tool')}
            </summary>
            <div className='border-border mt-2 divide-y rounded-md border'>
              {tools
                .filter((tool) => selected.includes(tool.id))
                .map((tool) => {
                  const guide: Record<
                    string,
                    { cmd: string; note: string; extraCmd?: string }
                  > = {
                    opencode: {
                      cmd: 'opencode',
                      note: t('Works right away — nothing to reopen.'),
                    },
                    'claude-code': {
                      cmd: 'claude',
                      note: t('New terminal first.'),
                    },
                    'gemini-cli': {
                      cmd: 'gemini',
                      note: t(
                        'New terminal first. If it asks how to sign in, pick "API key".'
                      ),
                    },
                    codex: {
                      cmd: 'codex',
                      note: t(
                        'New terminal first. If it still asks you to log in to ChatGPT, use this instead:'
                      ),
                      extraCmd: 'codex --profile deeprouter',
                    },
                  }
                  const row = guide[tool.id]
                  if (!row) return null
                  return (
                    <div
                      key={tool.id}
                      className='flex flex-wrap items-baseline gap-x-3 gap-y-1 px-3 py-2'
                    >
                      <span className='w-24 shrink-0 text-xs font-medium'>
                        {tool.name}
                      </span>
                      <code className='bg-background border-border rounded border px-1.5 py-0.5 font-mono text-[11px]'>
                        {row.cmd}
                      </code>
                      <span className='text-muted-foreground text-[11px]'>
                        {row.note}
                      </span>
                      {row.extraCmd && (
                        <code className='bg-background border-border rounded border px-1.5 py-0.5 font-mono text-[11px]'>
                          {row.extraCmd}
                        </code>
                      )}
                    </div>
                  )
                })}
            </div>
            <p className='text-muted-foreground mt-1.5 text-[11px]'>
              {t(
                '"New terminal first" means: close every terminal window, then open a fresh one. A terminal opened from inside an old one does not count.'
              )}
            </p>
          </details>
        )}

        {/* Two things a cautious user wants, behind one summary: how to take
            it back, and what the script actually does before running it.
            Undoing leads (@sam 2026-09-29, renamed from "Want to read the
            script first?") — it is the one people come looking for, and "I can
            undo it" is what makes a piped-shell command acceptable at all;
            rustup, nvm and homebrew all say it in the same breath (PRD §4.6).
            The template link stays below it: rustup, bun and homebrew all
            offer a read-it-first path, and not offering one pushes cautious
            users away. A download-read-run form used to sit here too; removed
            2026-08-28 (@sam: not useful) — PRD §5.4 records it, and the address
            still serves the script to anyone who saves it by hand. */}
        {selected.length > 0 && (
          <details className='mt-3'>
            <summary className='cursor-pointer text-xs font-semibold'>
              {t('Want to undo the setup?')}
            </summary>
            <div className='mt-2 space-y-2'>
              {/* Needs no token, so it is safe to show always, and both
                  platforms appear for the same reason the install commands do:
                  the browser is not necessarily the machine that was set up. */}
              <p className='text-muted-foreground text-[11px]'>
                {t('Changed your mind later? One line puts everything back:')}
              </p>
              {isWindows
                ? [uninstallWindowsRow, uninstallPosixRow]
                : [uninstallPosixRow, uninstallWindowsRow]}
              <p className='text-muted-foreground text-[11px]'>
                {t('See the template on GitHub:')}{' '}
                <a
                  href={SCRIPT_SOURCE_URL}
                  target='_blank'
                  rel='noreferrer noopener'
                  className='underline underline-offset-2'
                >
                  internal/connect/templates/
                </a>
              </p>
            </div>
          </details>
        )}
      </div>
    </section>
  )
}
