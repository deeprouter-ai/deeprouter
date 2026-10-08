// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { api } from '@/lib/api'
import type {
  OrgAlert,
  OrgAlertSettings,
  OrgApiResponse,
  OrgAuditLog,
  OrgDepartment,
  OrgInvite,
  OrgInvitePreview,
  OrgKey,
  OrgKeyGrant,
  OrgKeyHolder,
  OrgKeyInput,
  OrgKeyPatch,
  OrgKeyTemplate,
  OrgMember,
  OrgMembership,
  OrgOwnKey,
  OrgPage,
  OrgPermissionCatalog,
  OrgRole,
  OrgRoleInput,
  OrgUsageGroupBy,
  OrgUsageReport,
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
  keys: () => [...orgQueryKeys.all, 'keys'] as const,
  keyHolders: () => [...orgQueryKeys.all, 'key-holders'] as const,
  keyAssignees: () => [...orgQueryKeys.all, 'key-assignees'] as const,
  keyTemplates: () => [...orgQueryKeys.all, 'key-templates'] as const,
  keyModels: (holderId: number) =>
    [...orgQueryKeys.all, 'key-models', holderId] as const,
  usage: (params: OrgUsageParams) =>
    [...orgQueryKeys.all, 'usage', params] as const,
  alerts: () => [...orgQueryKeys.all, 'alerts'] as const,
  alertList: (params: OrgAlertsParams) =>
    [...orgQueryKeys.alerts(), params] as const,
  alertSettings: () => [...orgQueryKeys.all, 'alert-settings'] as const,
  auditLogs: (params: OrgAuditParams) =>
    [...orgQueryKeys.all, 'audit-logs', params] as const,
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

/**
 * Takes a member — a person or a service account — out of the organization
 * for good: their keys are taken back and their account is deleted. The answer
 * says how many keys that was.
 */
export async function removeOrgMember(
  id: number
): Promise<OrgApiResponse<{ reclaimed_keys: number }>> {
  const res = await api.delete(`/api/org/members/${id}`)
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

/** The organization's keys the caller may see, newest first, values masked. */
export async function fetchOrgKeys(): Promise<OrgKey[]> {
  const res = await api.get('/api/org/keys')
  return (res.data?.data as OrgKey[]) ?? []
}

/** The members the caller may create a key for. */
export async function fetchOrgKeyHolders(): Promise<OrgKeyHolder[]> {
  const res = await api.get('/api/org/key-holders')
  return (res.data?.data as OrgKeyHolder[]) ?? []
}

/** The members the caller may hand a key to; the owner is not one of them. */
export async function fetchOrgKeyAssignees(): Promise<OrgKeyHolder[]> {
  const res = await api.get('/api/org/key-assignees')
  return (res.data?.data as OrgKeyHolder[]) ?? []
}

/**
 * The caller's own keys that work right now and can serve a purpose of the
 * console ("video", "coding", …), newest first. A member of an organization
 * makes no key of their own, so this is what the console sets a tool up with.
 */
export async function fetchOrgSelfKeys(purpose: string): Promise<OrgOwnKey[]> {
  const res = await api.get('/api/org/self/keys', { params: { purpose } })
  return (res.data?.data as OrgOwnKey[]) ?? []
}

/** The policy templates a key can be given. */
export async function fetchOrgKeyTemplates(): Promise<OrgKeyTemplate[]> {
  const res = await api.get('/api/org/key-templates')
  return (res.data?.data as OrgKeyTemplate[]) ?? []
}

/**
 * The models a key held by the given member can be limited to by hand: what
 * that member can be served right now, in name order.
 */
export async function fetchOrgKeyModels(holderId: number): Promise<string[]> {
  const res = await api.get('/api/org/key-models', {
    params: { holder_id: holderId },
  })
  return (res.data?.data as string[]) ?? []
}

/**
 * Creates a key. The answer carries its value only when the holder is a
 * service account.
 */
export async function createOrgKey(
  input: OrgKeyInput
): Promise<OrgApiResponse<OrgKeyGrant>> {
  const res = await api.post('/api/org/keys', input)
  return res.data
}

/** Changes what a key is allowed; fields left out of the patch stay as they are. */
export async function updateOrgKey(
  id: number,
  patch: OrgKeyPatch
): Promise<OrgApiResponse<OrgKey>> {
  const res = await api.put(`/api/org/keys/${id}`, patch)
  return res.data
}

/**
 * Replaces a key's value; the old one stops working at once. The answer
 * carries the new value only for a service account's key.
 */
export async function rotateOrgKey(
  id: number
): Promise<OrgApiResponse<OrgKeyGrant>> {
  const res = await api.post(`/api/org/keys/${id}/rotate`)
  return res.data
}

/** Stops a key from working until it is unfrozen. */
export async function freezeOrgKey(id: number): Promise<OrgApiResponse> {
  const res = await api.post(`/api/org/keys/${id}/freeze`)
  return res.data
}

/** Makes a frozen key work again. */
export async function unfreezeOrgKey(id: number): Promise<OrgApiResponse> {
  const res = await api.post(`/api/org/keys/${id}/unfreeze`)
  return res.data
}

/**
 * Hands a key to another member. It gets a new value on the way, so the copy
 * its last holder has stops working; the answer carries the new value only
 * when the new holder is a service account.
 */
export async function assignOrgKey(
  id: number,
  holderId: number
): Promise<OrgApiResponse<OrgKeyGrant>> {
  const res = await api.post(`/api/org/keys/${id}/assign`, {
    holder_id: holderId,
  })
  return res.data
}

/**
 * Takes a key back from its holder: frozen, with a new value, parked under the
 * owner until it is handed to someone else.
 */
export async function reclaimOrgKey(
  id: number
): Promise<OrgApiResponse<OrgKey>> {
  const res = await api.post(`/api/org/keys/${id}/reclaim`)
  return res.data
}

/** Deletes a key for good; its usage history stays. */
export async function deleteOrgKey(id: number): Promise<OrgApiResponse> {
  const res = await api.delete(`/api/org/keys/${id}`)
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

/**
 * What a usage report is asked for. A bound that is left out leaves that side
 * of the period open; without a department the report covers every department
 * the caller may see.
 */
export type OrgUsageParams = {
  group_by: OrgUsageGroupBy
  /** Unix seconds, included. */
  start_timestamp?: number
  /** Unix seconds, included. */
  end_timestamp?: number
  department_id?: number
}

/**
 * The organization's usage over a period, summed by department, member, key
 * or model — as much of it as the caller's role lets them see. Naming a
 * department out of their reach is a 403.
 */
export async function fetchOrgUsage(
  params: OrgUsageParams
): Promise<OrgUsageReport> {
  const res = await api.get('/api/org/usage', { params })
  return res.data?.data as OrgUsageReport
}

/** Which page of the alert list to fetch; `state: 'open'` leaves out the ones dealt with. */
export type OrgAlertsParams = {
  p: number
  page_size: number
  state?: 'open'
}

/** One page of the alerts the caller may see, newest first. */
export async function fetchOrgAlerts(
  params: OrgAlertsParams
): Promise<OrgPage<OrgAlert>> {
  const res = await api.get('/api/org/alerts', { params })
  return res.data?.data as OrgPage<OrgAlert>
}

/** Marks an alert as handled or as a false alarm; the owner's and the admins' to do. */
export async function handleOrgAlert(
  id: number,
  state: 'handled' | 'false_alarm'
): Promise<OrgApiResponse> {
  const res = await api.post(`/api/org/alerts/${id}/handle`, { state })
  return res.data
}

/** What the warnings and alerts run on; only the owner and admins may read it. */
export async function fetchOrgAlertSettings(): Promise<OrgAlertSettings> {
  const res = await api.get('/api/org/alert-settings')
  return res.data?.data as OrgAlertSettings
}

/** Replaces the warning levels, the alert rules and the working hours. */
export async function updateOrgAlertSettings(
  settings: OrgAlertSettings
): Promise<OrgApiResponse<OrgAlertSettings>> {
  const res = await api.put('/api/org/alert-settings', settings)
  return res.data
}

/** Which page of the audit log to fetch, and what to narrow it down to. */
export type OrgAuditParams = {
  p: number
  page_size: number
  /** Part of the name of whoever did it. */
  actor?: string
  target_type?: string
  /** Unix seconds, included. */
  start_timestamp?: number
  /** Unix seconds, included. */
  end_timestamp?: number
}

/** One page of the organization's audit log, newest first. */
export async function fetchOrgAuditLogs(
  params: OrgAuditParams
): Promise<OrgPage<OrgAuditLog>> {
  const res = await api.get('/api/org/audit-logs', { params })
  return res.data?.data as OrgPage<OrgAuditLog>
}
