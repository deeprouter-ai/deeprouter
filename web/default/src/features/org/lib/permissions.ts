// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import type { OrgMembership, OrgRolePack, OrgRoleScope } from '../types'

/**
 * Whether the member's role grants a primitive, wherever it reaches. The
 * backend reports permissions with the reads that writes bring already
 * listed, so this is a lookup and nothing more — the rule lives in one place.
 * It decides what to show; every /api/org call is checked again on the server.
 */
export function holds(
  membership: OrgMembership | null | undefined,
  primitive: string
): boolean {
  return Boolean(membership?.permissions.includes(primitive))
}

/** The resource a primitive is about: `key` for `key.assign`. */
export function resourceOf(primitive: string): string {
  return primitive.split('.')[0]
}

/** Whether a primitive only looks: `key.read` does, `key.assign` does not. */
export function isRead(primitive: string): boolean {
  return primitive.endsWith('.read')
}

/**
 * The reads a selection of primitives brings with it: a write includes the
 * read of the same resource (PRD §2). The role form shows these as ticked and
 * locked; the backend adds them on save whatever the form sends.
 */
export function impliedReads(selected: string[]): Set<string> {
  return new Set(
    selected
      .filter((primitive) => !isRead(primitive))
      .map((primitive) => `${resourceOf(primitive)}.read`)
  )
}

/** The primitives in the order given, gathered under the resource they are about. */
export function groupByResource(
  primitives: string[]
): { resource: string; primitives: string[] }[] {
  const groups: { resource: string; primitives: string[] }[] = []
  for (const primitive of primitives) {
    const resource = resourceOf(primitive)
    const group = groups.find((g) => g.resource === resource)
    if (group) {
      group.primitives.push(primitive)
    } else {
      groups.push({ resource, primitives: [primitive] })
    }
  }
  return groups
}

/** Heading of a group of primitives. */
export function resourceLabel(t: TFunction, resource: string): string {
  switch (resource) {
    case 'key':
      return t('Keys')
    case 'member':
      return t('Members')
    case 'usage':
      return t('Usage')
    case 'alert':
      return t('Alerts')
    case 'audit':
      return t('Audit log')
    default:
      return resource
  }
}

/**
 * Name of a primitive or an inherent power in words. Anything the page has no
 * words for yet — a primitive added on the backend first — shows as its code.
 */
export function permissionLabel(t: TFunction, name: string): string {
  switch (name) {
    case 'key.read':
      return t('View keys')
    case 'key.create':
      return t('Create keys')
    case 'key.update':
      return t('Edit keys')
    case 'key.assign':
      return t('Assign keys')
    case 'key.rotate':
      return t('Rotate keys')
    case 'key.freeze':
      return t('Freeze keys')
    case 'key.delete':
      return t('Delete keys')
    case 'member.read':
      return t('View members')
    case 'member.invite':
      return t('Invite members')
    case 'member.remove':
      return t('Remove members')
    case 'usage.read':
      return t('View usage')
    case 'alert.read':
      return t('View alerts')
    case 'audit.read':
      return t('View the audit log')
    case 'role.manage':
      return t('Manage roles')
    case 'department.manage':
      return t('Manage departments')
    case 'service_account.manage':
      return t('Manage service accounts')
    case 'org.settings':
      return t('Organization settings')
    case 'alert.handle':
      return t('Handle alerts')
    case 'admin.appoint':
      return t('Appoint admins')
    case 'wallet.manage':
      return t('Company wallet')
    case 'org.transfer':
      return t('Transfer or close the organization')
    default:
      return name
  }
}

/** One line on what a primitive or an inherent power lets its holder do. */
export function permissionHint(t: TFunction, name: string): string {
  switch (name) {
    case 'key.read':
      return t('See keys, their settings and status. Key values stay masked.')
    case 'key.create':
      return t('Create keys, with a template, a quota and rate limits.')
    case 'key.update':
      return t("Change a key's quota, rate limits, template and expiry.")
    case 'key.assign':
      return t('Give a key to a member or a service account, or take it back.')
    case 'key.rotate':
      return t("Replace a key's value. The old one stops working at once.")
    case 'key.freeze':
      return t('Freeze a key and unfreeze it.')
    case 'key.delete':
      return t('Delete a key. Its past usage stays on record.')
    case 'member.read':
      return t('See members, departments and roles.')
    case 'member.invite':
      return t(
        'Invite people. Anyone but the owner and admins invites Staff only.'
      )
    case 'member.remove':
      return t('Take a member out of the organization.')
    case 'usage.read':
      return t('See usage reports.')
    case 'alert.read':
      return t('See the list of alerts.')
    case 'audit.read':
      return t('See who did what in the organization, and when.')
    case 'role.manage':
      return t('Create and change roles, and give members a role.')
    case 'department.manage':
      return t('Create, rename and delete departments, and move members.')
    case 'service_account.manage':
      return t('Add service accounts for pipelines and bots.')
    case 'org.settings':
      return t('Set alert thresholds and working hours.')
    case 'alert.handle':
      return t('Mark an alert as handled or as a false alarm.')
    case 'admin.appoint':
      return t('Make a member an admin, or take the role away.')
    case 'wallet.manage':
      return t('Top up and manage the balance every key draws on.')
    case 'org.transfer':
      return t('Hand the organization to someone else, or close it.')
    default:
      return ''
  }
}

/** How far a role reaches, in words. */
export function scopeLabel(t: TFunction, scope: OrgRoleScope | string): string {
  switch (scope) {
    case 'org':
      return t('Whole organization')
    case 'dept':
      return t('Departments they manage')
    case 'self':
      return t('Only themselves')
    default:
      return scope
  }
}

/**
 * The name a role pack is shown by — and adopted under, so the role comes out
 * in the language the admin is reading. A pack the page has no name for shows
 * the backend's.
 */
export function rolePackLabel(t: TFunction, pack: OrgRolePack): string {
  switch (pack.key) {
    case 'it_ops':
      return t('IT Ops')
    case 'hr_ops':
      return t('HR Ops')
    case 'finance':
      return t('Finance Ops')
    default:
      return pack.name
  }
}

/** What a role pack is for, in one line. */
export function rolePackHint(t: TFunction, pack: OrgRolePack): string {
  switch (pack.key) {
    case 'it_ops':
      return t(
        'Key administrator: creates, assigns, rotates and freezes keys across the organization.'
      )
    case 'hr_ops':
      return t(
        "People operations: invites and removes members, and cleans up a leaver's keys."
      )
    case 'finance':
      return t(
        'Reconciliation: sees what was spent, and neither keys nor members.'
      )
    default:
      return ''
  }
}
