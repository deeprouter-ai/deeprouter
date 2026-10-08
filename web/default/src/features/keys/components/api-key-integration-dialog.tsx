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
import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowRight, Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useOrgMembership } from '@/features/org/hooks/use-org-membership'
import {
  API_KEY_PLACEHOLDER,
  buildIntegrationSnippets,
  defaultBaseUrl,
  modelNameForPurpose,
  type IntegrationLanguage,
} from '../lib/integration'
import { isMediaPurpose } from '../lib/media-integration'
import type { SimplePurposeId } from '../types'
import { KeyModelDiscovery } from './key-model-discovery'
import { MediaKeySetup } from './media-key-setup'

type ApiKeyIntegrationDialogProps = {
  open: boolean
  onClose: () => void
  /** Real key when launched from a specific row / success dialog. */
  apiKey?: string | null
  purpose?: SimplePurposeId | string
}

const LANGUAGES: Array<{ id: IntegrationLanguage; label: string }> = [
  { id: 'claude-code', label: 'Claude Code' },
  { id: 'opencode', label: 'opencode' },
  { id: 'curl', label: 'cURL' },
  { id: 'python', label: 'Python' },
  { id: 'node', label: 'Node' },
]

const TOTAL_STEPS = 3

/** Masks a key like the keys table does: first 4 + ********** + last 4. */
function maskKey(key: string): string {
  const prefix = key.startsWith('sk-') ? 'sk-' : ''
  const bare = key.slice(prefix.length)
  if (bare.length <= 8) return prefix + '*'.repeat(bare.length)
  return `${prefix}${bare.slice(0, 4)}**********${bare.slice(-4)}`
}

/**
 * Step-by-step "how do I actually use my key" wizard. Closes the gap where a
 * customer creates a key and has no idea what code to write — the success
 * dialog only surfaced Base URL + model name and only at create time. This
 * is reachable any time from the /keys header and per-row menu, and reused
 * from the success dialog. Three steps:
 *   1. Copy credentials (key / Base URL / model)
 *   2. Pick cURL / Python / Node.js and copy a runnable snippet (or use a GUI client)
 *   3. Run & verify (link to the one-shot self-check)
 */
