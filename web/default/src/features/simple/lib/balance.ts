// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { getCurrencyDisplay } from '@/lib/currency'
import { DEFAULT_VIDEO_MODEL } from '@/features/video/lib/prompt-template'

/** The 6-second, 768P MiniMax-H3 clip the video prompt's test run uses. */
export const REFERENCE_CLIP_USD = 0.48

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

export const REFERENCE_CLIP_MODEL = DEFAULT_VIDEO_MODEL.name
