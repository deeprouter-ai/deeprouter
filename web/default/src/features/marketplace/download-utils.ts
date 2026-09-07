/*
Copyright (C) 2026 DeepRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
// Pure helpers for the Skill package download flow, kept out of api.ts so
// they can be unit-tested without pulling in the axios client. Adapted from
// the V1 marketplace helpers (removed in #159) for the V2 error envelope
// { success, message, data } — V1 used { error: { code, message } }.

export type DownloadErrorCode =
  | 'INSUFFICIENT_BALANCE'
  | 'NOT_FOUND'
  | 'DOWNLOAD_FAILED'

/**
 * Stable error thrown by downloadMarketplaceSkill so pages can map `code`
 * to copy. INSUFFICIENT_BALANCE (HTTP 402) additionally carries the price
 * details the backend attaches as `data`.
 */
export class DownloadSkillError extends Error {
  code: DownloadErrorCode
  priceUsd?: number
  quotaNeeded?: number

  constructor(
    code: DownloadErrorCode,
    message?: string,
    details?: { priceUsd?: number; quotaNeeded?: number }
  ) {
    super(message || code)
    this.name = 'DownloadSkillError'
    this.code = code
    this.priceUsd = details?.priceUsd
    this.quotaNeeded = details?.quotaNeeded
  }
}

export function sanitizeFilename(
  name: string | undefined,
  fallback: string
): string {
  // Sanitize the fallback too, so the helper is robust even if `fallback`
  // (the slug) ever contains path separators.
  const safeFallbackBase = fallback.replace(/[/\\]/g, '').trim() || 'skill'
  const fallbackName = safeFallbackBase.endsWith('.zip')
    ? safeFallbackBase
    : `${safeFallbackBase}.zip`
  if (!name) return fallbackName
  // Strip any path separators and surrounding quotes; reject empties.
  const cleaned = name
    .replace(/^["']|["']$/g, '')
    .replace(/[/\\]/g, '')
    .trim()
  return cleaned.length > 0 ? cleaned : fallbackName
}

// Prefer RFC 5987 `filename*=` (handles non-ASCII), fall back to `filename=`.
export function filenameFromContentDisposition(
  header: string | undefined,
  fallbackSlug: string
): string {
  let raw: string | undefined
  if (header) {
    const star = /filename\*=(?:UTF-8'')?([^;]+)/i.exec(header)
    if (star?.[1]) {
      try {
        raw = decodeURIComponent(star[1])
      } catch {
        raw = star[1]
      }
    }
    if (!raw) {
      const plain = /filename=([^;]+)/i.exec(header)
      if (plain?.[1]) raw = plain[1].trim()
    }
  }
  return sanitizeFilename(raw, fallbackSlug)
}

interface ErrorEnvelope {
  success?: boolean
  message?: string
  data?: { price_usd?: number; quota_needed?: number }
}

// Error responses arrive as a Blob (because responseType is 'blob'); axios
// does not parse them and the global interceptor cannot read them. Parse the
// V2 envelope here, tolerating Blob / string / object / non-JSON bodies.
export async function extractDownloadError(
  status: number | undefined,
  data: unknown
): Promise<DownloadSkillError> {
  let envelope: ErrorEnvelope | undefined
  let text: string | undefined

  if (data instanceof Blob) {
    try {
      text = await data.text()
    } catch {
      text = undefined
    }
  } else if (typeof data === 'string') {
    text = data
  } else if (data && typeof data === 'object') {
    envelope = data as ErrorEnvelope
  }

  if (!envelope && text) {
    try {
      envelope = JSON.parse(text) as ErrorEnvelope
    } catch {
      /* non-JSON (e.g. gateway HTML) — fall through */
    }
  }

  if (status === 402) {
    return new DownloadSkillError('INSUFFICIENT_BALANCE', envelope?.message, {
      priceUsd: envelope?.data?.price_usd,
      quotaNeeded: envelope?.data?.quota_needed,
    })
  }
  if (status === 404) {
    return new DownloadSkillError('NOT_FOUND', envelope?.message)
  }
  return new DownloadSkillError('DOWNLOAD_FAILED', envelope?.message)
}

// Hand the fetched ZIP to the browser as a normal file download.
export function saveBlob(blob: Blob, filename: string): void {
  const objectUrl = URL.createObjectURL(blob)
  try {
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
  } finally {
    URL.revokeObjectURL(objectUrl)
  }
}
