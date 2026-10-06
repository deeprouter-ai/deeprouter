// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { requireOrgAccess } from '@/features/org/hooks/use-org-membership'
import { OrgRolesPage } from '@/features/org/roles'

export const Route = createFileRoute('/_authenticated/org/roles')({
  beforeLoad: ({ context }) => requireOrgAccess(context.queryClient),
  component: OrgRolesPage,
})
