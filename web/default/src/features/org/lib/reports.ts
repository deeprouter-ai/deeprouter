// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { OrgMembership } from '../types'
import { holds } from './permissions'

/** The sections of the "Reports & alerts" page, in the order of its tabs. */
export type OrgReportsSection = 'usage' | 'alerts' | 'audit'

/**
 * The read permission each section takes. They are three separate
 * primitives: a finance role reads the usage report and not the audit log, a
 * manager usage and alerts but not the audit log.
 */
const SECTION_PRIMITIVE: Record<OrgReportsSection, string> = {
  usage: 'usage.read',
  alerts: 'alert.read',
  audit: 'audit.read',
}

/**
 * Whether this member's alerts section lists only the warnings on the keys
 * they hold themselves: so it is for whoever's role does not read alerts
 * (PRD D47). The backend cuts the list; this decides what the page says.
 */
export function seesOwnAlertsOnly(
  membership: OrgMembership | null | undefined
): boolean {
  return !holds(membership, SECTION_PRIMITIVE.alerts)
}

/**
 * The sections of the page this member may open, in tab order. Usage and the
 * audit log take their primitive. Alerts are every member's: a role that
 * reads alerts sees what it reaches, everyone else their own.
 */
export function reportSections(
  membership: OrgMembership | null | undefined
): OrgReportsSection[] {
  if (!membership) return []
  return (Object.keys(SECTION_PRIMITIVE) as OrgReportsSection[]).filter(
    (section) =>
      section === 'alerts' || holds(membership, SECTION_PRIMITIVE[section])
  )
}
