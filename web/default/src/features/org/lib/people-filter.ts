// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later

/**
 * What a list of people is narrowed down by. The "who is it for" list of a
 * key and the members table share it, so the two cannot come to work
 * differently. A department or role id of 0 leaves that one open.
 */
export type PeopleFilter = {
  search: string
  departmentId: number
  roleId: number
}

/** A department or a role a list of people can be narrowed down to. */
export type PeopleFacet = {
  id: number
  name: string
}

/** The choice of a filter that leaves it open. No department or role has id 0. */
export const ANY = 0

/**
 * Whether a person is still listed under a filter. The search looks for what
 * was typed anywhere in one of the given texts, whatever the case.
 */
export function personMatches(
  person: { department_id: number; role_id: number },
  texts: string[],
  filter: PeopleFilter
): boolean {
  if (filter.departmentId && person.department_id !== filter.departmentId) {
    return false
  }
  if (filter.roleId && person.role_id !== filter.roleId) return false
  const search = filter.search.trim().toLowerCase()
  return texts.some((text) => text.toLowerCase().includes(search))
}

/** The given facets, each once, in the order of their ids. */
export function distinctFacets(facets: PeopleFacet[]): PeopleFacet[] {
  const byId = new Map(facets.map((facet) => [facet.id, facet]))
  return [...byId.values()].sort((a, b) => a.id - b.id)
}
