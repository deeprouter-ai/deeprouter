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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Check, Copy, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import {
  createApiKey,
  getApiKeys,
  issueConnectToken,
} from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import {
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
} from '@/features/keys/lib'
import { keyPermitsModel } from '@/features/keys/lib/model-limits'
import type { ApiKey } from '@/features/keys/types'
import { useStatus } from '@/hooks/use-status'
import {
  buildVideoPrompt,
  DEFAULT_VIDEO_MODEL,
  VIDEO_MODELS,
  type PromptLanguage,
} from './lib/prompt-template'

/**
 * The「可以做视频」page (Video First Wave P2, PRD D6): a student copies one
 * block of text and pastes it into their AI tool, which configures itself and
 * offers a first clip (paid, so the prompt has the agent ask before spending).
 * The student's only actions here are get-a-key (one click) and copy — model
 * choice moved into the prompt itself: the default is the cheapest model, and
 * switching is a sentence to the AI, not a page control (decided 2026-10-04,
 * the card picker read as jargon to the actual audience).
 *
 * What travels in the copied text is a one-time token URL, never the key —
 * the same `internal/connect` machinery the key page's one-click block uses
 * (Q10, decided 2026-09-15: token, not plaintext key). The token is minted on
 * page load and bound to one of the user's enabled keys.
 *
 * Which key that is, is said on the page and can be changed — the same fix the
 * key page's setup card got on 2026-08-28 after @sam asked which key the
 * buttons were configuring and the page could not answer. Here it bit again
 * (@sam, 2026-09-16): the prompt silently took the first enabled key, and a
 * user with several had no way to tell which one the project's `.env` ended up
 * holding. With one key there is nothing to decide, so the page states the name
 * instead of showing a control.
 *
 * 🔴 Naming Claude Code / Codex here is the same deliberate CLAUDE.md §0
 * exception the one-click block records: "paste it into your AI tool" cannot
 * be acted on without saying which tools qualify.
 */
