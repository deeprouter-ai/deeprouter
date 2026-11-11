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
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { defaultBaseUrl, modelNameForPurpose } from '../lib/integration'
import { isMediaPurpose } from '../lib/media-integration'
import type { SimplePurposeId } from '../types'
import { ApiKeyIntegrationDialog } from './api-key-integration-dialog'
import { KeyModelDiscovery } from './key-model-discovery'
import { MediaKeySetup } from './media-key-setup'

type ApiKeySuccessDialogProps = {
  open: boolean
  onClose: () => void
  apiKey: string | null
  purpose?: SimplePurposeId
}

/**
 * Shown immediately after a Simple-mode API key is created. Closes the create
 * drawer's noisy toast path: the key is only revealed once, here, with one-tap
 * copy and a row of client-tutorial entry points. PRD docs/tasks/
 * api-key-simple-advanced-prd.md §4.2.
 */
export function ApiKeySuccessDialog({
  open,
  onClose,
  apiKey,
  purpose,
}: ApiKeySuccessDialogProps) {
  const { t } = useTranslation()
  const baseUrl = defaultBaseUrl()
  const modelName = modelNameForPurpose(purpose)
  const [showGuide, setShowGuide] = useState(false)
  const media = isMediaPurpose(purpose)
  return (
    <AlertDialog open={open} onOpenChange={(o) => !o && onClose()}>
      <AlertDialogContent className='max-h-[calc(100dvh-2rem)] !max-w-md overflow-y-auto sm:!max-w-2xl'>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Your new API key is ready')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Copy these values into your AI client now — the full key is only shown once.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {/* min-w-0: a grid item never shrinks below its content's min width,
         * so a long one-line value would widen the whole dialog instead of
         * truncating — cutting off the copy buttons behind a sideways scroll. */}
        <div className='min-w-0 space-y-3'>
          <CopyField
            label={t('API key')}
            value={apiKey ?? ''}
            secret
            warning={t('Only shown once. Copy and store it securely.')}
          />
          {media ? (
            <>
              {purpose === 'video' && (
                <Button
                  size='sm'
                  variant='outline'
                  render={<a href='/video'>{t('Go to Make videos →')}</a>}
                />
              )}
              <MediaKeySetup apiKey={apiKey} purpose={purpose!} />
            </>
          ) : (
            <>
              {/* Side by side on wider screens so the dialog fits without
               * scrolling; they are filled in together anyway. */}
              <div className='grid gap-3 sm:grid-cols-2'>
                <CopyField label={t('Base URL')} value={baseUrl} />
                <CopyField
                  label={t('Model name')}
                  value={modelName}
                  hint={t(
                    'Use this in your client. We route it to the right model based on this key.'
                  )}
                />
              </div>
              <KeyModelDiscovery apiKey={apiKey} />
              <div className='border-t pt-3'>
                <p className='text-foreground text-xs font-medium'>
                  {t('How to use this key')}
                </p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  {t(
                    'Paste it into the AI tool you already use — find the "API Key" field in its settings.'
                  )}
                </p>
                {/* Primary action is the self-check (onboarding-v2 §7.6) — it
                 * proves "my money turns into AI replies", which is the decisive
                 * casual step. Code examples are a developer extra, demoted to a
                 * quiet secondary link so non-coders aren't pushed toward code. */}
                <div className='mt-2 flex flex-wrap gap-2'>
                  <Button
                    size='sm'
                    className='rounded-full text-xs'
                    render={<a href='/keys/test'>{t('Test this key →')}</a>}
                  />
                  <Button
                    size='sm'
                    variant='ghost'
                    className='rounded-full text-xs'
                    onClick={() => setShowGuide(true)}
                  >
                    {t('Setup guide')}
                  </Button>
                </div>
              </div>
            </>
          )}
        </div>
        <AlertDialogFooter>
          <AlertDialogAction onClick={onClose}>{t('Done')}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>

      <ApiKeyIntegrationDialog
        open={showGuide}
        onClose={() => setShowGuide(false)}
        apiKey={apiKey}
        purpose={purpose}
      />
    </AlertDialog>
  )
}

function CopyField({
  label,
  value,
  hint,
  secret,
  warning,
}: {
  label: string
  value: string
  hint?: string
  secret?: boolean
  warning?: string
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
    <div className='min-w-0 space-y-1'>
      <div className='flex items-baseline justify-between'>
        <span className='text-foreground text-xs font-medium'>{label}</span>
        {warning && (
          <span className='text-[11px] text-amber-600 dark:text-amber-400'>
            {warning}
          </span>
        )}
      </div>
      <div className='border-border bg-muted/30 flex items-center gap-2 rounded-md border px-3 py-1.5'>
        <code
          className={cn(
            'flex-1 truncate font-mono text-xs',
            secret && 'tracking-wide'
          )}
          title={value}
        >
          {value || '—'}
        </code>
        <Button
          type='button'
          size='sm'
          variant='ghost'
          className='h-6 px-2'
          onClick={handleCopy}
          disabled={!value}
        >
          {copied ? (
            <Check className='h-3.5 w-3.5' />
          ) : (
            <Copy className='h-3.5 w-3.5' />
          )}
        </Button>
      </div>
      {hint && <p className='text-muted-foreground text-[11px]'>{hint}</p>}
    </div>
  )
}
