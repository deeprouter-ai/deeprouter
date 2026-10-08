// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { requireOrgMember } from '@/features/org/hooks/use-org-membership'
import { OrgReportsPage } from '@/features/org/reports'

// The section is in the address, so a notification can link straight to the
// alerts. One the page does not know opens the page on its first section.
const searchSchema = z.object({
  section: z.enum(['usage', 'alerts', 'audit']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/org/reports')({
  validateSearch: searchSchema,
  // Every member has something here: at least the alerts on their own keys.
  beforeLoad: ({ context }) => requireOrgMember(context.queryClient),
  component: OrgReportsRoute,
})

/** The page, with the section it shows kept in the address. */
function OrgReportsRoute() {
  const { section } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <OrgReportsPage
      section={section}
      onSectionChange={(next) =>
        void navigate({ search: { section: next }, replace: true })
      }
    />
  )
}
