// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import type { OrgMember, OrgMembership, OrgRole } from '../types'

/**
 * Display name of an organization role. The five presets are stored under
 * fixed English names and shown translated; a custom role shows the name its
 * company gave it.
 */
export function orgRoleLabel(t: TFunction, role: string): string {
  switch (role) {
    case 'owner':
      return t('Owner')
    case 'admin':
      return t('Admin')
    case 'manager':
      return t('Manager')
    case 'staff':
      return t('Staff')
    case 'readonly':
      return t('Read-only')
    default:
      return role
  }
}

/** Whether a role is the platform preset of that name, not a custom role that shares it. */
function isPresetRole(role: OrgRole, name: string): boolean {
  return role.is_preset && role.name === name
}

/**
 * The roles the acting member may hand out, by appointment or by invite: never
 * the owner role, and the admin role only when acting as the owner (PRD D10).
 * The backend enforces both; this keeps impossible choices off the screen.
 */
export function assignableRoles(
  roles: OrgRole[],
  actor: OrgMembership | null | undefined
): OrgRole[] {
  return roles.filter((role) => {
    if (isPresetRole(role, 'owner')) return false
    if (isPresetRole(role, 'admin')) return Boolean(actor?.is_owner)
    return true
  })
}

/**
 * Why the acting member cannot change a member's role, in words for the
 * screen — or null when they can.
 */
export function roleLockReason(
  t: TFunction,
  member: OrgMember,
  roles: OrgRole[],
  actor: OrgMembership | null | undefined
): string | null {
  if (member.is_owner) {
    return t("The owner's role cannot be changed.")
  }
  if (member.is_service) {
    return t('Service accounts always have the Staff role.')
  }
  const current = roles.find((role) => role.id === member.role_id)
  if (current && isPresetRole(current, 'admin') && !actor?.is_owner) {
    return t('Only the owner can appoint or dismiss admins.')
  }
  return null
}

/**
 * Whether holders of a role sit in the default department. The owner and the
 * admins run the whole organization, so they belong to no business unit
 * (PRD D26). The backend enforces it; this keeps the choice off the screen.
 */
export function sitsInDefaultDepartment(role: OrgRole | undefined): boolean {
  return (
    role !== undefined &&
    (isPresetRole(role, 'owner') || isPresetRole(role, 'admin'))
  )
}

/** The link a newcomer opens to sign up into the organization. */
export function orgInviteLink(code: string): string {
  const origin = typeof window === 'undefined' ? '' : window.location.origin
  return `${origin}/sign-up?org_invite=${encodeURIComponent(code)}`
}
