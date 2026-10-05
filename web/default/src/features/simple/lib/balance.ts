// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { getCurrencyDisplay } from '@/lib/currency'

/**
 * One Seedance 2.0 clip (5 s, 1080p, ≈ $1.0) — a model production actually
 * serves. The cheaper MiniMax-H3 figure overstated what a balance buys while
 * that channel was not live.
 */
export const REFERENCE_CLIP_USD = 1.0

/**
 * Turn a balance into something a non-technical user can picture: how many
 * short video clips it buys. `null` when it does not cover even one.
 */
export function clipsAffordable(
  quota: number | undefined,
  quotaPerUnit = getCurrencyDisplay().config.quotaPerUnit
): number | null {
  if (!quota || quota <= 0 || quotaPerUnit <= 0) return null
  const clips = Math.floor(quota / quotaPerUnit / REFERENCE_CLIP_USD)
  return clips >= 1 ? clips : null
}
