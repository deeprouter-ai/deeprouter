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
import { api } from '@/lib/api'
import {
  extractDownloadError,
  filenameFromContentDisposition,
  saveBlob,
} from './download-utils'
import type {
  MarketplaceListData,
  MarketplaceListParams,
  MarketplaceSkillDetail,
  UserPurchasesData,
  UserSkillsData,
} from './types'

export const marketplaceQueryKeys = {
  all: ['marketplace'] as const,
  list: (params: MarketplaceListParams) =>
    [...marketplaceQueryKeys.all, 'list', params] as const,
  detail: (slug: string) =>
    [...marketplaceQueryKeys.all, 'detail', slug] as const,
  mySkills: () => [...marketplaceQueryKeys.all, 'my-skills'] as const,
  myPurchases: () => [...marketplaceQueryKeys.all, 'my-purchases'] as const,
}

export async function fetchMarketplaceSkills(
  params: MarketplaceListParams
): Promise<MarketplaceListData> {
  const res = await api.get('/api/skills', { params })
  return res.data.data as MarketplaceListData
}

// skipErrorHandler: an unknown slug is an expected state the page renders
// itself (not-found panel), not something the global toast should announce.
export async function fetchMarketplaceSkill(
  slug: string
): Promise<MarketplaceSkillDetail> {
  const res = await api.get(`/api/skills/${encodeURIComponent(slug)}`, {
    skipErrorHandler: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data.data as MarketplaceSkillDetail
}

/**
 * Runs the download endpoint and hands the ZIP to the browser. Throws
 * DownloadSkillError; the 402 case carries price details. The error body
 * arrives as a Blob (responseType 'blob'), so the global interceptor cannot
 * parse it — extractDownloadError does.
 */
export async function downloadMarketplaceSkill(slug: string): Promise<void> {
  let res
  try {
    res = await api.post(
      `/api/skills/${encodeURIComponent(slug)}/download`,
      undefined,
      { responseType: 'blob', skipErrorHandler: true } as Record<
        string,
        unknown
      >
    )
  } catch (error) {
    const resp = (
      error as { response?: { status?: number; data?: unknown } }
    ).response
    throw await extractDownloadError(resp?.status, resp?.data)
  }
  const filename = filenameFromContentDisposition(
    (res.headers as Record<string, string | undefined>)?.[
      'content-disposition'
    ],
    slug
  )
  saveBlob(res.data as Blob, filename)
}

export async function fetchMySkills(params?: {
  page?: number
  limit?: number
}): Promise<UserSkillsData> {
  const res = await api.get('/api/user/skills', { params })
  return res.data.data as UserSkillsData
}

export async function fetchMyPurchases(params?: {
  page?: number
  limit?: number
}): Promise<UserPurchasesData> {
  const res = await api.get('/api/user/purchases', { params })
  return res.data.data as UserPurchasesData
}
