// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { SimpleMe } from '@/features/simple/pages/me'

export const Route = createFileRoute('/simple/me')({
  component: SimpleMe,
})
