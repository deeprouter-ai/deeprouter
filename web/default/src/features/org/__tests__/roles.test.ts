// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import { canManageOrg, canSeeOrg } from '../hooks/use-org-membership'
import {
  groupByResource,
  holds,
  impliedReads,
  permissionHint,
  permissionLabel,
  resourceLabel,
  rolePackHint,
  rolePackLabel,
  scopeLabel,
} from '../lib/permissions'
import {
  assignableRoles,
  managesDepartments,
  orgInviteLink,
  orgRoleLabel,
  roleLockReason,
  sitsInDefaultDepartment,
} from '../lib/roles'
import type { OrgRole } from '../types'
import {
  catalog,
  memberOf,
  membershipOf,
  POWERS,
  presetRoles,
  PRIMITIVES,
} from './fixtures'

// Enterprise Org P3 and P4: what the pages offer must match what the backend
// allows. The backend refuses the rest anyway — these keep impossible choices
// off the screen.

/** A translator that answers with the key, so tests read the English. */
const t = ((key: string) => key) as unknown as TFunction

// A company may name a custom role anything; "admin" only gets past the
// backend's name check if written by hand, and must still not count as one.
const lookalike: OrgRole = {
  id: 9,
  name: 'admin',
  scope: 'org',
  permissions: PRIMITIVES,
  powers: [],
  is_preset: false,
}
const roles: OrgRole[] = [...presetRoles, lookalike]

const owner = membershipOf('owner')
const admin = membershipOf('admin')
const manager = membershipOf('manager')
const readonly = membershipOf('readonly')
const staff = membershipOf('staff')

describe('canManageOrg', () => {
  it('is the owner and the admins, and nobody else', () => {
    expect(canManageOrg(owner)).toBe(true)
    expect(canManageOrg(admin)).toBe(true)
    expect(canManageOrg(manager)).toBe(false)
    expect(canManageOrg(readonly)).toBe(false)
    expect(canManageOrg(staff)).toBe(false)
    // Holding every primitive is still not running the organization.
    expect(
      canManageOrg(membershipOf('staff', { permissions: PRIMITIVES }))
    ).toBe(false)
  })

  it('is nobody without an organization', () => {
    expect(canManageOrg(null)).toBe(false)
    expect(canManageOrg(undefined)).toBe(false)
  })
})

describe('canSeeOrg', () => {
  it('is whoever may see members: not staff, not someone who only sees usage', () => {
    for (const member of [owner, admin, manager, readonly]) {
      expect(canSeeOrg(member)).toBe(true)
    }
    expect(canSeeOrg(staff)).toBe(false)
    expect(
      canSeeOrg(membershipOf('staff', { permissions: ['usage.read'] }))
    ).toBe(false)
    expect(canSeeOrg(null)).toBe(false)
    expect(canSeeOrg(undefined)).toBe(false)
  })
})

describe('holds', () => {
  it('looks the primitive up in what the backend reported', () => {
    expect(holds(manager, 'member.invite')).toBe(true)
    expect(holds(manager, 'member.remove')).toBe(false)
    expect(holds(readonly, 'audit.read')).toBe(true)
    expect(holds(readonly, 'member.invite')).toBe(false)
    expect(holds(staff, 'key.read')).toBe(false)
    expect(holds(null, 'key.read')).toBe(false)
  })

  it('never treats an inherent power as something a role lists', () => {
    for (const power of POWERS) {
      expect(holds(owner, power.name)).toBe(false)
    }
  })
})

describe('assignableRoles', () => {
  it('never offers the owner role', () => {
    for (const actor of [owner, admin, manager]) {
      expect(assignableRoles(roles, actor).map((r) => r.id)).not.toContain(1)
    }
  })

  it('offers the admin role to the owner only', () => {
    expect(assignableRoles(roles, owner).map((r) => r.id)).toEqual([
      2, 3, 4, 5, 9,
    ])
    expect(assignableRoles(roles, admin).map((r) => r.id)).toEqual([3, 4, 5, 9])
  })

  it('offers Staff alone to anyone who does not assign roles', () => {
    // PRD §2: nobody gives what they do not have. A manager who may invite
    // invites Staff — not managers, not a custom role, not a lookalike.
    expect(assignableRoles(roles, manager).map((r) => r.id)).toEqual([4])
    expect(assignableRoles(roles, readonly).map((r) => r.id)).toEqual([4])
    expect(assignableRoles(roles, null).map((r) => r.id)).toEqual([4])
  })
})

