// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useAuthStore } from '@/stores/auth-store'
import { updateUserSettings } from '../api'
import type { Persona, UserSettings } from '../types'
import { PERSONA_PRESETS } from './persona-presets'

/**
 * Save a persona (and its sidebar preset) on the server, then mirror it into
 * the local auth store so routing decisions made right after — the console
 * mode, the persona picker — see the new value without a refetch.
 *
 * Returns the server's error message on failure, `null` on success.
 */
export async function persistPersona(persona: Persona): Promise<string | null> {
  const patch: Partial<UserSettings> & { sidebar_modules?: string } = {
    persona,
  }
  const preset = PERSONA_PRESETS[persona]
  if (preset?.sidebarModules) patch.sidebar_modules = preset.sidebarModules

  const res = await updateUserSettings(patch)
  if (!res?.success) return res?.message || 'save failed'

  const { auth } = useAuthStore.getState()
  const user = auth.user
  if (user) {
    const raw = user.setting as unknown
    let current: Record<string, unknown> = {}
    if (typeof raw === 'string') {
      try {
        current = JSON.parse(raw) as Record<string, unknown>
      } catch {
        current = {}
      }
    } else if (raw && typeof raw === 'object') {
      current = raw as Record<string, unknown>
    }
    auth.setUser({
      ...user,
      setting: { ...current, ...patch },
      sidebar_modules: patch.sidebar_modules ?? user.sidebar_modules,
    })
  }
  return null
}
