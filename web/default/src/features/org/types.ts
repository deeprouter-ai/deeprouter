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
  /** How many of the organization's keys they hold — what removing them takes back. */
  key_count: number
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

/**
 * One key of the organization, as the keys page lists it. `key` is the masked
 * form: the value itself never reaches the page through this type (PRD D15).
 */
export type OrgKey = {
  id: number
  name: string
  key: string
  /** 1 enabled, 2 frozen, 3 expired, 4 out of quota — as the gateway stores it. */
  status: number
  holder_id: number
  /** Empty when that account no longer exists. */
  holder: string
  holder_is_service: boolean
  /**
   * The key has been handed to nobody: it is parked under the owner, where a
   * new key starts out and one that was taken back returns.
   */
  holder_is_owner: boolean
  /** The holder's department. */
  department_id: number
  department: string
  /** The key of the policy template applied to it; empty for none. */
  policy_template: string
  /** What it may call; empty means every model. */
  model_limits: string[]
  remain_quota: number
  used_quota: number
  unlimited_quota: boolean
  /** Unix seconds; -1 means never. */
  expired_time: number
  rpm_limit: number
  tpm_limit: number
  monthly_limit: number
  created_time: number
  accessed_time: number
}

/**
 * A key together with its value. The value is there only for a service
 * account's key, and only in the answer to the request that made that value —
 * a creation, a rotation, or handing the key to the account. A person's key
 * never has one.
 */
export type OrgKeyGrant = OrgKey & { value?: string }

/**
 * One of the caller's own keys as far as one purpose of the console goes:
 * `models` are the models of that purpose the key may call, where the purpose
 * names models at all.
 */
export type OrgOwnKey = {
  id: number
  name: string
  models: string[]
}

/** A member a key can be made out to, or handed to. */
export type OrgKeyHolder = {
  id: number
  name: string
  department_id: number
  department: string
  /**
   * The role the member holds. A viewer who may not read members is told
   * nobody's: the id is 0 and the name empty.
   */
  role_id: number
  role: string
  is_service: boolean
  is_owner: boolean
}

/** A ready-made model allowance for a key: a set of purposes. */
export type OrgKeyTemplate = {
  key: string
  purposes: string[]
}

/**
 * What a key is created with. A `holder_id` of 0 parks it under the owner.
 * What it may call is a policy template or a hand-picked list of models, never
 * both; with neither it may call every model.
 */
export type OrgKeyInput = {
  name: string
  holder_id: number
  policy_template: string
  model_limits: string[]
  remain_quota: number
  unlimited_quota: boolean
  expired_time: number
  rpm_limit: number
  tpm_limit: number
  monthly_limit: number
}

/**
 * A change to one key; a field left out stays as it is. `policy_template` and
 * `model_limits` are one thing — what the key may call — and are replaced
 * together: when one is sent without the other, the other counts as empty. A
 * template is applied afresh, also when it names the one the key already has.
 */
export type OrgKeyPatch = Partial<Omit<OrgKeyInput, 'holder_id'>>

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
