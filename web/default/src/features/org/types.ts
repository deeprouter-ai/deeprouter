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
  role: string
  role_scope: string
  permissions: string[]
  department_id: number
}

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
  scope: string
  permissions: string[]
  is_preset: boolean
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
