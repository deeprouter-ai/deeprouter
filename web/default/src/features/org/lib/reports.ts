// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { OrgMembership } from '../types'
import { holds } from './permissions'

/** The sections of the "Reports & alerts" page, in the order of its tabs. */
export type OrgReportsSection = 'usage' | 'alerts' | 'audit'

/**
 * The read permission each section takes. They are three separate
 * primitives: a finance role sees usage and neither of the others, a manager
 * usage and alerts but not the audit log.
 */
const SECTION_PRIMITIVE: Record<OrgReportsSection, string> = {
  usage: 'usage.read',
  alerts: 'alert.read',
  audit: 'audit.read',
}

/** The primitives any one of which opens the page. */
export const REPORT_PRIMITIVES = Object.values(SECTION_PRIMITIVE)

/** The sections of the page this member's role lets them read, in tab order. */
export function reportSections(
  membership: OrgMembership | null | undefined
): OrgReportsSection[] {
  return (Object.keys(SECTION_PRIMITIVE) as OrgReportsSection[]).filter(
    (section) => holds(membership, SECTION_PRIMITIVE[section])
  )
}
