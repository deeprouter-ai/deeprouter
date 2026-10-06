// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { OrgMembersPage } from '@/features/org'
import {
  canManageOrg,
  orgMembershipQuery,
} from '@/features/org/hooks/use-org-membership'
import type { OrgMembership } from '@/features/org/types'

export const Route = createFileRoute('/_authenticated/org/members')({
  // Organization roles are not the platform role: the gate asks the backend
  // who this user is in their organization. It is a courtesy — every
  // /api/org call checks again — but it keeps people who do not manage the
  // organization from landing on a page that can only show errors.
  beforeLoad: async ({ context }) => {
    const userId = useAuthStore.getState().auth.user?.id
    let membership: OrgMembership | null
    try {
      membership = await context.queryClient.fetchQuery({
        ...orgMembershipQuery(userId),
        staleTime: 30 * 1000,
      })
    } catch {
      // The question went unanswered (offline, rate-limited), which says
      // nothing about who is asking. Let the page load: it reports the
      // failure itself and offers a retry, where a 403 would be a wrong answer.
      return
    }
    if (!canManageOrg(membership)) {
      throw redirect({ to: '/403' })
    }
  },
  component: OrgMembersPage,
})
