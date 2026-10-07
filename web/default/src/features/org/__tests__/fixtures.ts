// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type {
  OrgInherentPower,
  OrgKey,
  OrgKeyHolder,
  OrgKeyTemplate,
  OrgMember,
  OrgMembership,
  OrgPermissionCatalog,
  OrgRole,
} from '../types'

// What the backend answers for Enterprise Org (meta-repo
// docs/enterprise-org-prd.md §2), written out once for every test of the
// organization pages. The pages decide what to offer from these answers, so a
// test with made-up permissions would be testing nothing.

/** The permission primitives, in the backend's order. */
export const PRIMITIVES = [
  'key.read',
  'key.create',
  'key.update',
  'key.assign',
  'key.rotate',
  'key.freeze',
  'key.delete',
  'member.read',
  'member.invite',
  'member.remove',
  'usage.read',
  'alert.read',
  'audit.read',
]

/** The inherent powers, in the backend's order. */
export const POWERS: OrgInherentPower[] = [
  { name: 'role.manage', owner_only: false },
  { name: 'department.manage', owner_only: false },
  { name: 'service_account.manage', owner_only: false },
  { name: 'org.settings', owner_only: false },
  { name: 'alert.handle', owner_only: false },
  { name: 'admin.appoint', owner_only: true },
  { name: 'wallet.manage', owner_only: true },
  { name: 'org.transfer', owner_only: true },
]

const ADMIN_POWERS = POWERS.filter((p) => !p.owner_only).map((p) => p.name)
const OWNER_POWERS = POWERS.map((p) => p.name)

/** The five preset roles as `GET /api/org/roles` lists them. */
export const presetRoles: OrgRole[] = [
  {
    id: 1,
    name: 'owner',
    scope: 'org',
    permissions: PRIMITIVES,
    powers: OWNER_POWERS,
    is_preset: true,
  },
  {
    id: 2,
    name: 'admin',
    scope: 'org',
    permissions: PRIMITIVES,
    powers: ADMIN_POWERS,
    is_preset: true,
  },
  {
    id: 3,
    name: 'manager',
    scope: 'dept',
    permissions: [
      'key.read',
      'key.assign',
      'member.read',
      'member.invite',
      'usage.read',
      'alert.read',
    ],
    powers: [],
    is_preset: true,
  },
  {
    id: 4,
    name: 'staff',
    scope: 'self',
    permissions: [],
    powers: [],
    is_preset: true,
  },
  {
    id: 5,
    name: 'readonly',
    scope: 'org',
    permissions: [
      'key.read',
      'member.read',
      'usage.read',
      'alert.read',
      'audit.read',
    ],
    powers: [],
    is_preset: true,
  },
]

/** What `GET /api/org/permissions` answers. */
export const catalog: OrgPermissionCatalog = {
  primitives: PRIMITIVES,
  powers: POWERS,
  role_packs: [
    {
      key: 'it_ops',
      name: 'IT Ops',
      scope: 'org',
      permissions: [
        'key.read',
        'key.create',
        'key.update',
        'key.assign',
        'key.rotate',
        'key.freeze',
        'key.delete',
        'member.read',
        'usage.read',
        'alert.read',
      ],
    },
    {
      key: 'hr_ops',
      name: 'HR Ops',
      scope: 'org',
      permissions: [
        'key.read',
        'key.freeze',
        'key.delete',
        'member.read',
        'member.invite',
        'member.remove',
      ],
    },
    {
      key: 'finance',
      name: 'Finance Ops',
      scope: 'org',
      permissions: ['usage.read'],
    },
  ],
}

type PresetName = 'owner' | 'admin' | 'manager' | 'staff' | 'readonly'

/**
 * A member's place in Acme as `GET /api/org/self` reports it for a preset
 * role, in department 10. A manager manages that department.
 */
export function membershipOf(
  role: PresetName,
  over: Partial<OrgMembership> = {}
): OrgMembership {
  const preset = presetRoles.find((r) => r.name === role) as OrgRole
  return {
    org_id: 1,
    org_name: 'Acme',
    is_owner: role === 'owner',
    is_admin: role === 'admin',
    role_id: preset.id,
    role,
    role_scope: preset.scope,
    permissions: preset.permissions,
    department_id: 10,
    managed_department_ids: preset.scope === 'dept' ? [10] : [],
    ...over,
  }
}

/** A staff member of department 10 with the given fields changed. */
export function memberOf(over: Partial<OrgMember>): OrgMember {
  return {
    id: 10,
    username: 'someone',
    display_name: '',
    email: '',
    role_id: 4,
    role: 'staff',
    department_id: 10,
    is_owner: false,
    is_service: false,
    managed_department_ids: [],
    ...over,
  }
}

/**
 * A working key held by sally, a person in department 11, as `GET
 * /api/org/keys` lists it: the value masked, no template, 10 dollars left.
 */
export function keyOf(over: Partial<OrgKey> = {}): OrgKey {
  return {
    id: 100,
    name: 'Design tools',
    key: 'abcd**********wxyz',
    status: 1,
    holder_id: 3,
    holder: 'sally',
    holder_is_service: false,
    department_id: 11,
    department: 'Sales',
    policy_template: '',
    model_limits: [],
    remain_quota: 5000000,
    used_quota: 0,
    unlimited_quota: false,
    expired_time: -1,
    rpm_limit: 0,
    tpm_limit: 0,
    monthly_limit: 0,
    created_time: 1790000000,
    accessed_time: 1790000000,
    ...over,
  }
}

/** Whom a key can be made out to, as `GET /api/org/key-holders` answers an admin. */
export const keyHolders: OrgKeyHolder[] = [
  {
    id: 1,
    name: 'Fiona Founder',
    department_id: 10,
    department: 'General',
    role_id: 1,
    role: 'owner',
    is_service: false,
    is_owner: true,
  },
  {
    id: 3,
    name: 'sally',
    department_id: 11,
    department: 'Sales',
    role_id: 4,
    role: 'staff',
    is_service: false,
    is_owner: false,
  },
  {
    id: 4,
    name: 'CI Pipeline',
    department_id: 10,
    department: 'General',
    role_id: 4,
    role: 'staff',
    is_service: true,
    is_owner: false,
  },
]

/** What `GET /api/org/key-templates` answers. */
export const keyTemplates: OrgKeyTemplate[] = [
  { key: 'creative', purposes: ['image', 'video', 'chat'] },
  { key: 'coding', purposes: ['coding'] },
]
