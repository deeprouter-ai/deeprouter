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
  MarketplaceListResponse,
  MarketplaceSkill,
  MySkill,
  SkillGrowthEntryPoint,
  SkillGrowthEventType,
} from './types'

export { skillDownloadURL } from './lib/growth-surfaces'

export async function getMarketplaceSkill(
  slugOrId: string
): Promise<MarketplaceSkill> {
  const res = await api.get(
    `/api/v1/marketplace/skills/${encodeURIComponent(slugOrId)}`,
    { skipErrorHandler: true } as Record<string, unknown>
  )
  return (res.data?.data ?? res.data) as MarketplaceSkill
}

export async function downloadSkillPackage(
  slugOrId: string,
  entryPoint?: SkillGrowthEntryPoint
): Promise<void> {
  const params: Record<string, string> = {}
  if (entryPoint === 'new' || entryPoint === 'recommended') {
    params.entry_point = entryPoint
  }
  const res = await api.get(
    `/api/v1/marketplace/skills/${encodeURIComponent(slugOrId)}/download`,
    {
      params,
      responseType: 'blob',
      disableDuplicate: true,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  const disposition = (res.headers as Record<string, string>)[
    'content-disposition'
  ]
  const fileMatch = disposition?.match(/filename="([^"]+)"/)
  const filename = fileMatch?.[1] ?? `${slugOrId}.zip`
  const blob = new Blob([res.data as BlobPart], { type: 'application/zip' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export interface MarketplaceSkillsParams {
  featured?: boolean
  limit?: number
  page?: number
  sort?: string
}

export async function getMarketplaceSkills(): Promise<
  MarketplaceListResponse<MarketplaceSkill>
> {
  const res = await api.get('/api/v1/marketplace/skills', {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getMarketplaceSkillsWithParams(
  params: MarketplaceSkillsParams
): Promise<MarketplaceListResponse<MarketplaceSkill>> {
  const res = await api.get('/api/v1/marketplace/skills', {
    params,
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

export async function recordMarketplaceSkillEvent(
  skillId: string,
  event: {
    event_type: SkillGrowthEventType
    entry_point: SkillGrowthEntryPoint
  }
): Promise<void> {
  await api.post(
    `/api/v1/marketplace/skills/${encodeURIComponent(skillId)}/events`,
    event,
    {
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
}
