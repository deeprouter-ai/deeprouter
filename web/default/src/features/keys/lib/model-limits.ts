/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import type { ApiKey } from '../types'

/**
 * Whether a key is allowed to call the given model, mirroring the gateway's
 * enforcement exactly (model/token.go MatchModelLimit): entries match the
 * model name verbatim, and an entry ending in "*" matches as a prefix.
 *
 * Frontend surfaces use this to refuse pairings the gateway would 403 anyway
 * — a chat-limited Simple key bound to the video page configures a project
 * whose very first generation fails, with nothing on the page to explain it.
 */
export function keyPermitsModel(
  key: Pick<ApiKey, 'model_limits_enabled' | 'model_limits'>,
  modelName: string
): boolean {
  if (!key.model_limits_enabled) return true
  const entries = (key.model_limits ?? '')
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean)
  return entries.some(
    (entry) =>
      entry === modelName ||
      (entry.endsWith('*') && modelName.startsWith(entry.slice(0, -1)))
  )
}
