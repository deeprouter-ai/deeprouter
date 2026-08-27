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
import { Check, Copy, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getApiKeys, getConnectTools, issueConnectToken } from '../api'
import type { ApiKey, ConnectTool } from '../types'
import { useApiKeys } from './api-keys-provider'

/** Where the script template lives, for the "read it first" link (PRD §5.3). */
const SCRIPT_SOURCE_URL =
  'https://github.com/deeprouter-ai/deeprouter/tree/main/internal/connect'

/**
 * One-click setup block for terminal AI tools.
 *
 * The user ticks what they have installed, copies one line, and pastes it into
 * a terminal. What travels in that line is a one-time token, never the key:
 * commands get copied, screenshotted and pasted into group chats, and a token
 * that dies after one use or fifteen minutes is worth nothing to whoever finds
 * it. The key is injected server-side when the script is fetched.
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
export function ApiKeysOneClickCard() {
  const { t } = useTranslation()
  const { refreshTrigger } = useApiKeys()

  const [tools, setTools] = useState<ConnectTool[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [keyId, setKeyId] = useState<number | null>(null)
  const [scriptUrl, setScriptUrl] = useState('')
  const [issuing, setIssuing] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)

  const apiKey = useMemo(
    () => keys.find((k) => k.id === keyId) ?? null,
    [keys, keyId]
  )

  // `label` is what the closed trigger renders, so the restriction hint has to
  // be baked into the string there; inside the open list it is a separate muted
  // span, which reads better but cannot be reused for the trigger.
  const keyOptions = useMemo(
    () =>
      keys.map((key) => ({
        value: String(key.id),
        name: key.name,
        limited: key.model_limits_enabled,
        label: key.model_limits_enabled
          ? `${key.name} ${t('(limited to some models)')}`
          : key.name,
      })),
    [keys, t]
  )

  const isWindows = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /win/i.test(navigator.platform || navigator.userAgent || '')
  }, [])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const res = await getConnectTools()
        if (cancelled || !res.success || !res.data) return
        setTools(res.data)
        // Default to everything ticked: someone who does not want to think
        // should be able to copy and go. Still visible, still changeable.
        setSelected(res.data.map((tool) => tool.id))
      } catch {
        /* leave the block hidden rather than showing a broken control */
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        // The list comes back newest-first, which is also the default pick:
        // creating a key and then setting it up is the common path.
        const res = await getApiKeys({ p: 1, size: 50 })
        if (cancelled) return
        const items = res.data?.items ?? []
        setKeys(items)
        setKeyId((prev) =>
          prev !== null && items.some((k) => k.id === prev)
            ? prev
            : (items[0]?.id ?? null)
        )
      } catch {
        if (!cancelled) {
          setKeys([])
          setKeyId(null)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [refreshTrigger])

  // Re-issue whenever the key or the tool selection changes: the token carries
  // both, so a stale command would set up the wrong thing. Older tokens are
  // left to expire on their own rather than being revoked (PRD §4.1).
  useEffect(() => {
    if (!apiKey || selected.length === 0) {
      setScriptUrl('')
      return
    }
    let cancelled = false
    setIssuing(true)
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
      } catch {
        if (!cancelled) setScriptUrl('')
      } finally {
        if (!cancelled) setIssuing(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apiKey, selected])

  const toggle = useCallback((id: string) => {
    setSelected((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    )
  }, [])

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

  // No key yet means there is nothing to configure; the create flow above is
  // the right next step, and a disabled block here would only add noise.
  if (!apiKey || tools.length === 0) return null

  const selectedNames = tools
    .filter((tool) => selected.includes(tool.id))
    .map((tool) => tool.name)

  const windowsCommand = `irm ${scriptUrl} | iex`
  const posixCommand = `curl -fsSL ${scriptUrl} | sh`

  // Same address as the one-liner, split so the script can be read before it
  // runs. Windows has no `less`, so it gets its own pair.
  const twoStepCommand = isWindows
    ? [
        `irm ${scriptUrl} -OutFile dr-setup.ps1`,
        `notepad dr-setup.ps1`,
        `.\\dr-setup.ps1`,
      ].join('\n')
    : [
        `curl -fsSL ${scriptUrl} -o dr-setup.sh`,
        `less dr-setup.sh`,
        `sh dr-setup.sh`,
      ].join('\n')

  const commandRow = (id: string, label: string, value: string) => (
    <div key={id}>
      <p className='text-muted-foreground text-[11px]'>{label}</p>
      <div className='border-border bg-background mt-1 flex items-center gap-2 rounded-md border px-2 py-1.5'>
        <code className='flex-1 truncate font-mono text-[11px]' title={value}>
          {issuing && !scriptUrl ? t('Preparing…') : value}
        </code>
        <Button
          type='button'
          size='sm'
          variant='ghost'
          className='h-6 px-1.5'
          onClick={() => handleCopy(value, id)}
          disabled={!scriptUrl}
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

  return (
    <div className='bg-muted/30 mb-4 rounded-lg border p-4 sm:p-5'>
      <h3 className='text-sm font-semibold'>{t('One-click setup')}</h3>
      <p className='text-muted-foreground mt-1 text-xs'>
        {t(
          'Already using an AI tool in your terminal? Tick it below, then paste one line — no settings to find, nothing to type by hand.'
        )}
      </p>

      {/* Which key this configures. With one key there is nothing to decide,
          so the row is not shown at all; with several, leaving it implicit
          would bake whichever sorts first into the command and only surface
          as a 403 inside some tool days later. */}
      {keys.length > 1 && (
        <div className='mt-3'>
          <label htmlFor='one-click-key' className='text-xs font-medium'>
            {t('Key to set up')}
          </label>
          {/* Not a native <select>: its popup is drawn by the OS and ignores
              the app's theme, so it showed up as a white list on a dark page. */}
          <Select
            items={keyOptions}
            value={String(keyId ?? '')}
            onValueChange={(v) => v !== null && setKeyId(Number(v))}
          >
            <SelectTrigger
              id='one-click-key'
              className='mt-1.5 w-full text-xs sm:max-w-sm'
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {keyOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    <span className='truncate'>{option.name}</span>
                    {option.limited && (
                      <span className='text-muted-foreground ml-1.5'>
                        {t('(limited to some models)')}
                      </span>
                    )}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
      )}

      {/* A key restricted to a few models is a legitimate thing to own and a
          poor thing to configure four tools with — say so here rather than
          letting it turn into "model not allowed" inside the tool. */}
      {apiKey.model_limits_enabled && (
        <p className='text-muted-foreground mt-2 flex items-start gap-1.5 text-[11px]'>
          <TriangleAlert className='mt-px h-3 w-3 shrink-0' />
          <span>
            {t(
              'This key is limited to certain models. Tools configured with it will only work for those — pick an unrestricted key if you want everything to work.'
            )}
          </span>
        </p>
      )}

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
        <p className='text-xs font-medium'>
          {t('2. Open your terminal, paste the line for your system, press Enter')}
        </p>

        {selected.length === 0 ? (
          <p className='border-border text-muted-foreground mt-2 rounded-md border border-dashed px-3 py-4 text-center text-xs'>
            {t('Pick at least one tool above to get your command.')}
          </p>
        ) : (
          <>
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
              {t('Valid for 15 minutes')}
            </p>
          </>
        )}

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

        {/* Piping a downloaded script into a shell is a fair thing to be wary
            of. rustup, bun and homebrew all offer a way to read it first, and
            not offering one just pushes cautious users away. The review link
            shows the template; the two-step form shows the exact bytes that
            will run, key and all — which the template alone cannot. */}
        {selected.length > 0 && scriptUrl && (
          <details className='mt-3'>
            <summary className='text-muted-foreground hover:text-foreground cursor-pointer text-[11px]'>
              {t('Want to read the script first?')}
            </summary>
            <div className='mt-2 space-y-2'>
              <p className='text-muted-foreground text-[11px]'>
                {t('See the template on GitHub:')}{' '}
                <a
                  href={SCRIPT_SOURCE_URL}
                  target='_blank'
                  rel='noreferrer noopener'
                  className='underline underline-offset-2'
                >
                  internal/connect/
                </a>
              </p>
              <p className='text-muted-foreground text-[11px]'>
                {t(
                  'Or download it, read it, then run it — same address as above:'
                )}
              </p>
              <pre className='border-border bg-background overflow-x-auto rounded-md border px-2 py-1.5 font-mono text-[11px]'>
                {twoStepCommand}
              </pre>
            </div>
          </details>
        )}
      </div>
    </div>
  )
}
