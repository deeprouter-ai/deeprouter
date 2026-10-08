// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { type QueryClient, useQuery } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { fetchOrgMembership, orgQueryKeys } from '../api'
import { holds } from '../lib/permissions'
import type { OrgMembership } from '../types'

/**
 * Query options for the signed-in user's organization membership. The user id
 * is part of the key: the query cache outlives a sign-out, and one account's
 * organization must never be shown to the next.
 */
export function orgMembershipQuery(userId: number | undefined) {
  return {
    queryKey: [...orgQueryKeys.self(), userId ?? 0] as const,
    queryFn: fetchOrgMembership,
  }
}

/** The signed-in user's organization and role; `null` for a personal account. */
export function useOrgMembership() {
  const userId = useAuthStore((s) => s.auth.user?.id)
  return useQuery({
    ...orgMembershipQuery(userId),
    enabled: Boolean(userId),
    staleTime: 5 * 60 * 1000,
  })
}

/**
 * Whether this member runs the organization: the owner and the admins, who
 * alone hold the inherent powers — roles, departments, service accounts
 * (PRD §2). No role grants these, so there is no permission to look up. The
 * backend enforces the same line; this only decides what to show.
 */
export function canManageOrg(
  membership: OrgMembership | null | undefined
): boolean {
  return Boolean(membership && (membership.is_owner || membership.is_admin))
}

/**
 * Whether this member may open the pages about people — members, departments
 * and roles — which takes `member.read`. A manager and a read-only member
 * may; Staff may not.
 */
export function canSeeOrg(
  membership: OrgMembership | null | undefined
): boolean {
  return holds(membership, 'member.read')
}

/**
 * Asks the backend who this user is in their organization — organization
 * roles are not the platform role — and sends them to the 403 page unless
 * `allowed` says the page is theirs to open. It is a courtesy: every /api/org
 * call checks again. It keeps people off pages which could only show them
 * errors.
 */
async function gateOrgPage(
  queryClient: QueryClient,
  allowed: (membership: OrgMembership | null) => boolean
) {
  const userId = useAuthStore.getState().auth.user?.id
  let membership: OrgMembership | null
  try {
    membership = await queryClient.fetchQuery({
      ...orgMembershipQuery(userId),
      staleTime: 30 * 1000,
    })
  } catch {
    // The question went unanswered (offline, rate-limited), which says
    // nothing about who is asking. Let the page load: it reports the
    // failure itself and offers a retry, where a 403 would be a wrong answer.
    return
  }
  if (!allowed(membership)) {
    throw redirect({ to: '/403' })
  }
}

/**
 * The `beforeLoad` gate of an organization page that takes a primitive:
 * `member.read` for the pages about people, `key.read` for the keys page.
 * Anyone whose role does not grant it is sent to the 403 page.
 */
export async function requireOrgAccess(
  queryClient: QueryClient,
  primitive: string = 'member.read'
) {
  return gateOrgPage(queryClient, (membership) => holds(membership, primitive))
}

/**
 * The `beforeLoad` gate of a page every member of an organization may open:
 * "Reports & alerts", where each member finds at least the alerts on their
 * own keys. Only a personal account is sent to the 403 page.
 */
export async function requireOrgMember(queryClient: QueryClient) {
  return gateOrgPage(queryClient, (membership) => membership !== null)
}
