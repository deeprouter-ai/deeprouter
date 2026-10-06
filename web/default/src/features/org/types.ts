// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later

/**
 * Enterprise Org (meta-repo `docs/enterprise-org-prd.md`): the shapes the
 * `/api/org/*` endpoints answer with. Org roles are their own axis — nothing
 * here relates to the platform `user.role` (1 / 10 / 100).
 */

/** The caller's place in their organization; `null` for a personal account. */
export type OrgMembership = {
  org_id: number
  org_name: string
  is_owner: boolean
  /** Holds the preset admin role. */
  is_admin: boolean
  role_id: number
  role: string
  role_scope: OrgRoleScope
  /**
   * Everything the role grants, the reads its writes bring included — look a
   * primitive up in it, there is no rule to apply on top.
   */
  permissions: string[]
  department_id: number
  /**
   * Where a department-scoped role reaches: the member's own department
   * first, then the ones added for them. Empty for every other scope.
   */
  managed_department_ids: number[]
}

/**
 * How far a role reaches: the whole organization, the departments its holder
 * manages, or — the preset Staff only — nothing but the holder's own keys.
 */
export type OrgRoleScope = 'org' | 'dept' | 'self'

export type OrgDepartment = {
  id: number
  name: string
  /** The catch-all department: it can be renamed, never deleted. */
  is_default: boolean
  member_count: number
}

export type OrgRole = {
  id: number
  name: string
  scope: OrgRoleScope
  permissions: string[]
  /**
   * The inherent powers that come with holding the role: some for the owner
   * and admin presets, none for any other role.
   */
  powers: string[]
  is_preset: boolean
}

/** What a custom role is made of, as the role endpoints take it. */
export type OrgRoleInput = {
  name: string
  scope: OrgRoleScope
  permissions: string[]
}

/** A power that comes with being the owner or an admin; no role can grant it. */
export type OrgInherentPower = {
  name: string
  owner_only: boolean
}

/** A ready-made custom role the platform offers for adoption. */
export type OrgRolePack = {
  key: string
  name: string
  scope: OrgRoleScope
  permissions: string[]
}

/**
 * Everything a role can be made of. The roles page draws its matrix from this
 * and keeps no list of its own, so it cannot drift from what the backend
 * enforces.
 */
export type OrgPermissionCatalog = {
  primitives: string[]
  powers: OrgInherentPower[]
  role_packs: OrgRolePack[]
}

export type OrgMember = {
  id: number
  username: string
  display_name: string
  email: string
  role_id: number
  role: string
  department_id: number
  is_owner: boolean
  /** A service account: holds keys, cannot sign in. */
  is_service: boolean
  /** The departments a member with a department-scoped role manages, their own first. */
  managed_department_ids: number[]
}

export type OrgInvite = {
  id: number
  code: string
  role_id: number
  role: string
  department_id: number
  /** Unix seconds. */
  expires_time: number
}

/** What an invite link leads to, shown on the sign-up page before joining. */
export type OrgInvitePreview = {
  org_name: string
  role: string
  department: string
}

export type OrgApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}