describe('roleLockReason', () => {
  it('locks the owner for everyone, the owner included', () => {
    const target = memberOf({ is_owner: true, role_id: 1, role: 'owner' })
    expect(roleLockReason(t, target, roles, owner)).toBe(
      "The owner's role cannot be changed."
    )
    expect(roleLockReason(t, target, roles, admin)).toBe(
      "The owner's role cannot be changed."
    )
  })

  it('locks a service account', () => {
    expect(
      roleLockReason(t, memberOf({ is_service: true }), roles, owner)
    ).toBe('Service accounts always have the Staff role.')
  })

  it('locks an admin for other admins, not for the owner', () => {
    const target = memberOf({ role_id: 2, role: 'admin' })
    expect(roleLockReason(t, target, roles, admin)).toBe(
      'Only the owner can appoint or dismiss admins.'
    )
    expect(roleLockReason(t, target, roles, owner)).toBeNull()
  })

  it('leaves everyone else editable, a custom role named admin included', () => {
    expect(roleLockReason(t, memberOf({}), roles, admin)).toBeNull()
    expect(
      roleLockReason(t, memberOf({ role_id: 9, role: 'admin' }), roles, admin)
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

describe('managesDepartments', () => {
  it('is every role with department scope, preset or custom', () => {
    expect(roles.filter(managesDepartments).map((r) => r.id)).toEqual([3])
    expect(managesDepartments({ ...lookalike, scope: 'dept' })).toBe(true)
    expect(managesDepartments(undefined)).toBe(false)
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

describe('permission words', () => {
  it('has a name and a line of explanation for everything the backend lists', () => {
    // A primitive or power without words would show as its code — fine for
    // one the backend adds first, a gap for the ones that exist today.
    for (const name of [...PRIMITIVES, ...POWERS.map((p) => p.name)]) {
      expect(permissionLabel(t, name), name).not.toBe(name)
      expect(permissionHint(t, name), name).not.toBe('')
    }
    for (const { resource } of groupByResource(PRIMITIVES)) {
      expect(resourceLabel(t, resource), resource).not.toBe(resource)
    }
    for (const scope of ['org', 'dept', 'self']) {
      expect(scopeLabel(t, scope), scope).not.toBe(scope)
    }
    for (const pack of catalog.role_packs) {
      expect(rolePackHint(t, pack), pack.key).not.toBe('')
    }
    expect(catalog.role_packs.map((pack) => rolePackLabel(t, pack))).toEqual([
      'IT Ops',
      'HR Ops',
      'Finance Ops',
    ])
  })

  it('shows what it has no words for as the backend sent it', () => {
    expect(permissionLabel(t, 'key.share')).toBe('key.share')
    expect(permissionHint(t, 'key.share')).toBe('')
    expect(resourceLabel(t, 'budget')).toBe('budget')
    expect(
      rolePackLabel(t, {
        key: 'legal',
        name: 'Legal',
        scope: 'org',
        permissions: [],
      })
    ).toBe('Legal')
  })

  it('groups the primitives under their resource, in the order given', () => {
    expect(groupByResource(PRIMITIVES).map((g) => g.resource)).toEqual([
      'key',
      'member',
      'usage',
      'alert',
      'audit',
    ])
    expect(groupByResource(PRIMITIVES)[1].primitives).toEqual([
      'member.read',
      'member.invite',
      'member.remove',
    ])
  })
})

describe('impliedReads', () => {
  it('is the read of every resource a write was picked from', () => {
    // PRD §2: 写权限自动包含同一资源的读权限.
    expect([...impliedReads(['key.assign', 'member.remove'])].sort()).toEqual([
      'key.read',
      'member.read',
    ])
    expect([...impliedReads(['key.freeze', 'key.delete'])]).toEqual([
      'key.read',
    ])
  })

  it('is nothing for reads alone', () => {
    expect(impliedReads(['key.read', 'usage.read']).size).toBe(0)
    expect(impliedReads([]).size).toBe(0)
  })
})
