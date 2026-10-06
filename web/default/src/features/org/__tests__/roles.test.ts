// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import { canManageOrg } from '../hooks/use-org-membership'
import {
  assignableRoles,
  orgInviteLink,
  orgRoleLabel,
  roleLockReason,
  sitsInDefaultDepartment,
} from '../lib/roles'
import type { OrgMember, OrgMembership, OrgRole } from '../types'

// Enterprise Org P3: what the page offers must match what the backend allows.
// The backend refuses the rest anyway — these keep impossible choices off the
// screen.

/** A translator that answers with the key, so tests read the English. */
const t = ((key: string) => key) as unknown as TFunction

const roles: OrgRole[] = [
  { id: 1, name: 'owner', scope: 'org', permissions: [], is_preset: true },
  { id: 2, name: 'admin', scope: 'org', permissions: [], is_preset: true },
  { id: 3, name: 'manager', scope: 'dept', permissions: [], is_preset: true },
  { id: 4, name: 'staff', scope: 'self', permissions: [], is_preset: true },
  { id: 5, name: 'readonly', scope: 'org', permissions: [], is_preset: true },
  // A company may name a custom role anything, "admin" included.
  { id: 9, name: 'admin', scope: 'org', permissions: [], is_preset: false },
]

/** A staff membership with the given fields changed. */
function membership(over: Partial<OrgMembership>): OrgMembership {
  return {
    org_id: 1,
    org_name: 'Acme',
    is_owner: false,
    is_admin: false,
    role: 'staff',
    role_scope: 'self',
    permissions: [],
    department_id: 1,
    ...over,
  }
}

/** A staff member with the given fields changed. */
function member(over: Partial<OrgMember>): OrgMember {
  return {
    id: 10,
    username: 'someone',
    display_name: '',
    email: '',
    role_id: 4,
    role: 'staff',
    department_id: 1,
    is_owner: false,
    is_service: false,
    ...over,
  }
}

const owner = membership({ is_owner: true, role: 'owner' })
const admin = membership({ is_admin: true, role: 'admin' })

describe('canManageOrg', () => {
  it('is the owner and the admins, and nobody else', () => {
    expect(canManageOrg(owner)).toBe(true)
    expect(canManageOrg(admin)).toBe(true)
    expect(canManageOrg(membership({ role: 'manager' }))).toBe(false)
    expect(canManageOrg(membership({ role: 'readonly' }))).toBe(false)
    expect(canManageOrg(membership({}))).toBe(false)
  })

  it('is nobody without an organization', () => {
    expect(canManageOrg(null)).toBe(false)
    expect(canManageOrg(undefined)).toBe(false)
  })
})

describe('assignableRoles', () => {
  it('never offers the owner role', () => {
    for (const actor of [owner, admin]) {
      expect(assignableRoles(roles, actor).map((r) => r.id)).not.toContain(1)
    }
  })

  it('offers the admin role to the owner only', () => {
    expect(assignableRoles(roles, owner).map((r) => r.id)).toEqual([
      2, 3, 4, 5, 9,
    ])
    expect(assignableRoles(roles, admin).map((r) => r.id)).toEqual([3, 4, 5, 9])
  })
})

describe('roleLockReason', () => {
  it('locks the owner for everyone, the owner included', () => {
    const target = member({ is_owner: true, role_id: 1, role: 'owner' })
    expect(roleLockReason(t, target, roles, owner)).toBe(
      "The owner's role cannot be changed."
    )
    expect(roleLockReason(t, target, roles, admin)).toBe(
      "The owner's role cannot be changed."
    )
  })

  it('locks a service account', () => {
    expect(roleLockReason(t, member({ is_service: true }), roles, owner)).toBe(
      'Service accounts always have the Staff role.'
    )
  })

  it('locks an admin for other admins, not for the owner', () => {
    const target = member({ role_id: 2, role: 'admin' })
    expect(roleLockReason(t, target, roles, admin)).toBe(
      'Only the owner can appoint or dismiss admins.'
    )
    expect(roleLockReason(t, target, roles, owner)).toBeNull()
  })

  it('leaves everyone else editable, a custom role named admin included', () => {
    expect(roleLockReason(t, member({}), roles, admin)).toBeNull()
    expect(
      roleLockReason(t, member({ role_id: 9, role: 'admin' }), roles, admin)
    ).toBeNull()
  })
})

describe('sitsInDefaultDepartment', () => {
  it('is the owner and the admins — the presets, not a custom role named like one', () => {
    const sitting = roles.filter(sitsInDefaultDepartment).map((r) => r.id)
    expect(sitting).toEqual([1, 2])
    expect(sitsInDefaultDepartment(undefined)).toBe(false)
  })
})

describe('orgRoleLabel', () => {
  it('translates the five presets and shows a custom role as named', () => {
    expect(
      ['owner', 'admin', 'manager', 'staff', 'readonly'].map((role) =>
        orgRoleLabel(t, role)
      )
    ).toEqual(['Owner', 'Admin', 'Manager', 'Staff', 'Read-only'])
    expect(orgRoleLabel(t, 'IT Ops')).toBe('IT Ops')
  })
})

describe('orgInviteLink', () => {
  it('points at the sign-up page of this site, carrying the code', () => {
    expect(orgInviteLink('AbC123')).toBe(
      `${window.location.origin}/sign-up?org_invite=AbC123`
    )
  })
})
