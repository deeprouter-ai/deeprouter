// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute, redirect } from '@tanstack/react-router'
import { findPurpose } from '@/features/simple/lib/purposes'
import { SimpleUsePurpose } from '@/features/simple/pages/use-purpose'

export const Route = createFileRoute('/simple/use/$purpose')({
  beforeLoad: ({ params }) => {
    if (!findPurpose(params.purpose)) throw redirect({ to: '/simple' })
  },
  component: UsePurposeRoute,
})

function UsePurposeRoute() {
  const { purpose } = Route.useParams()
  const found = findPurpose(purpose)
  // beforeLoad already redirected unknown ids; key by id so switching
  // purposes remounts and mints a fresh token.
  return found ? <SimpleUsePurpose key={found.id} purpose={found} /> : null
}
