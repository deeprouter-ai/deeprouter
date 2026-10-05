// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createFileRoute } from '@tanstack/react-router'
import { SimpleRecords } from '@/features/simple/pages/records'

export const Route = createFileRoute('/simple/records')({
  component: SimpleRecords,
})
