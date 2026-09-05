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
  CreateSkillRequest,
  DeleteResponse,
  FeaturedRequest,
  ListSkillsParams,
  ListSkillsResponse,
  SkillDetailResponse,
  SkillLogsResponse,
  SkillResponse,
  SkillVersionResponse,
  SkillVersionsResponse,
  UpdateSkillRequest,
  UpdateVersionRequest,
  UploadVersionRequest,
} from './types'

// router/api-router.go: apiRouter.Group("/admin/skills"), gated by AdminAuth()
const BASE = '/api/admin/skills'

// ============================================================================
// Skill CRUD + state machine + featured + logs (P1)
// ============================================================================

export async function listSkills(
  params: ListSkillsParams = {}
): Promise<ListSkillsResponse> {
  const res = await api.get(`${BASE}/`, { params })
  return res.data
}

export async function getSkill(id: number): Promise<SkillDetailResponse> {
  const res = await api.get(`${BASE}/${id}`)
  return res.data
}

export async function createSkill(
  data: CreateSkillRequest
): Promise<SkillResponse> {
  const res = await api.post(`${BASE}/`, data)
  return res.data
}

export async function updateSkill(
  id: number,
  data: UpdateSkillRequest
): Promise<SkillResponse> {
  const res = await api.put(`${BASE}/${id}`, data)
  return res.data
}

export async function publishSkill(id: number): Promise<SkillResponse> {
  const res = await api.post(`${BASE}/${id}/publish`)
  return res.data
}

export async function deprecateSkill(id: number): Promise<SkillResponse> {
  const res = await api.post(`${BASE}/${id}/deprecate`)
  return res.data
}

export async function deleteSkill(id: number): Promise<DeleteResponse> {
  const res = await api.delete(`${BASE}/${id}`)
  return res.data
}

export async function updateFeatured(
  id: number,
  data: FeaturedRequest
): Promise<SkillResponse> {
  const res = await api.put(`${BASE}/${id}/featured`, data)
  return res.data
}

export async function getSkillLogs(id: number): Promise<SkillLogsResponse> {
  const res = await api.get(`${BASE}/${id}/logs`)
  return res.data
}

// ============================================================================
// Version management + activation (P2)
// ============================================================================

export async function listVersions(
  skillId: number
): Promise<SkillVersionsResponse> {
  const res = await api.get(`${BASE}/${skillId}/versions`)
  return res.data
}

export async function uploadVersion(
  skillId: number,
  data: UploadVersionRequest
): Promise<SkillVersionResponse> {
  const res = await api.post(`${BASE}/${skillId}/versions`, data)
  return res.data
}

export async function updateVersion(
  skillId: number,
  versionId: number,
  data: UpdateVersionRequest
): Promise<SkillVersionResponse> {
  const res = await api.put(`${BASE}/${skillId}/versions/${versionId}`, data)
  return res.data
}

export async function activateVersion(
  skillId: number,
  versionId: number
): Promise<SkillVersionResponse> {
  const res = await api.post(
    `${BASE}/${skillId}/versions/${versionId}/activate`
  )
  return res.data
}

export async function deleteVersion(
  skillId: number,
  versionId: number
): Promise<DeleteResponse> {
  const res = await api.delete(`${BASE}/${skillId}/versions/${versionId}`)
  return res.data
}
