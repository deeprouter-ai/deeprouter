// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { memberDepartments, memberMatches, memberRoles } from '../lib/members'
import type { PeopleFilter } from '../lib/people-filter'
import type { OrgDepartment } from '../types'
import { memberOf } from './fixtures'

// Enterprise Org (meta-repo docs/enterprise-org-prd.md D39): finding a member
// in a list too long to scroll through — by what the list shows of them, by
// department and by role. The rules are the ones the "who is it for" list of
// a key goes by (D34, holders.test.ts); what is pinned here is what the
// members table adds to them. The page test walks the table itself.

const fiona = memberOf({
  id: 1,
  username: 'founder',
  display_name: 'Fiona Founder',
  email: 'fiona@acme.test',
  role_id: 1,
  role: 'owner',
  is_owner: true,
})
const sally = memberOf({ id: 3, username: 'sally', department_id: 11 })
const bot = memberOf({
  id: 4,
  username: 'svc-abc123def456',
  display_name: 'CI Pipeline',
  is_service: true,
})
// Sits in Sales and manages Product on top of it.
const mona = memberOf({
  id: 5,
  username: 'mona',
  email: 'Mona.Lee@acme.test',
  role_id: 3,
  role: 'manager',
  department_id: 11,
  managed_department_ids: [11, 12],
})
const paula = memberOf({
  id: 6,
  username: 'paula',
  role_id: 31,
  role: 'IT Ops',
  department_id: 12,
})
const everyone = [sally, paula, bot, fiona, mona]

const departments: OrgDepartment[] = [
  { id: 12, name: 'Product', is_default: false, member_count: 1 },
  { id: 10, name: 'General', is_default: true, member_count: 2 },
  { id: 11, name: 'Sales', is_default: false, member_count: 2 },
  { id: 13, name: 'Support', is_default: false, member_count: 0 },
]

/** A filter that leaves everything open, but for what is given. */
function filter(over: Partial<PeopleFilter> = {}): PeopleFilter {
  return { search: '', departmentId: 0, roleId: 0, ...over }
}

/** The usernames of the members a filter leaves, in the order given. */
function usernamesUnder(over: Partial<PeopleFilter>): string[] {
  return everyone
    .filter((each) => memberMatches(each, filter(over)))
    .map((each) => each.username)
}

describe('narrowing the members down', () => {
  it('leaves everyone while nothing is asked for', () => {
    expect(usernamesUnder({})).toHaveLength(everyone.length)
    expect(usernamesUnder({ search: '   ' })).toHaveLength(everyone.length)
  })

  it('finds a member by any part of the name, the username or the email', () => {
    // The name a person gave themselves.
    expect(usernamesUnder({ search: 'fiona f' })).toEqual(['founder'])
    // What they sign in with, which the row shows under the name.
    expect(usernamesUnder({ search: 'found' })).toEqual(['founder'])
    expect(usernamesUnder({ search: 'svc-' })).toEqual(['svc-abc123def456'])
    // The email, whatever the case on either side.
    expect(usernamesUnder({ search: 'mona.lee@' })).toEqual(['mona'])
    expect(usernamesUnder({ search: '  ACME.TEST ' })).toEqual([
      'founder',
      'mona',
    ])
    expect(usernamesUnder({ search: 'nobody' })).toEqual([])
  })

  it('does not look in the department or the role for what was typed', () => {
    expect(usernamesUnder({ search: 'Sales' })).toEqual([])
    expect(usernamesUnder({ search: 'manager' })).toEqual([])
  })

  it('keeps the department a member sits in, not one they manage besides', () => {
    expect(usernamesUnder({ departmentId: 11 })).toEqual(['sally', 'mona'])
    expect(usernamesUnder({ departmentId: 12 })).toEqual(['paula'])
  })

  it('keeps one role', () => {
    expect(usernamesUnder({ roleId: 4 })).toEqual(['sally', 'svc-abc123def456'])
    expect(usernamesUnder({ roleId: 31 })).toEqual(['paula'])
  })

  it('asks for all of it at once', () => {
    expect(usernamesUnder({ departmentId: 11, roleId: 3 })).toEqual(['mona'])
    expect(
      usernamesUnder({ departmentId: 10, roleId: 4, search: 'pipe' })
    ).toEqual(['svc-abc123def456'])
    expect(
      usernamesUnder({ departmentId: 11, roleId: 4, search: 'mona' })
    ).toEqual([])
  })
})

describe('what the members can be narrowed down to', () => {
  it('offers the departments somebody listed sits in, the oldest first', () => {
    // Support has nobody: choosing it could only empty the table.
    expect(memberDepartments(everyone, departments)).toEqual([
      { id: 10, name: 'General' },
      { id: 11, name: 'Sales' },
      { id: 12, name: 'Product' },
    ])
  })

  it('offers no department the viewer is not told the name of', () => {
    const told = departments.filter((department) => department.id !== 12)
    expect(memberDepartments(everyone, told)).toEqual([
      { id: 10, name: 'General' },
      { id: 11, name: 'Sales' },
    ])
  })

  it('offers each role once, the presets before the organization’s own', () => {
    expect(memberRoles(everyone)).toEqual([
      { id: 1, name: 'owner' },
      { id: 3, name: 'manager' },
      { id: 4, name: 'staff' },
      { id: 31, name: 'IT Ops' },
    ])
  })
})
