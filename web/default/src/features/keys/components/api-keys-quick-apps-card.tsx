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
import { useCallback, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ExternalLink, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  chatLinkRequiresApiKey,
  chatPresetAction,
  resolveChatUrl,
  type ChatPreset,
} from '@/features/chat/lib/chat-links'
import { sendToFluent } from '@/features/chat/lib/send-to-fluent'
import { DOC_CATEGORIES } from '@/features/docs/catalog'
import type { ApiKey } from '../types'
import { useApiKeys } from './api-keys-provider'

/** Guide slug whose title matches this preset, so "nothing happened" has somewhere to go. */
function guideSlugFor(name: string): string | null {
  const norm = (value: string) => value.toLowerCase().replace(/[^a-z0-9]/g, '')
  const target = norm(name)
  for (const category of DOC_CATEGORIES) {
    for (const entry of category.entries) {
      if (norm(entry.title) === target || norm(entry.slug) === target) {
        return entry.slug
      }
    }
  }
  return null
}

/**
 * One-click setup for desktop chat apps, on the key page itself.
 *
 * The mechanism has worked in production for a while — clicking hands the app
 * its provider, base URL, key and models, and the user types nothing. What was
 * missing is that it lived only in a row's three-dot menu, and someone who just
 * paid does not go looking behind three dots (PRD §4.7 B1).
 *
 * A section of `ApiKeysSetupCard`, alongside the terminal block: same job, two
 * kinds of tool. The key comes down as a prop because one control above both
 * sections decides it — the buttons used to resolve their own, silently, which
 * is the question that started this (@sam, 2026-08-28).
 *
 * The presets come from `/api/status`, never a hardcoded array: operators add
 * and remove them in the admin console and this has to follow.
 */
export function ApiKeysQuickAppsSection({
  presets,
  serverAddress,
  apiKey,
}: {
  presets: ChatPreset[]
  serverAddress: string
  apiKey: ApiKey
}) {
  const { t } = useTranslation()
  const { setOpen, setResolvedKey, resolveRealKey } = useApiKeys()
  const [loadingId, setLoadingId] = useState<string | null>(null)
  const [launched, setLaunched] = useState<ChatPreset | null>(null)

  const handleOpen = useCallback(
    async (preset: ChatPreset) => {
      if (loadingId) return
      setLoadingId(preset.id)
      try {
        // Every path here needs the plaintext key, CC Switch included: its
        // dialog reads `resolvedKey` off the context and builds the import URL
        // from it, so opening the dialog without filling that in produced an
        // `apiKey=sk-` import that failed inside the app (2026-08-28).
        const realKey = await resolveRealKey(apiKey.id)
        if (!realKey) return

        if (chatPresetAction(preset.url) === 'cc-switch-dialog') {
          setResolvedKey(realKey)
          setOpen('cc-switch')
          return
        }

        if (preset.type === 'fluent') {
          if (sendToFluent(realKey, serverAddress)) {
            toast.success(t('Sent the API key to FluentRead.'))
          } else {
            toast.info(
              t('If nothing opens, this app is probably not installed.')
            )
          }
          return
        }

        const url = resolveChatUrl({
          template: preset.url,
          apiKey: chatLinkRequiresApiKey(preset.url) ? realKey : undefined,
          serverAddress,
        })
        if (!url) {
          toast.error(
            t('Invalid chat link. Please contact your administrator.')
          )
          return
        }

        // A custom-protocol link that finds no app registered fails silently in
        // every browser — no error, no event to listen for. So the notice is
        // shown unconditionally after launching, rather than on a detection we
        // cannot actually perform, and it is worded as a condition ("if
        // nothing opens") instead of a claim.
        setLaunched(preset)
        if (preset.type === 'web') {
          window.open(url, '_blank', 'noopener')
          return
        }
        // 🔴 Not `window.open` for a custom protocol: with no app registered
        // for the scheme the browser still opens the tab and leaves it blank
        // and untitled — and that empty tab takes the focus, so the one place
        // explaining what went wrong is now behind it (@sam saw exactly this,
        // 2026-08-28). Assigning to `location` hands the URL to the OS with no
        // tab involved: the app comes up if it is installed, and nothing moves
        // if it is not.
        try {
          window.location.href = url
        } catch {
          window.open(url, '_blank', 'noopener')
        }
        toast.info(t('If nothing opens, this app is probably not installed.'))
      } finally {
        setLoadingId(null)
      }
    },
    [
      loadingId,
      apiKey.id,
      resolveRealKey,
      serverAddress,
      setOpen,
      setResolvedKey,
      t,
    ]
  )

  if (presets.length === 0) return null

  const launchedSlug = launched ? guideSlugFor(launched.name) : null

  return (
    <section className='border-border mt-4 border-t pt-4'>
      {/* Not "chat apps": the operator's preset list also carries a browser
          translation extension and a couple of workspace tools, so a reader
          scanning for the button they recognise was being told the wrong
          category (@sam, 2026-08-28). */}
      <h4 className='text-xs font-semibold'>{t('AI apps and extensions')}</h4>
      <p className='text-muted-foreground mt-1 text-xs'>
        {t(
          'Already have one of these installed? Click it and the app opens with everything filled in — address, key and models. You type nothing.'
        )}
      </p>

      <div className='mt-3 flex flex-wrap gap-2'>
        {presets.map((preset) => (
          <Button
            key={preset.id}
            type='button'
            size='sm'
            variant='outline'
            className='h-8 text-xs'
            disabled={loadingId !== null}
            onClick={() => void handleOpen(preset)}
          >
            {loadingId === preset.id ? (
              <Loader2 className='mr-1.5 h-3 w-3 animate-spin' />
            ) : (
              <ExternalLink className='mr-1.5 h-3 w-3' />
            )}
            {preset.name}
          </Button>
        ))}
      </div>

      {/* Measured on Cherry Studio 2026-08-26: the app shows the provider as
          "New API", because the deep link matches its built-in preset whose id
          is `new-api`. That is upstream naming we cannot change, so say it
          before the click — otherwise it reads as "I clicked the wrong thing"
          (PRD §4.7 B1). */}
      <p className='text-muted-foreground mt-2 text-[11px]'>
        {t(
          'Your browser will ask for permission to open the app. Inside it the provider is called "New API" — that is the right one.'
        )}
      </p>

      {launched && launched.type !== 'web' && (
        <p className='text-muted-foreground mt-2 text-[11px]'>
          {t('Nothing happened? {{name}} may not be installed.', {
            name: launched.name,
          })}{' '}
          {launchedSlug ? (
            <Link
              to='/resources/$slug'
              params={{ slug: launchedSlug }}
              className='underline underline-offset-2'
            >
              {t('Set it up by hand instead')}
            </Link>
          ) : (
            <Link to='/resources' className='underline underline-offset-2'>
              {t('Set it up by hand instead')}
            </Link>
          )}
        </p>
      )}
    </section>
  )
}
