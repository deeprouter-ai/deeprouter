// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { OrgMembersPage } from '@/features/org'
import { requireOrgAccess } from '@/features/org/hooks/use-org-membership'

export const Route = createFileRoute('/_authenticated/org/members')({
  beforeLoad: ({ context }) => requireOrgAccess(context.queryClient),
  component: OrgMembersPage,
})
