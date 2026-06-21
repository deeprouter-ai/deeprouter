/*
Copyright (C) 2023-2026 QuantumNous

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

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import type {
  MarketplaceErrorResponse,
  MarketplaceListResponse,
  MarketplaceSkill,
  MySkill,
  PublicSkillDetail,
} from './types'

export async function getMarketplaceSkills(): Promise<
  MarketplaceListResponse<MarketplaceSkill>
> {
  const res = await api.get('/api/v1/marketplace/skills', {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getMySkills(): Promise<MarketplaceListResponse<MySkill>> {
  const res = await api.get('/api/v1/marketplace/my-skills', {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getMarketplaceSkill(
  idOrSlug: string
): Promise<PublicSkillDetail> {
  const res = await api.get(
    '/api/v1/marketplace/skills/' + encodeURIComponent(idOrSlug),
    { skipErrorHandler: true } as Record<string, unknown>
  )
  return res.data?.data ?? res.data
}

/**
 * Stable error thrown by downloadSkillPackage so the detail page can map
 * `code` to copy. `AUTH_REQUIRED` here means web login/session failure for the
 * download action — NOT a missing runner key (that is the DR-62 runtime
 * client). Never present this as "add your key".
 */
export class DownloadSkillError extends Error {
  code: string
  constructor(code: string, message?: string) {
    super(message || code)
    this.name = 'DownloadSkillError'
    this.code = code
  }
}

// Reject empty / cross-origin download URLs before they reach axios. The URL
// must come from the backend `download_cta.url` (an API-relative path); we do
// not construct it on the frontend.
function isSafeDownloadUrl(url: string): boolean {
  if (!url || typeof url !== 'string') return false
  let resolved: URL
  try {
    resolved = new URL(url, window.location.origin)
  } catch {
    return false
  }
  // Same-origin only (rejects `//host`, absolute cross-origin, malformed).
  if (resolved.origin !== window.location.origin) return false
  // Must be the marketplace skill download endpoint — not an arbitrary
  // same-origin path. Uses contains+endsWith so a future `/api` prefix change
  // does not silently break it.
  return (
    resolved.pathname.includes('/marketplace/skills/') &&
    resolved.pathname.endsWith('/download')
  )
}

function sanitizeFilename(name: string | undefined, fallback: string): string {
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
function filenameFromContentDisposition(
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

// Error responses arrive as a Blob (because responseType is 'blob'); axios does
// not parse them and the global interceptor cannot read business codes. Parse
// the envelope `{ "error": { "code", "message" } }` here. Tolerate Blob /
// string / object / non-JSON / HTML bodies and fall back safely.
async function extractDownloadError(data: unknown): Promise<DownloadSkillError> {
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
    const obj = data as MarketplaceErrorResponse
    if (obj.error?.code) {
      return new DownloadSkillError(obj.error.code, obj.error.message)
    }
  }

  if (text) {
    try {
      const parsed = JSON.parse(text) as MarketplaceErrorResponse
      if (parsed?.error?.code) {
        return new DownloadSkillError(parsed.error.code, parsed.error.message)
      }
    } catch {
      /* non-JSON (e.g. gateway HTML) — fall through to generic */
    }
  }

  return new DownloadSkillError('DOWNLOAD_FAILED')
}

/**
 * Download a Skill package zip. `downloadCtaUrl` must be the backend-provided
 * `download_cta.url` (not constructed on the frontend). Goes through the axios
 * `api` client so the `New-Api-User` header is attached — a native `<a download>`
 * would omit it and be rejected by SkillUserAuth. On failure throws a
 * DownloadSkillError carrying the backend `error.code`.
 */
export async function downloadSkillPackage(
  downloadCtaUrl: string,
  fallbackSlug: string
): Promise<void> {
  if (!isSafeDownloadUrl(downloadCtaUrl)) {
    throw new DownloadSkillError('DOWNLOAD_UNAVAILABLE')
  }

  let res
  try {
    res = await api.get(downloadCtaUrl, {
      responseType: 'blob',
      skipErrorHandler: true,
    } as Record<string, unknown>)
  } catch (error) {
    const data = (error as { response?: { data?: unknown } })?.response?.data
    throw await extractDownloadError(data)
  }

  const filename = filenameFromContentDisposition(
    res.headers?.['content-disposition'],
    fallbackSlug
  )
  const objectUrl = URL.createObjectURL(res.data as Blob)
  try {
    const anchor = document.createElement('a')
    anchor.href = objectUrl
    anchor.download = filename
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
  } finally {
    URL.revokeObjectURL(objectUrl)
  }
}
