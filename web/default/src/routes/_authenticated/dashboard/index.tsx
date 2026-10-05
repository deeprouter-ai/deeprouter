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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { DASHBOARD_DEFAULT_SECTION } from '@/features/dashboard/section-registry'
import { consoleModeFor, SIMPLE_HOME } from '@/features/simple/lib/mode'

export const Route = createFileRoute('/_authenticated/dashboard/')({
  beforeLoad: () => {
    // Every "go to the console" entry (sign-in, OAuth, the home page button)
    // lands here, so this is the one place that routes Simple users home.
    if (consoleModeFor(useAuthStore.getState().auth.user) === 'simple') {
      throw redirect({ to: SIMPLE_HOME })
    }
    throw redirect({
      to: '/dashboard/$section',
      params: { section: DASHBOARD_DEFAULT_SECTION },
    })
  },
})
