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
import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { TitledCard } from '@/components/ui/titled-card'
import { updateTelemetryConsent } from '../api'
import type { UserProfile } from '../types'

type TelemetryConsentCardProps = {
  profile: UserProfile | null
  loading: boolean
  onProfileUpdate: () => void
}

function formatConsentTime(value?: string) {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function TelemetryConsentCard(props: TelemetryConsentCardProps) {
  const { t } = useTranslation()
  const [optimisticEnabled, setOptimisticEnabled] = useState<boolean | null>(
    null
  )
  const [saving, setSaving] = useState(false)
  const enabled =
    optimisticEnabled ?? Boolean(props.profile?.tier2_telemetry_consent)

  const consentedAt = formatConsentTime(
    props.profile?.tier2_telemetry_consented_at
  )

  const handleConsentChange = async (nextEnabled: boolean) => {
    const previous = enabled
    setOptimisticEnabled(nextEnabled)
    setSaving(true)
    try {
      const response = await updateTelemetryConsent({
        tier2_telemetry_consent: nextEnabled,
      })
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to update settings'))
      }
      setOptimisticEnabled(response.data.tier2_telemetry_consent)
      props.onProfileUpdate()
      toast.success(
        nextEnabled
          ? t('Tier 2 telemetry consent enabled')
          : t('Tier 2 telemetry consent disabled')
      )
    } catch (_error) {
      setOptimisticEnabled(previous)
      toast.error(t('Failed to update telemetry consent'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <TitledCard
      title={t('Privacy')}
      description={t('Control optional Skill usage telemetry')}
      icon={<ShieldCheck className='h-4 w-4' />}
    >
      {props.loading ? (
        <div className='space-y-3'>
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-20 w-full' />
        </div>
      ) : (
        <div className='space-y-4'>
          <div className='border-border bg-card flex flex-col gap-4 rounded-lg border p-4 sm:flex-row sm:items-start sm:justify-between'>
            <div className='min-w-0 space-y-2'>
              <div className='flex flex-wrap items-center gap-2'>
                <h3 className='text-sm font-semibold'>
                  {t('Tier 2 telemetry consent')}
                </h3>
                <Badge variant='outline' className='rounded-full'>
                  {enabled ? t('Enabled') : t('Disabled')}
                </Badge>
              </div>
              <p className='text-muted-foreground text-sm leading-6'>
                {t(
                  'Allow DeepRouter to store Skill usage metadata such as downloaded Skills, token counts, cost, model tier, event time, and success status for support and analytics.'
                )}
              </p>
              <p className='text-muted-foreground text-sm leading-6'>
                {t(
                  'Raw prompts, raw input or output, provider payloads, package contents, and instruction templates are not stored or shown in this view.'
                )}
              </p>
              {consentedAt && (
                <p className='text-muted-foreground text-xs'>
                  {t('Last enabled at {{time}}', { time: consentedAt })}
                </p>
              )}
            </div>
            <Switch
              aria-label={t('Tier 2 telemetry consent')}
              checked={enabled}
              disabled={saving}
              onCheckedChange={handleConsentChange}
            />
          </div>
        </div>
      )}
    </TitledCard>
  )
}
