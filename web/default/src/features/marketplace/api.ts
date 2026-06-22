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
import {
  DownloadSkillError,
  extractDownloadError,
  filenameFromContentDisposition,
  isSafeDownloadUrl,
} from './download-utils'
import type {
  MarketplaceListResponse,
  MarketplaceSkill,
  MySkill,
  PublicSkillDetail,
} from './types'

// Re-export so existing importers (e.g. skill-detail.tsx) keep importing the
// error type from './api'. The implementation now lives in ./download-utils.
export { DownloadSkillError } from './download-utils'

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
