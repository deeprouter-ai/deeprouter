/*
Copyright (C) 2026 DeepRouter

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
*/
import type { MarketplaceSkill } from '../types'

// PRD §5.4: skill prices are always shown in USD, exactly $X.XX.
export function skillPriceLabel(
  skill: Pick<MarketplaceSkill, 'monetization_type' | 'price_usd'>,
  freeLabel: string
): string {
  return skill.monetization_type === 'free'
    ? freeLabel
    : `$${skill.price_usd.toFixed(2)}`
}