export function ApiKeyIntegrationDialog({
  open,
  onClose,
  apiKey,
  purpose,
}: ApiKeyIntegrationDialogProps) {
  const { t } = useTranslation()
  const [step, setStep] = useState(1)
  const [lang, setLang] = useState<IntegrationLanguage>('claude-code')
  // The one-click test runs on the playground endpoint, which is closed to
  // organization accounts (Enterprise Org PRD §4): they are not sent to it.
  const inOrg = Boolean(useOrgMembership().data)

  const baseUrl = defaultBaseUrl()
  const model = modelNameForPurpose(purpose)
  const hasRealKey = Boolean(apiKey)
  const snippets = buildIntegrationSnippets({ baseUrl, model, apiKey })
  // The guide shows the key masked everywhere (screen sharing, shoulder
  // surfing); every copy button still copies the real value.
  const shownKey = apiKey ? maskKey(apiKey) : undefined
  const shownSnippets = apiKey
    ? buildIntegrationSnippets({ baseUrl, model, apiKey: shownKey })
    : snippets

  // Reset the wizard on close so the next open starts at step 1 — avoids a
  // setState-in-effect on `open`.
  const handleClose = () => {
    onClose()
    setStep(1)
    setLang('claude-code')
  }

  if (isMediaPurpose(purpose)) {
    return (
      <Dialog open={open} onOpenChange={(value) => !value && handleClose()}>
        <DialogContent className='max-h-[calc(100dvh-2rem)] !max-w-lg overflow-y-auto sm:!max-w-xl'>
          <DialogHeader>
            <DialogTitle>{t('Setup guide')}</DialogTitle>
            <DialogDescription>
              {t(
                'Your key includes the models for the purpose you selected. Choose one below to use it.'
              )}
            </DialogDescription>
          </DialogHeader>
          <CopyField
            label={t('API key')}
            value={apiKey ?? API_KEY_PLACEHOLDER}
            display={shownKey}
            copyable={hasRealKey}
            secret
          />
          {/* Video keys skip this: their path is the video page's one-time
           * paste-prompt. Image and voice keys have no such prompt. */}
          {apiKey && purpose !== 'video' && (
            <AiHandoffField apiKey={apiKey} baseUrl={baseUrl} />
          )}
          <MediaKeySetup apiKey={apiKey} purpose={purpose!} />
          <DialogFooter>
            <Button onClick={handleClose}>{t('Done')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className='max-h-[calc(100dvh-2rem)] !max-w-lg overflow-y-auto sm:!max-w-xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            {t('Setup guide')}
            <span className='text-muted-foreground text-xs font-normal'>
              {step} / {TOTAL_STEPS}
            </span>
          </DialogTitle>
          <DialogDescription>
            {step === 1 &&
              t(
                'Copy your credentials. You will paste these into your code or AI client.'
              )}
            {step === 2 &&
              t('Pick how you want to connect, then copy the snippet.')}
            {step === 3 && t('Run it and confirm you get a reply back.')}
          </DialogDescription>
        </DialogHeader>

        <StepDots step={step} />

        {/* min-w-0: a grid item never shrinks below its content's min width,
         * so the long one-line handoff would widen the whole dialog. */}
        <div className='min-h-[18rem] min-w-0'>
          {step === 1 && (
            <div className='space-y-3'>
              <CopyField
                label={t('API key')}
                value={apiKey ?? API_KEY_PLACEHOLDER}
                display={shownKey}
                copyable={hasRealKey}
                secret
                hint={
                  hasRealKey
                    ? t('Only shown once at creation. Keep it secret.')
                    : t(
                        'Open this guide from a specific key (its menu → Setup guide) to fill the key automatically, or copy it from the table.'
                      )
                }
              />
              {apiKey && <AiHandoffField apiKey={apiKey} baseUrl={baseUrl} />}
              <KeyModelDiscovery apiKey={apiKey} />
              {/* Side by side on wider screens so step 1 fits without
               * scrolling; they are filled in together anyway. */}
              <div className='grid gap-3 sm:grid-cols-2'>
                <CopyField label={t('Base URL')} value={baseUrl} copyable />
                <CopyField
                  label={t('Model name')}
                  value={model}
                  copyable
                  hint={t(
                    'Use this model name. We route it to the best underlying model for this key.'
                  )}
                />
              </div>
            </div>
          )}

          {step === 2 && (
            <Tabs
              value={lang}
              onValueChange={(v) => setLang(v as IntegrationLanguage)}
            >
              <TabsList className='w-full'>
                {LANGUAGES.map((l) => (
                  <TabsTrigger key={l.id} value={l.id} className='flex-1'>
                    {l.label}
                  </TabsTrigger>
                ))}
                <TabsTrigger value='gui' className='flex-1'>
                  {t('AI client')}
                </TabsTrigger>
              </TabsList>

              {LANGUAGES.map((l) => (
                <TabsContent key={l.id} value={l.id} className='mt-3'>
                  <CodeBlock
                    code={snippets[l.id]}
                    display={shownSnippets[l.id]}
                  />
                  {!hasRealKey && (
                    <p className='text-muted-foreground mt-2 text-[11px]'>
                      {t('Replace')}{' '}
                      <code className='bg-muted rounded px-1 py-0.5 font-mono'>
                        {API_KEY_PLACEHOLDER}
                      </code>{' '}
                      {t('with your real key.')}
                    </p>
                  )}
                </TabsContent>
              ))}

              <TabsContent value='gui' className='mt-3'>
                <p className='text-muted-foreground text-xs leading-relaxed'>
                  {t(
                    'Already use an AI app? Open its settings, find the "API key" and "Base URL" (sometimes "Endpoint") fields, paste both from step 1, and save.'
                  )}
                </p>
              </TabsContent>
            </Tabs>
          )}

          {step === 3 && (
            <div className='space-y-4 text-sm'>
              <div className='bg-muted/30 rounded-md border p-3'>
                <p className='font-medium'>{t('What you should see')}</p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  {t(
                    "A successful call returns a JSON reply with the model's answer (e.g. choices[0].message.content). If you get a 401, the key is wrong; a 402/insufficient-balance means you need to top up."
                  )}
                </p>
              </div>
              {!inOrg && (
                <div className='space-y-2'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Not a developer? Run a one-click test instead:')}
                  </p>
                  <Button
                    size='sm'
                    variant='outline'
                    render={
                      <Link to='/keys/test' onClick={handleClose}>
                        {t('Test this key →')}
                      </Link>
                    }
                  />
                </div>
              )}
              <p className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'Lost the key value? Regenerate it from the table — your balance stays safe.'
                )}
              </p>
            </div>
          )}
        </div>

        <DialogFooter className='flex-row justify-between sm:justify-between'>
          <Button
            variant='ghost'
            size='sm'
            disabled={step === 1}
            onClick={() => setStep((s) => Math.max(1, s - 1))}
          >
            <ArrowLeft className='mr-1 h-4 w-4' />
            {t('Back')}
          </Button>
          {step < TOTAL_STEPS ? (
            <Button size='sm' onClick={() => setStep((s) => s + 1)}>
              {t('Next')}
              <ArrowRight className='ml-1 h-4 w-4' />
            </Button>
          ) : (
            <Button size='sm' onClick={handleClose}>
              {t('Done')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/**
 * One sentence to paste into an AI assistant. A bare key tells an AI nothing
 * (it looks like any sk- key), so the docs and base URL travel with it.
 */
function AiHandoffField({
  apiKey,
  baseUrl,
}: {
  apiKey: string
  baseUrl: string
}) {
  const { t } = useTranslation()
  // Agent-readable docs index, served on the same host as the gateway.
  const docsUrl = baseUrl.replace(/\/v1$/, '/llms.txt')
  const sentence = (key: string) =>
    t(
      'I use DeepRouter, an AI API gateway. Read {{docsUrl}} before calling it: it explains chat, video, image and speech requests. Base URL: {{baseUrl}}. API key: {{apiKey}}. List the models this key can call with GET {{baseUrl}}/models. Keep the key in an environment variable or .env and never print it.',
      { docsUrl, baseUrl, apiKey: key }
    )
  return (
    <CopyField
      label={t('For your AI assistant')}
      value={sentence(apiKey)}
      display={sentence(maskKey(apiKey))}
      hint={t('Contains the key — only paste it into tools you trust.')}
    />
  )
}

function StepDots({ step }: { step: number }) {
  return (
    <div className='flex items-center gap-1.5'>
      {Array.from({ length: TOTAL_STEPS }, (_, i) => i + 1).map((n) => (
        <span
          key={n}
          className={cn(
            'h-1.5 flex-1 rounded-full transition-colors',
            n <= step ? 'bg-foreground' : 'bg-muted'
          )}
        />
      ))}
    </div>
  )
}

function CodeBlock({
  code,
  display,
}: {
  code: string
  /** Shown instead of code (e.g. with a masked key); copying takes code. */
  display?: string
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }
  return (
    <div className='border-border bg-muted/30 relative rounded-md border'>
      <Button
        type='button'
        size='sm'
        variant='ghost'
        className='absolute top-2 right-2 h-7 px-2'
        onClick={handleCopy}
      >
        {copied ? (
          <Check className='h-3.5 w-3.5' />
        ) : (
          <Copy className='h-3.5 w-3.5' />
        )}
      </Button>
      <pre className='overflow-x-auto p-3 pr-12 text-[11px] leading-relaxed'>
        <code className='font-mono'>{display ?? code}</code>
      </pre>
    </div>
  )
}

function CopyField({
  label,
  value,
  display,
  hint,
  secret,
  copyable = true,
}: {
  label: string
  value: string
  /** Shown (and in the tooltip) instead of value, e.g. a masked key. */
  display?: string
  hint?: string
  secret?: boolean
  copyable?: boolean
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }
  return (
    // min-w-0: in the media branch this field is itself a grid item.
    <div className='min-w-0 space-y-1'>
      <span className='text-foreground text-xs font-medium'>{label}</span>
      <div className='border-border bg-muted/30 flex items-center gap-2 rounded-md border px-3 py-2'>
        <code
          className={cn(
            'flex-1 truncate font-mono text-xs',
            secret && 'tracking-wide'
          )}
          title={display ?? value}
        >
          {(display ?? value) || '—'}
        </code>
        {copyable && (
          <Button
            type='button'
            size='sm'
            variant='ghost'
            className='h-7 px-2'
            onClick={handleCopy}
            disabled={!value}
          >
            {copied ? (
              <Check className='h-3.5 w-3.5' />
            ) : (
              <Copy className='h-3.5 w-3.5' />
            )}
          </Button>
        )}
      </div>
      {hint && <p className='text-muted-foreground text-[11px]'>{hint}</p>}
    </div>
  )
}