export function VideoPage() {
  const { t, i18n } = useTranslation()

  const [keys, setKeys] = useState<ApiKey[]>([])
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null)
  const [keysLoaded, setKeysLoaded] = useState(false)
  const [scriptUrl, setScriptUrl] = useState('')
  const [issuing, setIssuing] = useState(false)
  const [creating, setCreating] = useState(false)
  const [copied, setCopied] = useState(false)
  const { status } = useStatus()

  // Only enabled keys are offered: a disabled one configures the project just
  // as happily and then answers 401 from inside the tool, with nothing here to
  // explain it. The list arrives newest-first.
  const loadKeys = useCallback(async () => {
    try {
      const res = await getApiKeys({ p: 1, size: 100 })
      setKeys(
        (res.data?.items ?? []).filter(
          (k) => k.status === API_KEY_STATUS.ENABLED
        )
      )
    } catch {
      setKeys([])
    } finally {
      setKeysLoaded(true)
    }
  }, [])

  useEffect(() => {
    void loadKeys()
  }, [loadKeys])

  // No model picker: every prompt embeds the cheapest model
  // (DEFAULT_VIDEO_MODEL), and the in-prompt menu teaches the alternatives —
  // switching is a sentence to the AI, not a page control.
  const model = DEFAULT_VIDEO_MODEL

  // A video key must be able to call EVERY model the prompt teaches (AC-G):
  // one that passes only some would break the moment the user switches by
  // voice. The one-click purpose key qualifies by construction.
  const videoKeys = useMemo(
    () =>
      keys.filter((k) => VIDEO_MODELS.every((m) => keyPermitsModel(k, m.id))),
    [keys]
  )

  // The binding is derived, not reconciled in an effect: `selectedKeyId`
  // records the user's pick, and the effective key falls back to the newest
  // video key whenever that pick is absent (e.g. just deleted elsewhere).
  const apiKey = useMemo(
    () => videoKeys.find((k) => k.id === selectedKeyId) ?? videoKeys[0] ?? null,
    [videoKeys, selectedKeyId]
  )

  // Mint a one-time redeem link for the bound key. The tools list only
  // matters to someone who *runs* the redeemed script; the prompt has the
  // agent read two variables out of its env form instead.
  const mintScriptUrl = useCallback(async (): Promise<string> => {
    if (!apiKey) return ''
    const res = await issueConnectToken(apiKey.id, ['claude-code'])
    if (!res.success || !res.data) return ''
    const base = res.data.base_url.replace(/\/+$/, '')
    return `${base}${res.data.script_path}`
  }, [apiKey])

  // Mint ONE preview link when the page first has a key — not on every key
  // switch. Minting is a CriticalRateLimit endpoint (20 / 20 min); binding it
  // to row selection burned the whole budget in a few clicks (429, measured
  // 2026-10-04). The preview token is not key-identifying anyway, and every
  // copy re-mints for the key actually selected (handleCopy), so switching
  // rows needs no server round-trip. `hadKey` flips true only once a preview
  // link lands, so a failed or cancelled first attempt retries.
  const hadKey = useRef(false)
  useEffect(() => {
    if (!apiKey) {
      hadKey.current = false
      setScriptUrl('')
      return
    }
    if (hadKey.current) return
    let cancelled = false
    setIssuing(true)
    void (async () => {
      try {
        const url = await mintScriptUrl()
        if (!cancelled) {
          setScriptUrl(url)
          if (url) hadKey.current = true
        }
      } catch {
        if (!cancelled) setScriptUrl('')
      } finally {
        if (!cancelled) setIssuing(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apiKey, mintScriptUrl])

  // The prompt follows the UI locale: an English-mode user must not be handed
  // a block of Chinese they cannot read (and vice versa).
  const promptLanguage: PromptLanguage = i18n.language?.startsWith('zh')
    ? 'zh'
    : 'en'

  const prompt = useMemo(
    () =>
      scriptUrl
        ? buildVideoPrompt({ scriptUrl, model, language: promptLanguage })
        : '',
    [scriptUrl, model, promptLanguage]
  )

  const handleCopy = async () => {
    if (!prompt) return
    // Re-mint on every copy: the link in `prompt` is one-shot, so a previous
    // paste (or any fetch of it) already killed it, and a page left open has
    // outlived the 30-minute TTL. If minting fails, fall back to the shown
    // prompt — its own step-1 text tells the agent how to recover.
    let text = prompt
    try {
      const fresh = await mintScriptUrl()
      if (fresh) {
        setScriptUrl(fresh)
        text = buildVideoPrompt({
          scriptUrl: fresh,
          model,
          language: promptLanguage,
        })
      }
    } catch {
      // keep the currently displayed prompt
    }
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  // One-click create: a Simple key with the video purpose — the backend
  // derives its whitelist from the purpose registry, which covers every
  // video model (P5). Payload mirrors the keys drawer exactly.
  const handleCreateKey = async () => {
    setCreating(true)
    try {
      const payload = transformFormDataToPayload({
        ...getApiKeyFormDefaultValues(
          status?.default_use_auto_group === true,
          'simple'
        ),
        simple_purpose: 'video',
      })
      if (videoKeys.length > 0) {
        // Same suffix scheme as the drawer's batch create, so repeat clicks
        // stay tellable apart on the keys page.
        payload.name = `my-video-key-${Math.random().toString(36).slice(2, 8)}`
      }
      const result = await createApiKey(payload)
      if (result.success) {
        toast.success(t('Video key created'))
        if (result.data?.id) setSelectedKeyId(result.data.id)
        await loadKeys()
      } else {
        toast.error(result.message || t('Could not create the key'))
      }
    } catch {
      toast.error(t('Could not create the key'))
    } finally {
      setCreating(false)
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Make videos')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t(
          'Copy one block of text, paste it into an AI coding tool on your computer (Claude Code or Codex) — it sets itself up and makes your first clip. After that, just say "生成视频".'
        )}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='space-y-6'>
          {/* Step 1 — the video key. One click mints a Simple video-purpose
              key (backend derives a whitelist covering every video model);
              below it, every existing video-capable key, keys-page style, so
              which key the prompt configures is always said out loud. */}
          <section>
            <h3 className='text-sm font-semibold'>{t('1. Your video key')}</h3>
            <div className='mt-3 space-y-3'>
              <Button
                type='button'
                onClick={handleCreateKey}
                disabled={creating || !keysLoaded}
                className='bg-accent text-accent-foreground hover:bg-accent/90'
              >
                <Plus className='h-4 w-4' />
                {creating ? t('Creating...') : t('Create a video key')}
              </Button>

              {keysLoaded && videoKeys.length === 0 ? (
                <p className='border-border text-muted-foreground rounded-[7px] border border-dashed px-4 py-6 text-sm'>
                  {t('No video key yet — the button above makes one in a second.')}
                </p>
              ) : (
                videoKeys.length > 0 && (
                  <div className='border-border bg-card rounded-[7px] border'>
                    {videoKeys.map((k) => {
                      const selected = apiKey?.id === k.id
                      return (
                        <button
                          key={k.id}
                          type='button'
                          onClick={() => setSelectedKeyId(k.id)}
                          aria-pressed={selected}
                          className='border-border hover:bg-muted/40 flex w-full items-center justify-between gap-3 border-b px-4 py-2.5 text-left last:border-b-0'
                        >
                          <span className='truncate text-sm font-medium'>
                            {k.name}
                          </span>
                          {/* Forward-looking wording on purpose: "in use"
                              read as a live connection, as if clicking rows
                              re-pointed projects configured earlier. This
                              selection only shapes the NEXT copied text. */}
                          {selected && (
                            <span className='inline-flex shrink-0 items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-sm font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'>
                              <Check className='h-4 w-4' />
                              {t('Selected')}
                            </span>
                          )}
                        </button>
                      )
                    })}
                  </div>
                )
              )}

              {videoKeys.length > 1 && (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Switching rows only changes future copies — a project you already set up keeps its key until you paste a new text there.'
                  )}
                </p>
              )}

              <p className='text-muted-foreground text-xs'>
                {t(
                  'A video key can call every video model. Each clip is billed by the model used — the AI quotes the price before generating.'
                )}{' '}
                <Link
                  to='/keys'
                  className='text-foreground underline underline-offset-2'
                >
                  {t('Manage keys')}
                </Link>
              </p>
            </div>
          </section>

          {/* Step 2 — the paste-prompt. */}
          <section>
            <h3 className='text-sm font-semibold'>
              {t(
                '2. Copy this, paste it into Claude Code or Codex, press Enter'
              )}
            </h3>

            {keysLoaded && !apiKey ? (
              <p className='border-border text-muted-foreground mt-3 rounded-[7px] border border-dashed px-4 py-6 text-sm'>
                {t(
                  'Create a video key above first — the text to copy appears here.'
                )}
              </p>
            ) : (
              <>
                {/* No "limited key" warning and no picker here: only keys
                    that can run every video model are listed at all, and the
                    panel above says which one is in use. */}
                <div className='border-border bg-card mt-3 rounded-[7px] border'>
                  <pre className='max-h-72 overflow-auto p-4 text-xs leading-5 whitespace-pre-wrap'>
                    {prompt || (issuing ? t('Preparing…') : '')}
                  </pre>
                  <div className='border-border flex items-center justify-between gap-3 border-t px-4 py-3'>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Every copy carries a fresh link — valid for 30 minutes, usable once. Your key itself is not in this text.'
                      )}
                    </p>
                    <Button
                      type='button'
                      onClick={handleCopy}
                      disabled={!prompt}
                      className='bg-accent text-accent-foreground hover:bg-accent/90 shrink-0'
                    >
                      {copied ? (
                        <Check className='h-4 w-4' />
                      ) : (
                        <Copy className='h-4 w-4' />
                      )}
                      {copied ? t('Copied') : t('Copy')}
                    </Button>
                  </div>
                </div>
                <p className='text-muted-foreground mt-2 text-xs'>
                  {t(
                    'You only paste this once per project. At the end, the AI asks before making a small paid test clip — skipping it costs nothing.'
                  )}
                </p>
              </>
            )}
          </section>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
