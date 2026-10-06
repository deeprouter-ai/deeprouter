// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { api } from '@/lib/api'
import type {
  OrgApiResponse,
  OrgDepartment,
  OrgInvite,
  OrgInvitePreview,
  OrgMember,
  OrgMembership,
  OrgPermissionCatalog,
  OrgRole,
  OrgRoleInput,
} from './types'

export const orgQueryKeys = {
  all: ['org'] as const,
  self: () => [...orgQueryKeys.all, 'self'] as const,
  departments: () => [...orgQueryKeys.all, 'departments'] as const,
  roles: () => [...orgQueryKeys.all, 'roles'] as const,
  permissions: () => [...orgQueryKeys.all, 'permissions'] as const,
  members: () => [...orgQueryKeys.all, 'members'] as const,
  invites: () => [...orgQueryKeys.all, 'invites'] as const,
  invitePreview: (code: string) =>
    [...orgQueryKeys.all, 'invite-preview', code] as const,
}

/** The caller's organization and role, or null for a personal account. */
export async function fetchOrgMembership(): Promise<OrgMembership | null> {
  const res = await api.get('/api/org/self', {
    // A probe the sidebar runs for everyone: a failure must stay quiet.
    skipErrorHandler: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  return (res.data?.data as OrgMembership | null) ?? null
}

/** The departments the caller may see, the default one first, with head counts. */
export async function fetchOrgDepartments(): Promise<OrgDepartment[]> {
  const res = await api.get('/api/org/departments')
  return (res.data?.data as OrgDepartment[]) ?? []
}

/** The roles members of the organization can hold: the presets, then its own. */
export async function fetchOrgRoles(): Promise<OrgRole[]> {
  const res = await api.get('/api/org/roles')
  return (res.data?.data as OrgRole[]) ?? []
}

/** What roles are made of: the primitives, the inherent powers and the role packs. */
export async function fetchOrgPermissions(): Promise<OrgPermissionCatalog> {
  const res = await api.get('/api/org/permissions')
  return res.data?.data as OrgPermissionCatalog
}

/** Adds a custom role built from the primitives. */
export async function createOrgRole(
  input: OrgRoleInput
): Promise<OrgApiResponse<OrgRole>> {
  const res = await api.post('/api/org/roles', input)
  return res.data
}

/** Changes a custom role; its holders are judged by it from their next request. */
export async function updateOrgRole(
  id: number,
  input: OrgRoleInput
): Promise<OrgApiResponse<OrgRole>> {
  const res = await api.put(`/api/org/roles/${id}`, input)
  return res.data
}

/** Deletes a custom role; the members who held it become Staff. */
export async function deleteOrgRole(id: number): Promise<OrgApiResponse> {
  const res = await api.delete(`/api/org/roles/${id}`)
  return res.data
}

/**
 * Copies a platform role pack into the organization as a custom role, under
 * the name the page shows the pack by.
 */
export async function adoptOrgRolePack(
  key: string,
  name: string
): Promise<OrgApiResponse<OrgRole>> {
  const res = await api.post(
    `/api/org/role-packs/${encodeURIComponent(key)}/adopt`,
    { name }
  )
  return res.data
}

/** The members the caller may see, service accounts included. */
export async function fetchOrgMembers(): Promise<OrgMember[]> {
  const res = await api.get('/api/org/members')
  return (res.data?.data as OrgMember[]) ?? []
}

/** The usable invite links the caller could have issued themselves, newest first. */
export async function fetchOrgInvites(): Promise<OrgInvite[]> {
  const res = await api.get('/api/org/invites')
  return (res.data?.data as OrgInvite[]) ?? []
}

/** Adds a department. */
export async function createOrgDepartment(
  name: string
): Promise<OrgApiResponse<OrgDepartment>> {
  const res = await api.post('/api/org/departments', { name })
  return res.data
}

/** Renames a department; the default one can be renamed too. */
export async function renameOrgDepartment(
  id: number,
  name: string
): Promise<OrgApiResponse> {
  const res = await api.put(`/api/org/departments/${id}`, { name })
  return res.data
}

/** Deletes a department; its members and unused invite links move to the default one. */
export async function deleteOrgDepartment(id: number): Promise<OrgApiResponse> {
  const res = await api.delete(`/api/org/departments/${id}`)
  return res.data
}

/** A change to one member; omitted fields stay as they are. */
export type OrgMemberPatch = {
  role_id?: number
  department_id?: number
  /** Replaces the departments the member manages; their own is always one of them. */
  managed_department_ids?: number[]
}

/** Changes a member's role, department, the departments they manage, or any of them at once. */
export async function updateOrgMember(
  id: number,
  patch: OrgMemberPatch
): Promise<OrgApiResponse> {
  const res = await api.put(`/api/org/members/${id}`, patch)
  return res.data
}

/** Adds a service account — a member that holds keys and cannot sign in. */
export async function createOrgServiceAccount(payload: {
  name: string
  department_id: number
}): Promise<OrgApiResponse<OrgMember>> {
  const res = await api.post('/api/org/service-accounts', payload)
  return res.data
}

/** Issues an invite link for a role and a department. */
export async function createOrgInvite(payload: {
  role_id: number
  department_id: number
}): Promise<OrgApiResponse<OrgInvite>> {
  const res = await api.post('/api/org/invites', payload)
  return res.data
}

/** Makes an invite link stop working at once. */
export async function revokeOrgInvite(id: number): Promise<OrgApiResponse> {
  const res = await api.delete(`/api/org/invites/${id}`)
  return res.data
}

/**
 * Where an invite code leads. Public — the sign-up page calls it for a
 * visitor with no account. Resolves to null for a code that is unknown,
 * expired or revoked; the page says so itself, so the global toast stays out.
 */
export async function fetchOrgInvitePreview(
  code: string
): Promise<OrgInvitePreview | null> {
  const res = await api.get(`/api/org/invite/${encodeURIComponent(code)}`, {
    skipErrorHandler: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data?.success ? (res.data.data as OrgInvitePreview) : null
}
