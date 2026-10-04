// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { ensureAuthenticated } from '@/lib/auth-guard'
import { SimpleShell } from '@/features/simple/components/simple-shell'

export const Route = createFileRoute('/simple')({
  beforeLoad: ({ location }) => ensureAuthenticated(location.href),
  component: SimpleShell,
})
