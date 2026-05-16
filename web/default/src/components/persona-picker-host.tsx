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
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useShouldPromptPersona } from '@/hooks/use-persona'
import { useAuthStore } from '@/stores/auth-store'
import { updateUserSettings } from '@/features/profile/api'
import { PERSONA_PRESETS } from '@/features/profile/lib/persona-presets'
import type { Persona, UserSettings } from '@/features/profile/types'
import { PersonaPickerDialog } from './persona-picker-dialog'

function parseSetting(
  setting: Record<string, unknown> | string | undefined
): UserSettings {
  if (!setting) return {}
  if (typeof setting === 'string') {
    try {
      return JSON.parse(setting) as UserSettings
    } catch {
      return {}
    }
  }
  return setting as UserSettings
}

/**
 * Renders the one-shot persona picker when the authenticated user's
 * setting JSON contains the 'unset' sentinel placed by backend Register.
 * Mounted once inside AuthenticatedLayout — feature components don't need
 * to know about it.
 *
 * On pick:
 *   1. PUT /api/user/setting with persona + matching sidebar_modules preset
 *   2. Update authStore so all consumers see the new persona immediately
 *   3. Navigate to the persona's default route (only if currently on /dashboard*)
 */
export function PersonaPickerHost() {
  const { t } = useTranslation()
  const shouldPrompt = useShouldPromptPersona()
  const user = useAuthStore((s) => s.auth.user)
  const setUser = useAuthStore((s) => s.auth.setUser)
  const navigate = useNavigate()
  const [submitting, setSubmitting] = useState(false)

  if (!user || !shouldPrompt) return null

  const handlePick = async (persona: Persona) => {
    if (submitting) return
    setSubmitting(true)
    try {
      const preset = PERSONA_PRESETS[persona]
      const currentSetting = parseSetting(user.setting)
      const nextSetting: UserSettings = {
        ...currentSetting,
        persona,
      }

      const res = await updateUserSettings({
        ...nextSetting,
        persona,
        sidebar_modules: preset.sidebarModules,
      })

      if (!res.success) {
        toast.error(res.message || t('Could not save your selection.'))
        return
      }

      // Reflect new persona + sidebar in auth store so the rest of the
      // app re-renders immediately (sidebar config hook depends on these).
      setUser({
        ...user,
        setting: nextSetting as unknown as Record<string, unknown>,
        sidebar_modules: preset.sidebarModules,
      })

      // Navigate to the persona's home only if we're on a generic landing
      // (the dashboard/index/keys empty state). Avoid hijacking deep links.
      const path =
        typeof window !== 'undefined' ? window.location.pathname : ''
      const landingPaths = ['/', '/dashboard', '/dashboard/overview', '/keys']
      if (landingPaths.some((p) => path === p || path.startsWith(p + '/'))) {
        // TanStack Router's typed routes don't always include
        // /dashboard/overview; persona presets store a path string so we
        // bypass the type-narrowed `to` here.
        navigate({ to: preset.defaultRoute as never, replace: true })
      }

      toast.success(t('Welcome aboard!'))
    } catch (_e) {
      toast.error(t('Could not save your selection.'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <PersonaPickerDialog
      open={shouldPrompt}
      onOpenChange={() => {
        /* dismissible={false}; ignore close attempts so it's a forced step */
      }}
      onPick={handlePick}
      dismissible={false}
    />
  )
}
