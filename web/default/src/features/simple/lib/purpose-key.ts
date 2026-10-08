// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { createApiKey, getApiKeys } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import {
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
} from '@/features/keys/lib/api-key-form'
import type { ApiKey } from '@/features/keys/types'
import { fetchOrgSelfKeys } from '@/features/org/api'
import type { SimplePurposeId } from './purposes'

/**
 * What a purpose page needs of the key it sets a tool up with: which key, and
 * which models it is limited to.
 */
export type PurposeKey = Pick<
  ApiKey,
  'id' | 'model_limits_enabled' | 'model_limits'
>

/**
 * A member of an organization has been handed no key that can serve the
 * purpose. Nothing went wrong: there is somebody to ask, and the page says so.
 */
export class NoAssignedKeyError extends Error {
  constructor() {
    super('no key assigned for this purpose')
    this.name = 'NoAssignedKeyError'
  }
}

/** The newest enabled key created for this purpose, if any (list is newest-first). */
export function pickPurposeKey(
  keys: ApiKey[],
  purpose: SimplePurposeId
): ApiKey | null {
  return (
    keys.find(
      (k) =>
        k.status === API_KEY_STATUS.ENABLED &&
        (k.simple_purpose ?? '') === purpose
    ) ?? null
  )
}

/**
 * Find this user's key for a purpose, creating one the first time. A Simple
 * user never sees a key: tapping a purpose card is the whole "get a key" step.
 * Creation goes through the same Simple payload as the key form, so the
 * backend grants exactly what that purpose allows (media purposes snapshot the
 * account's current catalog and fail closed when it is empty).
 *
 * A member of an organization makes no key of their own (Enterprise Org PRD
 * D16). For them the key is the newest one they were handed that can serve the
 * purpose — the backend says which, and which of the purpose's models it may
 * call — and when there is none, a `NoAssignedKeyError`.
 *
 * Throws with the server message when no key can be produced.
 */
export async function ensurePurposeKey(
  purpose: SimplePurposeId,
  defaultUseAutoGroup: boolean,
  orgMember = false
): Promise<PurposeKey> {
  if (orgMember) {
    const [assigned] = await fetchOrgSelfKeys(purpose)
    if (!assigned) throw new NoAssignedKeyError()
    // The page reads a key's whitelist to learn which models to offer. What
    // the backend names here is that list already cut down to this purpose.
    return {
      id: assigned.id,
      model_limits_enabled: assigned.models.length > 0,
      model_limits: assigned.models.join(','),
    }
  }

  const list = async () => {
    const res = await getApiKeys({ p: 1, size: 100 })
    return res.data?.items ?? []
  }

  const existing = pickPurposeKey(await list(), purpose)
  if (existing) return existing

  const payload = transformFormDataToPayload({
    ...getApiKeyFormDefaultValues(defaultUseAutoGroup, 'simple'),
    simple_purpose: purpose,
  })
  const created = await createApiKey(payload)
  if (!created.success) {
    throw new Error(created.message || 'could not create a key')
  }
  if (created.data?.id) return created.data

  // Older backends answer create without the row; read it back.
  const fresh = pickPurposeKey(await list(), purpose)
  if (fresh) return fresh
  throw new Error('could not create a key')
}
