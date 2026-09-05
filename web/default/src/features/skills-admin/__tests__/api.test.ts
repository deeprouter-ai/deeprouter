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
// Coverage: each wrapper hits the exact method + URL that
// router/api-router.go actually registers under /api/admin/skills. Two of
// these (deleteSkill, deleteVersion) were never exercised by the real-
// browser walkthrough either — nothing ever clicked Delete — so a URL typo
// here had no safety net at all before this file.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as skillsApi from '../api'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: { get: mockGet, post: mockPost, put: mockPut, delete: mockDelete },
}))

beforeEach(() => {
  vi.clearAllMocks()
  const ok = { data: { success: true } }
  mockGet.mockResolvedValue(ok)
  mockPost.mockResolvedValue(ok)
  mockPut.mockResolvedValue(ok)
  mockDelete.mockResolvedValue(ok)
})

describe('skills-admin api', () => {
  it('listSkills: GET /api/admin/skills/ with params', async () => {
    await skillsApi.listSkills({ status: 'draft', page: 2 })
    expect(mockGet).toHaveBeenCalledWith('/api/admin/skills/', {
      params: { status: 'draft', page: 2 },
    })
  })

  it('getSkill: GET /api/admin/skills/:id', async () => {
    await skillsApi.getSkill(5)
    expect(mockGet).toHaveBeenCalledWith('/api/admin/skills/5')
  })

  it('createSkill: POST /api/admin/skills/', async () => {
    const body = {
      slug: 's',
      name: 'n',
      description: 'd',
      category: 'c',
      monetization_type: 'free' as const,
    }
    await skillsApi.createSkill(body)
    expect(mockPost).toHaveBeenCalledWith('/api/admin/skills/', body)
  })

  it('updateSkill: PUT /api/admin/skills/:id', async () => {
    await skillsApi.updateSkill(5, { name: 'renamed' })
    expect(mockPut).toHaveBeenCalledWith('/api/admin/skills/5', {
      name: 'renamed',
    })
  })

  it('publishSkill: POST /api/admin/skills/:id/publish', async () => {
    await skillsApi.publishSkill(5)
    expect(mockPost).toHaveBeenCalledWith('/api/admin/skills/5/publish')
  })

  it('deprecateSkill: POST /api/admin/skills/:id/deprecate', async () => {
    await skillsApi.deprecateSkill(5)
    expect(mockPost).toHaveBeenCalledWith('/api/admin/skills/5/deprecate')
  })

  it('deleteSkill: DELETE /api/admin/skills/:id', async () => {
    await skillsApi.deleteSkill(5)
    expect(mockDelete).toHaveBeenCalledWith('/api/admin/skills/5')
  })

  it('updateFeatured: PUT /api/admin/skills/:id/featured', async () => {
    await skillsApi.updateFeatured(5, { featured_flag: true, featured_rank: 1 })
    expect(mockPut).toHaveBeenCalledWith('/api/admin/skills/5/featured', {
      featured_flag: true,
      featured_rank: 1,
    })
  })

  it('getSkillLogs: GET /api/admin/skills/:id/logs', async () => {
    await skillsApi.getSkillLogs(5)
    expect(mockGet).toHaveBeenCalledWith('/api/admin/skills/5/logs')
  })

  it('listVersions: GET /api/admin/skills/:id/versions', async () => {
    await skillsApi.listVersions(5)
    expect(mockGet).toHaveBeenCalledWith('/api/admin/skills/5/versions')
  })

  it('uploadVersion: POST /api/admin/skills/:id/versions', async () => {
    const body = {
      version: '1.0.0',
      skill_md_content: '# x',
      manifest_json: {},
    }
    await skillsApi.uploadVersion(5, body)
    expect(mockPost).toHaveBeenCalledWith('/api/admin/skills/5/versions', body)
  })

  it('updateVersion: PUT /api/admin/skills/:id/versions/:vid', async () => {
    await skillsApi.updateVersion(5, 9, { changelog: 'x' })
    expect(mockPut).toHaveBeenCalledWith('/api/admin/skills/5/versions/9', {
      changelog: 'x',
    })
  })

  it('activateVersion: POST /api/admin/skills/:id/versions/:vid/activate', async () => {
    await skillsApi.activateVersion(5, 9)
    expect(mockPost).toHaveBeenCalledWith(
      '/api/admin/skills/5/versions/9/activate'
    )
  })

  it('deleteVersion: DELETE /api/admin/skills/:id/versions/:vid', async () => {
    await skillsApi.deleteVersion(5, 9)
    expect(mockDelete).toHaveBeenCalledWith('/api/admin/skills/5/versions/9')
  })
})
