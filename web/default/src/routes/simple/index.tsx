// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { SimpleHome } from '@/features/simple/pages/home'

const searchSchema = z.object({
  topup: z.boolean().optional(),
})

export const Route = createFileRoute('/simple/')({
  validateSearch: searchSchema,
  component: SimpleHomeRoute,
})

function SimpleHomeRoute() {
  const { topup } = Route.useSearch()
  return <SimpleHome openTopup={topup} />
}
