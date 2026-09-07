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
// Shapes returned by the Skill Marketplace V2 user-facing API (P3):
// GET /api/skills, GET /api/skills/:slug, GET /api/user/skills,
// GET /api/user/purchases. All arrive inside the standard
// { success, message, data } envelope; these types describe `data`.

export type SkillStatus = 'published' | 'deprecated'

export interface MarketplaceSkill {
  id: number
  slug: string
  name: string
  description: string
  category: string
  tags: string[]
  status: SkillStatus
  monetization_type: 'free' | 'paid'
  price_usd: number
  featured_flag: boolean
  featured_rank: number
  created_at: string
  updated_at: string
  /** Active version semver; empty when the skill has no active version. */
  version: string
}

export interface MarketplaceSkillDetail extends MarketplaceSkill {
  changelog: string
}

export interface MarketplaceListParams {
  category?: string
  q?: string
  page?: number
  limit?: number
}

export interface MarketplaceListData {
  skills: MarketplaceSkill[]
  total: number
  page: number
  limit: number
}

export interface UserSkillEntry {
  skill_id: number
  slug: string
  name: string
  version: string
  enabled_at: string
  /** 'deprecated' means the delisted badge must be shown. */
  skill_status: SkillStatus
}

export interface UserSkillsData {
  skills: UserSkillEntry[]
  total: number
  page: number
  limit: number
}

export interface UserPurchaseEntry {
  skill_id: number
  slug: string
  name: string
  price_usd: number
  purchased_at: string
}

export interface UserPurchasesData {
  purchases: UserPurchaseEntry[]
  total: number
  page: number
  limit: number
}
