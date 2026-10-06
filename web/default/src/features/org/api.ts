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
  OrgRole,
} from './types'

export const orgQueryKeys = {
  all: ['org'] as const,
  self: () => [...orgQueryKeys.all, 'self'] as const,
  departments: () => [...orgQueryKeys.all, 'departments'] as const,
  roles: () => [...orgQueryKeys.all, 'roles'] as const,
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

/** The organization's departments, the default one first, with head counts. */
export async function fetchOrgDepartments(): Promise<OrgDepartment[]> {
  const res = await api.get('/api/org/departments')
  return (res.data?.data as OrgDepartment[]) ?? []
}

/** The roles members of the organization can hold: the presets, then its own. */
export async function fetchOrgRoles(): Promise<OrgRole[]> {
  const res = await api.get('/api/org/roles')
  return (res.data?.data as OrgRole[]) ?? []
}

/** Everyone in the organization, service accounts included. */
export async function fetchOrgMembers(): Promise<OrgMember[]> {
  const res = await api.get('/api/org/members')
  return (res.data?.data as OrgMember[]) ?? []
}

/** The invite links that still admit new members, newest first. */
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

/** Change a member's role, department, or both; omitted fields stay as they are. */
export async function updateOrgMember(
  id: number,
  patch: { role_id?: number; department_id?: number }
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
