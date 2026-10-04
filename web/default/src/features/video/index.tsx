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
import { useEffect, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { SectionPageLayout } from '@/components/layout'
import { getApiKeys, issueConnectToken } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import { keyPermitsModel } from '@/features/keys/lib/model-limits'
import type { ApiKey } from '@/features/keys/types'
import {
  buildVideoPrompt,
  DEFAULT_VIDEO_MODEL,
  VIDEO_MODELS,
  type PromptLanguage,
} from './lib/prompt-template'

/**
 * The「可以做视频」page (Video First Wave P2, PRD D6): a student copies one
 * block of text and pastes it into their AI tool, which configures itself and
 * produces a first clip. The student's only actions here are pick-a-model
 * (optional) and copy.
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
  const [modelId, setModelId] = useState(DEFAULT_VIDEO_MODEL.id)
  const [copied, setCopied] = useState(false)

  // Only enabled keys are offered: a disabled one configures the project just
  // as happily and then answers 401 from inside the tool, with nothing here to
  // explain it. The list arrives newest-first.
  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const res = await getApiKeys({ p: 1, size: 100 })
        if (cancelled) return
        setKeys(
          (res.data?.items ?? []).filter(
            (k) => k.status === API_KEY_STATUS.ENABLED
          )
        )
      } catch {
        if (!cancelled) setKeys([])
      } finally {
        if (!cancelled) setKeysLoaded(true)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const model =
    VIDEO_MODELS.find((m) => m.id === modelId) ?? DEFAULT_VIDEO_MODEL

  // A key qualifies only if the gateway would let it call the model currently
  // selected (AC-G): a chat-limited Simple key otherwise binds fine here and
  // fails with a 403 buried inside the agent's flow on the first generation.
  const capableKeys = useMemo(
    () => keys.filter((k) => keyPermitsModel(k, model.id)),
    [keys, model.id]
  )

  // The binding is derived, not reconciled in an effect: `selectedKeyId`
  // records the user's pick, and the effective key falls back to the newest
  // capable one whenever that pick is absent or invalidated by a model
  // switch. Switching back restores the user's original choice.
  const apiKey = useMemo(
    () =>
      capableKeys.find((k) => k.id === selectedKeyId) ?? capableKeys[0] ?? null,
    [capableKeys, selectedKeyId]
  )

  // All enabled keys stay visible — hiding the unusable ones would read as
  // "my key is gone". They are disabled with the reason instead; `label` is
  // what the closed trigger renders, and only capable keys can be selected.
  const keyOptions = useMemo(
    () =>
      keys.map((key) => {
        const capable = keyPermitsModel(key, model.id)
        return {
          value: String(key.id),
          name: key.name,
          capable,
          label: capable
            ? key.name
            : `${key.name} ${t("(can't run video models)")}`,
        }
      }),
    [keys, model.id, t]
  )

  // Mint the one-time token once the key is known (same lifecycle as the
  // one-click block: issue on load, let stale ones expire server-side).
  // The tools list only matters to someone who *runs* the redeemed script;
  // the prompt has the agent read two variables out of its text instead.
  useEffect(() => {
    if (!apiKey) {
      setScriptUrl('')
      return
    }
    let cancelled = false
    setIssuing(true)
    void (async () => {
      try {
        const res = await issueConnectToken(apiKey.id, ['claude-code'])
        if (cancelled) return
        if (!res.success || !res.data) {
          setScriptUrl('')
          return
        }
        const base = res.data.base_url.replace(/\/+$/, '')
        setScriptUrl(`${base}${res.data.script_path}`)
      } catch {
        if (!cancelled) setScriptUrl('')
      } finally {
        if (!cancelled) setIssuing(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apiKey])

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
    try {
      await navigator.clipboard.writeText(prompt)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Make videos')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t(
          'Copy one block of text, paste it into your AI tool — it sets itself up and makes your first clip. After that, just say "生成视频".'
        )}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='space-y-6'>
          {/* Step 1 — model menu. H3 and Seedance side by side, each with its
              price and traits (AC-A: 并列为可选模型，标注各自单价与特点). */}
          <section>
            <h3 className='text-sm font-semibold'>
              {t('1. Pick a model — or keep the default')}
            </h3>
            <div className='mt-3 grid gap-3 sm:grid-cols-3'>
              {VIDEO_MODELS.map((m) => {
                const selected = m.id === model.id
                return (
                  <button
                    key={m.id}
                    type='button'
                    onClick={() => setModelId(m.id)}
                    aria-pressed={selected}
                    className={`bg-card rounded-[7px] border p-3 text-left transition-colors ${
                      selected
                        ? 'border-accent ring-ring/15 ring-[3px]'
                        : 'border-border hover:border-[rgba(28,28,28,0.18)]'
                    }`}
                  >
                    <span className='block text-sm font-semibold'>
                      {m.name}
                    </span>
                    <span className='text-muted-foreground mt-1 block text-xs'>
                      {t(m.traits)}
                    </span>
                    <span className='mt-2 block text-xs tabular-nums'>
                      {t(m.price)}
                    </span>
                  </button>
                )
              })}
            </div>
            {/* AC: the page states the approximate cost per generation. */}
            <p className='text-muted-foreground mt-2 text-xs'>
              {t(
                'Each generation is charged from your balance at the price shown on the card.'
              )}
            </p>
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
                {keys.length > 0
                  ? t(
                      'None of your keys can run video models — create one with the "Video generation" purpose first:'
                    )
                  : t(
                      'You need a key first — one click on the keys page:'
                    )}{' '}
                <Link
                  to='/keys'
                  className='text-foreground underline underline-offset-2'
                >
                  {t('API Keys')}
                </Link>
              </p>
            ) : (
              <>
                {/* Which key the copied text configures. With one key there is
                    nothing to decide, so the name is simply stated; with
                    several, leaving the pick implicit means the project's .env
                    ends up holding whichever key happened to sort first and the
                    page never said which. */}
                {apiKey &&
                  (keys.length > 1 ? (
                    <div className='mt-3'>
                      <label
                        htmlFor='video-key'
                        className='text-xs font-medium'
                      >
                        {t('Key to set up')}
                      </label>
                      {/* Not a native <select>: its popup is drawn by the OS
                          and ignores the app's theme. */}
                      <Select
                        items={keyOptions}
                        value={String(apiKey?.id ?? '')}
                        onValueChange={(v) =>
                          v !== null && setSelectedKeyId(Number(v))
                        }
                      >
                        <SelectTrigger
                          id='video-key'
                          className='mt-1.5 w-full text-xs sm:max-w-sm'
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {keyOptions.map((option) => (
                              <SelectItem
                                key={option.value}
                                value={option.value}
                                disabled={!option.capable}
                              >
                                <span className='truncate'>{option.name}</span>
                                {!option.capable && (
                                  <span className='text-muted-foreground ml-1.5'>
                                    {t("(can't run video models)")}
                                  </span>
                                )}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </div>
                  ) : (
                    <p className='text-muted-foreground mt-3 text-xs'>
                      {t('Key to set up')}:{' '}
                      <span className='text-foreground font-semibold'>
                        {apiKey.name}
                      </span>
                    </p>
                  ))}

                {/* No generic "limited key" warning here anymore: the picker
                    only binds keys that CAN run the selected model (a video-
                    purpose key is limited by design and perfectly fine), and
                    incapable ones are disabled with the reason in place. */}
                <div className='border-border bg-card mt-3 rounded-[7px] border'>
                  <pre className='max-h-72 overflow-auto p-4 text-xs leading-5 whitespace-pre-wrap'>
                    {prompt || (issuing ? t('Preparing…') : '')}
                  </pre>
                  <div className='border-border flex items-center justify-between gap-3 border-t px-4 py-3'>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Valid for 30 minutes after you open this page — if it expires, refresh and copy again. Your key itself is not in this text.'
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
                    'The first run makes a small test clip (cost shown in the text), so you can see it working right away.'
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
