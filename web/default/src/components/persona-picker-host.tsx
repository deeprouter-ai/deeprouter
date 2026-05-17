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
import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useShouldPromptPersona } from '@/hooks/use-persona'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Universal onboarding-routing host. Mounted once inside
 * AuthenticatedLayout. When the authenticated user's setting JSON
 * contains the 'unset' persona sentinel (placed by backend Register
 * OR seeded for new OAuth signups), redirect them to /welcome — the
 * full-page 3-step wizard handles persona/brand/client capture.
 *
 * Why a redirect instead of a modal:
 *   - Single funnel: email/password Register, OAuth callbacks, and
 *     legacy users who haven't picked persona all land at /welcome
 *   - More space than a modal — wizard cards + welcome banner fit
 *   - Easier to test, link to, A/B
 *   - PersonaPickerDialog modal still exists (used by /welcome and
 *     /profile preset switcher) — just not as a blocking layout-level
 *     modal anymore
 *
 * Loop prevention: skip the redirect when already on /welcome.
 */
export function PersonaPickerHost() {
  const shouldPrompt = useShouldPromptPersona()
  const user = useAuthStore((s) => s.auth.user)
  const navigate = useNavigate()

  useEffect(() => {
    if (!user || !shouldPrompt) return
    const path =
      typeof window !== 'undefined' ? window.location.pathname : ''
    if (path === '/welcome') return
    navigate({ to: '/welcome', replace: true })
  }, [user, shouldPrompt, navigate])

  return null
}
