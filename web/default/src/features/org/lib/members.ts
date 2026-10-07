// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { OrgDepartment, OrgMember } from '../types'
import {
  distinctFacets,
  personMatches,
  type PeopleFacet,
  type PeopleFilter,
} from './people-filter'

/**
 * Whether a member is still listed under a filter. The search looks in what
 * the members table shows of a person: the name, the username and the email.
 */
export function memberMatches(
  member: OrgMember,
  filter: PeopleFilter
): boolean {
  return personMatches(
    member,
    [member.display_name, member.username, member.email],
    filter
  )
}

/**
 * The departments the members sit in, oldest first — which puts the default
 * department first, as the departments tab does. A department the viewer is
 * not told the name of is left out: there would be nothing to call the choice.
 */
export function memberDepartments(
  members: OrgMember[],
  departments: OrgDepartment[]
): PeopleFacet[] {
  const occupied = new Set(members.map((member) => member.department_id))
  return distinctFacets(
    departments
      .filter((department) => occupied.has(department.id))
      .map((department) => ({ id: department.id, name: department.name }))
  )
}

/**
 * The roles the members hold, the presets first and then the organization's
 * own, as the roles page lists them.
 */
export function memberRoles(members: OrgMember[]): PeopleFacet[] {
  return distinctFacets(
    members.map((member) => ({ id: member.role_id, name: member.role }))
  )
}
