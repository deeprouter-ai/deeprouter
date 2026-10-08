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
// Coverage: a real browser click-through caught what neither the Go unit
// tests nor curl-built query strings could — axios's default array
// serialization is tags[]=a&tags[]=b, which Gin's `form:"tags"` binding on
// a []string field never matches, so the filter silently returned every
// skill regardless of selection. A fully-mocked api.get (this repo's usual
// api.ts test style, see skills-admin/__tests__/api.test.ts) would not have
// caught this: the mock never serializes anything, so it can only prove
// "the right options were passed," not "the right URL comes out." The
// second test below uses axios's real URL-building instead, for that
// reason.
import axios from 'axios'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchMarketplaceSkills } from '../api'

const { mockGet } = vi.hoisted(() => ({ mockGet: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get: mockGet } }))

beforeEach(() => {
  vi.clearAllMocks()
  mockGet.mockResolvedValue({ data: { data: { skills: [], total: 0 } } })
})

describe('fetchMarketplaceSkills', () => {
  it('passes a paramsSerializer that avoids bracket notation for tags', async () => {
    await fetchMarketplaceSkills({ tags: ['writing', 'code'], limit: 100 })
    expect(mockGet).toHaveBeenCalledWith(
      '/api/skills',
      expect.objectContaining({ paramsSerializer: { indexes: null } })
    )
  })

  it('serializes multiple tags as repeated tags= keys, not tags[]=', () => {
    // Exercises axios's actual query-string builder with the same
    // paramsSerializer fetchMarketplaceSkills passes — this is the part a
    // mocked api.get cannot verify.
    const url = axios.getUri({
      url: '/api/skills',
      params: { tags: ['writing', 'code'], limit: 100 },
      paramsSerializer: { indexes: null },
    })
    expect(url).toBe('/api/skills?tags=writing&tags=code&limit=100')
    expect(url).not.toContain('tags[]')
    expect(url).not.toContain('tags%5B%5D')
  })
})
