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
import { useAuthStore } from '@/stores/auth-store'
import {
  LEGACY_USER_PERSONA,
  NEW_USER_PERSONA_SENTINEL,
  resolveEffectivePersona,
} from '@/features/profile/lib/persona-presets'
import type { Persona } from '@/features/profile/types'

function settingToRawString(
  setting: Record<string, unknown> | string | undefined
): string | undefined {
  if (!setting) return undefined
  if (typeof setting === 'string') return setting
  try {
    return JSON.stringify(setting)
  } catch {
    return undefined
  }
}

/**
 * The user's effective persona for UI tailoring. Returns a real persona
 * value (`casual` / `dev` / `team`) — the sentinel `unset` is collapsed
 * to LEGACY_USER_PERSONA here, because callers that want to *act* on
 * persona shouldn't have to handle the "prompt me" marker. Components
 * that need the "should I show the picker?" signal use
 * `useShouldPromptPersona` below.
 */
export function usePersona(): Persona {
  return useAuthStore((s) => {
    const raw = settingToRawString(s.auth.user?.setting)
    const resolved = resolveEffectivePersona(raw)
    return resolved === NEW_USER_PERSONA_SENTINEL ? LEGACY_USER_PERSONA : resolved
  })
}

/**
 * True when the user's setting JSON contains the explicit 'unset'
 * sentinel placed by backend Register on new accounts. Drives the one-
 * shot persona-picker modal in the authenticated layout.
 */
export function useShouldPromptPersona(): boolean {
  return useAuthStore((s) => {
    const raw = settingToRawString(s.auth.user?.setting)
    return resolveEffectivePersona(raw) === NEW_USER_PERSONA_SENTINEL
  })
}
