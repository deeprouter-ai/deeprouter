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
import { useIsCasual } from '@/hooks/use-casual'

/**
 * Conditional render wrapper that hides children for casual persona.
 * Use for advanced options we want kept out of casual sight (currency
 * toggle, custom amounts, redemption codes, advanced setting tabs).
 *
 * @example
 *   <HideForCasual>
 *     <CurrencyToggle />
 *     <CustomAmountInput />
 *   </HideForCasual>
 */
export function HideForCasual({
  children,
}: {
  children: React.ReactNode
}) {
  if (useIsCasual()) return null
  return <>{children}</>
}

/**
 * Conditional render wrapper that ONLY renders for casual persona.
 * Inverse of HideForCasual — use for cards / banners that are
 * specifically educational and would be noise for power users.
 */
export function CasualOnly({
  children,
}: {
  children: React.ReactNode
}) {
  if (!useIsCasual()) return null
  return <>{children}</>
}
