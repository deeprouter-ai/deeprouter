// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Check,
  ChevronDown,
  ChevronLeft,
  Copy,
  Loader2,
  RotateCw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { issueConnectToken } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import {
  videoModelsForKey,
  type PromptLanguage,
} from '@/features/video/lib/prompt-template'
import { buildPurposePrompt } from '../lib/prompt'
import { ensurePurposeKey } from '../lib/purpose-key'
import type { SimplePurpose } from '../lib/purposes'

type Phase =
  | { state: 'preparing' }
  | { state: 'ready'; scriptUrl: string; key: ApiKey }
  | { state: 'failed'; message: string }

/**
 * A purpose's pushed page: one big "Copy for my AI" button. Behind it the
 * page finds or creates this purpose's key and mints a one-time token, so the
 * copied text never carries the key itself (same machinery as the video
 * page). Naming Claude / Codex is the deliberate CLAUDE.md §0 exception the
 * video page records: "paste it into your AI" cannot be acted on otherwise.
 */
export function SimpleUsePurpose({ purpose }: { purpose: SimplePurpose }) {
  const { t, i18n } = useTranslation()
  const { status, loading: statusLoading } = useStatus()
  // Wait for the system status before creating a key: its auto-group default
  // decides which group a new key lands in, and creating early would pin it.
  const statusReady = !statusLoading || !!status
  const defaultUseAutoGroup = status?.default_use_auto_group === true
  const [phase, setPhase] = useState<Phase>({ state: 'preparing' })
  const [attempt, setAttempt] = useState(0)
  const [copied, setCopied] = useState(false)
  const [manualOpen, setManualOpen] = useState(false)
  const [pickedModel, setPickedModel] = useState<string | null>(null)

  useEffect(() => {
    if (!statusReady) return
    let cancelled = false
    void (async () => {
      try {
        const key = await ensurePurposeKey(purpose.id, defaultUseAutoGroup)
        const res = await issueConnectToken(key.id, ['claude-code'])
        if (!res.success || !res.data) {
          throw new Error(res.message || '')
        }
        const base = res.data.base_url.replace(/\/+$/, '')
        if (!cancelled) {
          setPhase({
            state: 'ready',
            scriptUrl: `${base}${res.data.script_path}`,
            key,
          })
        }
      } catch (error) {
        if (!cancelled) {
          setPhase({
            state: 'failed',
            message: error instanceof Error ? error.message : '',
          })
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [purpose.id, defaultUseAutoGroup, statusReady, attempt])

  const language: PromptLanguage = i18n.language?.startsWith('zh') ? 'zh' : 'en'
  // Video models this key can call (its grant snapshot), cheapest first.
  // The owner picks one here; the copied prompt makes it the default.
  const videoChoices = useMemo(
    () =>
      purpose.id === 'video' && phase.state === 'ready'
        ? videoModelsForKey(phase.key)
        : null,
    [purpose.id, phase]
  )
  const videoModel =
    videoChoices?.models.find((m) => m.id === pickedModel) ??
    videoChoices?.defaultModel

  const prompt = useMemo(
    () =>
      phase.state === 'ready'
        ? buildPurposePrompt({
            purpose: purpose.id,
            scriptUrl: phase.scriptUrl,
            language,
            apiKey: phase.key,
            videoModelId: videoModel?.id,
          })
        : '',
    [phase, purpose.id, language, videoModel?.id]
  )

  const handleCopy = async () => {
    if (!prompt) return
    try {
      await navigator.clipboard.writeText(prompt)
      setCopied(true)
      navigator.vibrate?.(12)
      toast.success(t('Copied — now paste it into your AI'))
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  const Icon = purpose.icon

  return (
    <div className='animate-in slide-in-from-right-8 fade-in flex min-h-dvh flex-col duration-200'>
      <header className='bg-background/95 sticky top-0 z-10 flex h-12 items-center px-2 backdrop-blur'>
        <Link
          to='/simple'
          className='text-accent dark:text-foreground inline-flex h-11 items-center gap-0.5 rounded-full pr-3 pl-1 text-[15px] font-medium active:opacity-60'
        >
          <ChevronLeft className='size-6' />
          {t('Back')}
        </Link>
        <h1 className='absolute left-1/2 -translate-x-1/2 text-[15px] font-semibold'>
          {t(purpose.title)}
        </h1>
      </header>

      <main className='flex flex-1 flex-col items-center px-6 pt-10 text-center'>
        <span className='bg-accent/10 text-accent dark:bg-muted dark:text-foreground inline-flex size-20 items-center justify-center rounded-3xl'>
          <Icon className='size-10' />
        </span>
        <h2 className='mt-5 text-2xl font-semibold'>{t(purpose.title)}</h2>
        <p className='text-muted-foreground mt-2 max-w-xs text-sm leading-relaxed'>
          {t(
            'Tap the button, then paste into Claude or Codex. Your AI sets itself up and runs a first test.'
          )}
        </p>
        {videoChoices && videoModel && (
          <div className='mt-5 w-full text-left'>
            {videoChoices.models.length > 1 ? (
              <>
                <p className='text-muted-foreground mb-2 px-1 text-xs'>
                  {t('Pick a video model — you can switch any time')}
                </p>
                <ul className='flex flex-col gap-2' role='radiogroup'>
                  {videoChoices.models.map((m) => {
                    const active = m.id === videoModel.id
                    return (
                      <li key={m.id}>
                        <button
                          type='button'
                          role='radio'
                          aria-checked={active}
                          onClick={() => setPickedModel(m.id)}
                          className={cn(
                            'bg-card flex min-h-12 w-full items-start gap-3 rounded-2xl border px-4 py-3 text-left transition-transform active:scale-[0.99]',
                            active
                              ? 'border-accent ring-accent/30 dark:border-foreground dark:ring-foreground/20 ring-2'
                              : 'border-border'
                          )}
                        >
                          <span
                            className={cn(
                              'mt-1 inline-flex size-4 shrink-0 items-center justify-center rounded-full border',
                              active
                                ? 'border-accent bg-accent dark:border-foreground dark:bg-foreground'
                                : 'border-muted-foreground/40'
                            )}
                          >
                            {active && (
                              <Check className='text-accent-foreground dark:text-background size-3' />
                            )}
                          </span>
                          <span className='min-w-0'>
                            <span className='block text-[15px] font-medium'>
                              {m.name}
                            </span>
                            <span className='text-muted-foreground block text-xs'>
                              {m.promptLine[language]}
                            </span>
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              </>
            ) : (
              <p className='bg-card border-border rounded-full border px-4 py-1.5 text-center text-sm'>
                {t('One clip: {{cost}}', {
                  cost: videoModel.promptLine[language],
                })}
              </p>
            )}
          </div>
        )}
        <p className='text-muted-foreground mt-4 max-w-xs text-xs leading-relaxed'>
          {t(
            'You need Claude Code or Codex (AI assistants that work on a computer). Chat apps like ChatGPT or Doubao on your phone cannot do this step.'
          )}
        </p>

        <div className='mt-10 w-full'>
          {phase.state === 'failed' ? (
            <div className='flex flex-col items-center gap-3'>
              <p className='text-destructive text-sm'>
                {phase.message
                  ? t('Could not get this ready: {{message}}', {
                      message: phase.message,
                    })
                  : t('Could not get this ready. Please try again.')}
              </p>
              <button
                type='button'
                onClick={() => {
                  setPhase({ state: 'preparing' })
                  setAttempt((n) => n + 1)
                }}
                className='border-border bg-card inline-flex h-11 items-center gap-2 rounded-full border px-5 text-sm font-medium active:scale-95'
              >
                <RotateCw className='size-4' />
                {t('Try again')}
              </button>
            </div>
          ) : (
            <button
              type='button'
              onClick={handleCopy}
              disabled={phase.state !== 'ready'}
              className={cn(
                'inline-flex h-14 w-full items-center justify-center gap-2 rounded-2xl text-base font-semibold transition-transform active:scale-[0.98] disabled:opacity-60',
                copied
                  ? 'bg-emerald-600 text-white'
                  : 'bg-primary text-primary-foreground'
              )}
            >
              {phase.state === 'preparing' ? (
                <>
                  <Loader2 className='size-5 animate-spin' />
                  {t('Getting ready…')}
                </>
              ) : copied ? (
                <>
                  <Check className='size-5' />
                  {t('Copied')}
                </>
              ) : (
                <>
                  <Copy className='size-5' />
                  {t('Copy for my AI')}
                </>
              )}
            </button>
          )}
          <p className='text-muted-foreground mt-3 text-xs'>
            {t('The link inside works once and expires in 30 minutes.')}
          </p>
        </div>

        <section className='bg-card border-border mt-8 w-full rounded-2xl border p-4 text-left'>
          <h3 className='text-[15px] font-semibold'>
            {t('How to use it afterwards')}
          </h3>
          <ol className='mt-3 flex flex-col gap-3 text-sm'>
            <UsageStep n={1}>
              {t(
                'On your computer, open Claude Code or Codex in a folder for this work.'
              )}
            </UsageStep>
            <UsageStep n={2}>
              {t(
                'Paste what you copied and wait — it sets itself up and makes one test.'
              )}
            </UsageStep>
            <UsageStep n={3}>
              {t('From then on, just tell it what you want, for example:')}
              <span className='bg-muted mt-1.5 block rounded-xl px-3 py-2 text-[13px]'>
                “{t(purpose.example)}”
              </span>
            </UsageStep>
            {videoChoices && videoChoices.models.length > 1 && videoModel && (
              <UsageStep n={4}>
                {t('To use a different model, name it:')}
                <span className='bg-muted mt-1.5 block rounded-xl px-3 py-2 text-[13px]'>
                  “
                  {t('Use {{model}} for this one', {
                    model: (
                      videoChoices.models.find((m) => m.id !== videoModel.id) ??
                      videoModel
                    ).name,
                  })}
                  ”
                </span>
              </UsageStep>
            )}
          </ol>
        </section>
      </main>

      <footer className='px-6 pt-6 pb-[calc(1.5rem+env(safe-area-inset-bottom))]'>
        <button
          type='button'
          onClick={() => setManualOpen((v) => !v)}
          className='text-muted-foreground mx-auto flex items-center gap-1 text-sm'
          aria-expanded={manualOpen}
        >
          {t('I want to set it up myself')}
          <ChevronDown
            className={cn(
              'size-4 transition-transform',
              manualOpen && 'rotate-180'
            )}
          />
        </button>
        {manualOpen && (
          <div className='bg-card border-border mt-3 rounded-2xl border p-4 text-left text-sm'>
            <p className='text-muted-foreground'>
              {t(
                'A step-by-step guide for every AI app, if you prefer to do it by hand.'
              )}
            </p>
            <Link
              to='/resources/$slug'
              params={{ slug: 'GUIDE' }}
              className='text-accent dark:text-foreground mt-2 inline-block font-medium underline-offset-4 hover:underline'
            >
              {t('Open the setup guide')}
            </Link>
          </div>
        )}
      </footer>
    </div>
  )
}

function UsageStep(props: { n: number; children: ReactNode }) {
  return (
    <li className='flex gap-3'>
      <span className='bg-accent/10 text-accent dark:bg-muted dark:text-foreground inline-flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
        {props.n}
      </span>
      <span className='min-w-0 flex-1 leading-relaxed'>{props.children}</span>
    </li>
  )
}
