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
import { Loader2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TitledCard } from '@/components/ui/titled-card'
import { updateUserSettings } from '../api'
import { parseUserSettings } from '../lib'
import { PERSONA_PRESETS } from '../lib/persona-presets'
import type { Persona, UserProfile, UserSettings } from '../types'

type Props = {
  profile: UserProfile | null
  onProfileUpdate: () => void
}

type BrandValue = 'claude' | 'openai' | 'gemini' | 'deepseek' | ''

const PERSONA_OPTIONS: ReadonlyArray<{ value: Persona; label: string }> = [
  { value: 'casual', label: 'Casual — chat, write, translate' },
  { value: 'dev', label: 'Developer — code & API' },
  { value: 'team', label: 'Team / Enterprise' },
]

const BRAND_OPTIONS: ReadonlyArray<{ value: BrandValue; label: string }> = [
  { value: '', label: 'No preference' },
  { value: 'claude', label: 'Claude' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'deepseek', label: 'DeepSeek' },
]

/**
 * Lets the user change persona / favourite AI provider after the
 * onboarding wizard. Hidden when no value has ever been set — keeps
 * /profile uncluttered for the early-stage account that hasn't done
 * the wizard yet (the PersonaPickerHost redirect will pull them to
 * /welcome instead).
 *
 * Per CLAUDE.md §0 Rule 1, no third-party client brand names are
 * surfaced here — the old "Preferred client" selector (Cherry Studio /
 * Chatbox / …) was removed. The `preferred_client` setting still exists
 * in the data model but is no longer user-editable from this card.
 */
export function OnboardingPreferencesCard({ profile, onProfileUpdate }: Props) {
  const { t } = useTranslation()
  const setUser = useAuthStore((s) => s.auth.setUser)
  const user = useAuthStore((s) => s.auth.user)

  const initial = useMemo<UserSettings>(
    () => parseUserSettings(profile?.setting),
    [profile?.setting]
  )

  const [persona, setPersona] = useState<Persona>(
    (initial.persona === 'casual' ||
    initial.persona === 'dev' ||
    initial.persona === 'team'
      ? initial.persona
      : 'dev') as Persona
  )
  const [brand, setBrand] = useState<BrandValue>(
    (initial.brand_preference as BrandValue) ?? ''
  )
  const [saving, setSaving] = useState<null | 'persona' | 'brand'>(null)

  // Keep local state in sync when the user record refreshes from
  // another tab / external save.
  useEffect(() => {
    if (
      initial.persona === 'casual' ||
      initial.persona === 'dev' ||
      initial.persona === 'team'
    ) {
      setPersona(initial.persona)
    }
    setBrand((initial.brand_preference as BrandValue) ?? '')
  }, [initial.persona, initial.brand_preference])

  // Only show the card after the user has gone through the wizard at
  // least once. New / OAuth-just-created users land on /welcome via the
  // unset sentinel and never see this card until they've picked.
  const hasWizardData =
    initial.persona === 'casual' ||
    initial.persona === 'dev' ||
    initial.persona === 'team'
  if (!hasWizardData) return null

  const handlePersonaChange = async (next: string | null) => {
    if (!next) return
    const nextPersona = next as Persona
    if (nextPersona === persona) return
    const previous = persona
    setPersona(nextPersona)
    setSaving('persona')
    const preset = PERSONA_PRESETS[nextPersona]
    const res = await updateUserSettings({
      persona: nextPersona,
      // Swap the sidebar to the new persona's preset — same behaviour
      // the wizard applies on Finish. Without this the casual user
      // who switches to dev still sees the casual sidebar layout.
      sidebar_modules: preset?.sidebarModules,
    })
    setSaving(null)
    if (!res.success) {
      setPersona(previous)
      toast.error(res.message || t('Could not save your selection.'))
      return
    }
    onProfileUpdate()
    // Sync authStore so persona-aware hooks (useIsCasual, sidebar
    // visibility) re-render without a page reload.
    if (user) {
      const rawSetting = user.setting
      const currentSetting =
        typeof rawSetting === 'string'
          ? parseUserSettings(rawSetting)
          : ((rawSetting as UserSettings | undefined) ?? {})
      setUser({
        ...user,
        setting: {
          ...currentSetting,
          persona: nextPersona,
        } as unknown as Record<string, unknown>,
        sidebar_modules: preset?.sidebarModules ?? user.sidebar_modules,
      })
    }
    toast.success(t('Preferences updated'))
  }

  const handleBrandChange = async (next: string | null) => {
    const nextBrand = (next ?? '') as BrandValue
    if (nextBrand === brand) return
    const previous = brand
    setBrand(nextBrand)
    setSaving('brand')
    const res = await updateUserSettings({ brand_preference: nextBrand })
    setSaving(null)
    if (!res.success) {
      setBrand(previous)
      toast.error(res.message || t('Could not save your selection.'))
      return
    }
    onProfileUpdate()
    toast.success(t('Preferences updated'))
  }

  return (
    <TitledCard
      icon={<Sparkles className='size-4' aria-hidden='true' />}
      title={t('How you use DeepRouter')}
      description={t(
        'Set during signup. Change these any time — they only affect the UI, not your billing or access.'
      )}
    >
      <div className='grid gap-4 sm:grid-cols-2'>
        <Row label={t('Persona')} saving={saving === 'persona'}>
          <Select value={persona} onValueChange={handlePersonaChange}>
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {PERSONA_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {t(opt.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Row>
        <Row
          label={t('Favourite AI provider')}
          saving={saving === 'brand'}
        >
          <Select value={brand} onValueChange={handleBrandChange}>
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {BRAND_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value || 'none'} value={opt.value}>
                    {t(opt.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Row>
      </div>
    </TitledCard>
  )
}

function Row({
  label,
  saving,
  children,
}: {
  label: string
  saving: boolean
  children: React.ReactNode
}) {
  return (
    <div className='flex flex-col gap-1.5'>
      <div className='flex items-center justify-between'>
        <label className='text-muted-foreground text-xs font-medium'>
          {label}
        </label>
        {saving && (
          <Loader2
            className='text-muted-foreground size-3 animate-spin'
            aria-hidden='true'
          />
        )}
      </div>
      {children}
    </div>
  )
}
