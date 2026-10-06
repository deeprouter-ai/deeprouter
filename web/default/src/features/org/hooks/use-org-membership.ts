// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { fetchOrgMembership, orgQueryKeys } from '../api'
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
 * Whether this member runs the organization. Until the permission engine
 * (PRD P4) lands, that is the owner and the admins — the backend enforces the
 * same line, this only decides what to show.
 */
export function canManageOrg(
  membership: OrgMembership | null | undefined
): boolean {
  return Boolean(membership && (membership.is_owner || membership.is_admin))
}
