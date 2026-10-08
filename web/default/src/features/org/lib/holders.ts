// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { OrgKeyHolder } from '../types'
import {
  distinctFacets,
  personMatches,
  type PeopleFacet,
  type PeopleFilter,
} from './people-filter'

/**
 * The holders in the order they are listed: the owner first — that row is
 * where a key waits until it is handed out — then everyone else by name, the
 * way the given language sorts names.
 */
export function holdersInOrder(
  holders: OrgKeyHolder[],
  locale: string
): OrgKeyHolder[] {
  const collator = new Intl.Collator(locale)
  return [...holders].sort((a, b) => {
    if (a.is_owner !== b.is_owner) return a.is_owner ? -1 : 1
    return collator.compare(a.name, b.name)
  })
}

/**
 * Whether a holder is still listed under a filter. The search looks for what
 * was typed anywhere in the name, whatever the case.
 */
export function holderMatches(
  holder: OrgKeyHolder,
  filter: PeopleFilter
): boolean {
  return personMatches(holder, [holder.name], filter)
}

/**
 * The departments the holders sit in, oldest first — which puts the default
 * department first, as the departments page does.
 */
export function holderDepartments(holders: OrgKeyHolder[]): PeopleFacet[] {
  return distinctFacets(
    holders.map((holder) => ({
      id: holder.department_id,
      name: holder.department,
    }))
  )
}

/**
 * The roles the holders have, the presets first and then the organization's
 * own, as the roles page lists them. Empty for a viewer who is not told
 * anybody's role, and there is then nothing to narrow down by.
 */
export function holderRoles(holders: OrgKeyHolder[]): PeopleFacet[] {
  return distinctFacets(
    holders
      .filter((holder) => holder.role_id > 0)
      .map((holder) => ({ id: holder.role_id, name: holder.role }))
  )
}
