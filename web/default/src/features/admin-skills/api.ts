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
  AdminSkill,
  AdminSkillAuditLogResponse,
  AdminSkillListParams,
  AdminSkillListResponse,
  AdminSkillVersionsResponse,
  CreateSkillPayload,
  CreateVersionPayload,
  SkillVersionMetadata,
  UpdateSkillPayload,
} from './types'

export async function getAdminSkills(
  params: AdminSkillListParams
): Promise<AdminSkillListResponse> {
  const res = await api.get('/api/v1/admin/skills', {
    params,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function createAdminSkill(
  payload: CreateSkillPayload
): Promise<AdminSkill> {
  const res = await api.post('/api/v1/admin/skills', payload)
  return res.data.data
}

export async function updateAdminSkill(
  skillId: string,
  payload: UpdateSkillPayload
): Promise<AdminSkill> {
  const res = await api.patch(`/api/v1/admin/skills/${skillId}`, payload)
  return res.data.data
}

export async function createAdminSkillVersion(
  skillId: string,
  payload: CreateVersionPayload
): Promise<SkillVersionMetadata> {
  const res = await api.post(
    `/api/v1/admin/skills/${skillId}/versions`,
    payload
  )
  return res.data.data
}

export async function getAdminSkillVersions(
  skillId: string,
  params?: { page?: number; limit?: number }
): Promise<AdminSkillVersionsResponse> {
  const res = await api.get(`/api/v1/admin/skills/${skillId}/versions`, {
    params,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getAdminSkillAuditLog(
  skillId: string,
  params?: { page?: number; limit?: number }
): Promise<AdminSkillAuditLogResponse> {
  const res = await api.get(`/api/v1/admin/skills/${skillId}/audit-log`, {
    params,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}
