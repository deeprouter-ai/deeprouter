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
 * Whether this member has a place in the organization area at all: its pages
 * list members, departments and roles, which takes `member.read`. A manager
 * and a read-only member have it; Staff do not.
 */
export function canSeeOrg(
  membership: OrgMembership | null | undefined
): boolean {
  return holds(membership, 'member.read')
}

/**
 * The `beforeLoad` gate of the organization pages. Organization roles are not
 * the platform role, so it asks the backend who this user is in their
 * organization and sends anyone whose role does not grant the page's primitive
 * to the 403 page: `member.read` for the pages about people, `key.read` for
 * the keys page. A page made of sections that each take their own primitive
 * passes all of them, and whoever holds one gets in. It is a courtesy — every
 * /api/org call checks again — that keeps people off pages which could only
 * show them errors.
 */
export async function requireOrgAccess(
  queryClient: QueryClient,
  primitive: string | string[] = 'member.read'
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
  if (![primitive].flat().some((one) => holds(membership, one))) {
    throw redirect({ to: '/403' })
  }
}
