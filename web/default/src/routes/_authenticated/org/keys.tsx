// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { requireOrgAccess } from '@/features/org/hooks/use-org-membership'
import { OrgKeysPage } from '@/features/org/keys'

export const Route = createFileRoute('/_authenticated/org/keys')({
  beforeLoad: ({ context }) =>
    requireOrgAccess(context.queryClient, 'key.read'),
  component: OrgKeysPage,
})
