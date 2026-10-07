// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import {
  holderDepartments,
  holderMatches,
  holderRoles,
  holdersInOrder,
  type HolderFilter,
} from '../lib/holders'
import type { OrgKeyHolder } from '../types'

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3, D34): finding
// who a key is for in a company too large to scroll through — by name, by
// department and by role. These are the rules the picker applies; the page
// test walks the picker itself.

/** A holder: staff of Sales unless said otherwise. */
function holder(id: number, name: string, over: Partial<OrgKeyHolder> = {}) {
  return {
    id,
    name,
    department_id: 11,
    department: 'Sales',
    role_id: 4,
    role: 'staff',
    is_service: false,
    is_owner: false,
    ...over,
  }
}

const general = { department_id: 10, department: 'General' }
const product = { department_id: 12, department: 'Product' }
const fiona = holder(1, 'Fiona Founder', {
  ...general,
  role_id: 1,
  role: 'owner',
  is_owner: true,
})
const sally = holder(3, 'sally')
const bot = holder(4, 'CI Pipeline', { ...general, is_service: true })
const mark = holder(5, 'Mark', { role_id: 3, role: 'manager' })
const paula = holder(6, 'paula', { ...product, role_id: 31, role: 'IT Ops' })
const everyone = [sally, paula, bot, fiona, mark]

/** A filter that leaves everything open, but for what is given. */
function filter(over: Partial<HolderFilter> = {}): HolderFilter {
  return { search: '', departmentId: 0, roleId: 0, ...over }
}

/** The names of the holders a filter leaves, in the order given. */
function namesUnder(over: Partial<HolderFilter>): string[] {
  return everyone
    .filter((each) => holderMatches(each, filter(over)))
    .map((each) => each.name)
}

describe('the order holders are listed in', () => {
  it('puts the owner first and everyone else by name, whatever the case', () => {
    expect(holdersInOrder(everyone, 'en').map((each) => each.name)).toEqual([
      'Fiona Founder',
      'CI Pipeline',
      'Mark',
      'paula',
      'sally',
    ])
  })

  it('sorts names the way the language of the page does', () => {
    const names = ['张伟', '李娜', '陈静'].map((name, i) =>
      holder(20 + i, name)
    )
    // By pronunciation — chen, li, zhang — not by where the characters happen
    // to sit in the character set.
    expect(holdersInOrder(names, 'zh').map((each) => each.name)).toEqual([
      '陈静',
      '李娜',
      '张伟',
    ])
  })

  it('leaves the list it was given as it was', () => {
    const given = [...everyone]
    holdersInOrder(given, 'en')
    expect(given).toEqual(everyone)
  })
})

describe('narrowing the list down', () => {
  it('leaves everyone while nothing is asked for', () => {
    expect(namesUnder({})).toEqual(everyone.map((each) => each.name))
    expect(namesUnder({ search: '   ' })).toEqual(
      everyone.map((each) => each.name)
    )
  })

  it('finds a name by any part of it, whatever the case', () => {
    expect(namesUnder({ search: 'fio' })).toEqual(['Fiona Founder'])
    expect(namesUnder({ search: 'FOUND' })).toEqual(['Fiona Founder'])
    expect(namesUnder({ search: '  pipe ' })).toEqual(['CI Pipeline'])
    expect(namesUnder({ search: 'a' })).toEqual([
      'sally',
      'paula',
      'Fiona Founder',
      'Mark',
    ])
    expect(namesUnder({ search: 'nobody' })).toEqual([])
  })

  it('does not look in the department or the role for the name', () => {
    expect(namesUnder({ search: 'Sales' })).toEqual([])
    expect(namesUnder({ search: 'staff' })).toEqual([])
  })

  it('keeps one department', () => {
    expect(namesUnder({ departmentId: 11 })).toEqual(['sally', 'Mark'])
    expect(namesUnder({ departmentId: 10 })).toEqual([
      'CI Pipeline',
      'Fiona Founder',
    ])
    expect(namesUnder({ departmentId: 99 })).toEqual([])
  })

  it('keeps one role', () => {
    expect(namesUnder({ roleId: 4 })).toEqual(['sally', 'CI Pipeline'])
    expect(namesUnder({ roleId: 31 })).toEqual(['paula'])
  })

  it('asks for all of it at once', () => {
    expect(namesUnder({ departmentId: 11, roleId: 4 })).toEqual(['sally'])
    expect(namesUnder({ departmentId: 10, roleId: 4, search: 'ci' })).toEqual([
      'CI Pipeline',
    ])
    expect(namesUnder({ departmentId: 11, roleId: 4, search: 'mark' })).toEqual(
      []
    )
  })
})

describe('what the list can be narrowed down to', () => {
  it('offers each department once, the oldest first', () => {
    expect(holderDepartments(everyone)).toEqual([
      { id: 10, name: 'General' },
      { id: 11, name: 'Sales' },
      { id: 12, name: 'Product' },
    ])
  })

  it('offers each role once, the presets before the organization’s own', () => {
    expect(holderRoles(everyone)).toEqual([
      { id: 1, name: 'owner' },
      { id: 3, name: 'manager' },
      { id: 4, name: 'staff' },
      { id: 31, name: 'IT Ops' },
    ])
  })

  it('offers no role to a viewer who is told nobody’s', () => {
    const untold = everyone.map((each) => ({ ...each, role_id: 0, role: '' }))
    expect(holderRoles(untold)).toEqual([])
    // The departments are still there to narrow down by.
    expect(holderDepartments(untold)).toHaveLength(3)
  })
})
