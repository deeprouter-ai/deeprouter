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
import { TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useChatPresets } from '@/features/chat/hooks/use-chat-presets'
import { chatPresetAction } from '@/features/chat/lib/chat-links'
import { getConnectTools } from '../api'
import type { ConnectTool } from '../types'
import { ApiKeysAskAiSection } from './api-keys-ask-ai-section'
import { ApiKeysOneClickSection } from './api-keys-one-click-card'
import { useApiKeys } from './api-keys-provider'
import { ApiKeysQuickAppsSection } from './api-keys-quick-apps-card'

/**
 * The one box that turns a key into a working tool.
 *
 * It holds two ways of doing the same thing — a line to paste into a terminal,
 * and a button per chat app — and one key choice above both. That choice used
 * to live inside the terminal block while the chat buttons silently picked
 * their own; @sam asked for a single control on 2026-08-28 after asking which
 * key the buttons were actually configuring and finding that the page could
 * not say.
 *
 * Terminal first, chat apps second (@sam, same day). Not the persona ordering
 * the PRD assumed — the terminal path is the finished, real-machine-verified
 * one, and it is what the people setting up today are using.
 *
 * Both sections hide themselves when they have nothing to offer, so this
 * component owns what they need to make that call: with no key there is
 * nothing to configure at all, and with neither a CLI tool nor a chat preset
 * the box would be a heading over an empty space.
 */
export function ApiKeysSetupCard() {
  const { t } = useTranslation()
  const { setupKeys, setupKeyId, setSetupKeyId, setupKey } = useApiKeys()
  const { chatPresets, serverAddress } = useChatPresets()
  const [tools, setTools] = useState<ConnectTool[]>([])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const res = await getConnectTools()
        if (cancelled || !res.success || !res.data) return
        setTools(res.data)
      } catch {
        /* leave the section hidden rather than showing a broken control */
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  // A preset whose value is neither a link nor a marker we know cannot be
  // acted on at all — a button for it produces a dead click.
  const appPresets = useMemo(
    () =>
      chatPresets.filter(
        (preset) => chatPresetAction(preset.url) !== 'unsupported'
      ),
    [chatPresets]
  )

  // `label` is what the closed trigger renders, so the restriction hint has to
  // be baked into the string there; inside the open list it is a separate muted
  // span, which reads better but cannot be reused for the trigger.
  const keyOptions = useMemo(
    () =>
      setupKeys.map((key) => ({
        value: String(key.id),
        name: key.name,
        limited: key.model_limits_enabled,
        label: key.model_limits_enabled
          ? `${key.name} ${t('(limited to some models)')}`
          : key.name,
      })),
    [setupKeys, t]
  )

  // No key yet means there is nothing to configure; the create flow above is
  // the right next step, and a disabled block here would only add noise.
  if (!setupKey) return null
  if (tools.length === 0 && appPresets.length === 0) return null

  return (
    <div className='bg-muted/30 mb-4 rounded-lg border p-4 sm:p-5'>
      <h3 className='text-sm font-semibold'>{t('One-click setup')}</h3>
      <p className='text-muted-foreground mt-1 text-xs'>
        {t(
          'Set up whatever you already use — no settings to find, nothing to type by hand.'
        )}
      </p>

      {/* Which key everything below configures. With one key there is nothing
          to decide, so the row is not shown at all; with several, leaving it
          implicit would bake whichever sorts first into the command and only
          surface as a 403 inside some tool days later. */}
      {setupKeys.length > 1 && (
        <div className='mt-3'>
          <label htmlFor='setup-key' className='text-xs font-medium'>
            {t('Key to set up')}
          </label>
          {/* Not a native <select>: its popup is drawn by the OS and ignores
              the app's theme, so it showed up as a white list on a dark page. */}
          <Select
            items={keyOptions}
            value={String(setupKeyId ?? '')}
            onValueChange={(v) => v !== null && setSetupKeyId(Number(v))}
          >
            <SelectTrigger
              id='setup-key'
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
          poor thing to configure every tool with — say so here rather than
          letting it turn into "model not allowed" inside the tool. */}
      {setupKey.model_limits_enabled && (
        <p className='text-muted-foreground mt-2 flex items-start gap-1.5 text-[11px]'>
          <TriangleAlert className='mt-px h-3 w-3 shrink-0' />
          <span>
            {t(
              'This key is limited to certain models. Tools configured with it will only work for those — pick an unrestricted key if you want everything to work.'
            )}
          </span>
        </p>
      )}

      <ApiKeysOneClickSection tools={tools} apiKey={setupKey} />
      <ApiKeysQuickAppsSection
        presets={appPresets}
        serverAddress={serverAddress}
        apiKey={setupKey}
      />
      {/* Last, because it is the fallback: neither a deep link nor the script
          covers Chatbox / Cursor / Copilot / Zed, and what those users do is
          ask whatever AI is in front of them (PRD §4.7 B2). */}
      <ApiKeysAskAiSection />
    </div>
  )
}
