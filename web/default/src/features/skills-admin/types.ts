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

// Mirrors internal/skill-marketplace/model + service (P1/P2, deeprouter Go backend).

export type SkillStatus = 'draft' | 'published' | 'deprecated'
export type SkillVersionStatus = 'draft' | 'active' | 'archived'
export type MonetizationType = 'free' | 'paid'

export interface Skill {
  id: number
  slug: string
  name: string
  description: string
  category: string
  tags: string[]
  status: SkillStatus
  monetization_type: MonetizationType
  price_usd: number
  featured_flag: boolean
  featured_rank: number
  active_version_id?: number
  created_by: number
  created_at: string
  updated_at: string
}

// AdminListSkills embeds model.Skill and adds the active version's semver string.
export interface SkillSummary extends Skill {
  active_version?: string
}

export interface SkillVersion {
  id: number
  skill_id: number
  version: string
  status: SkillVersionStatus
  skill_md_content: string
  manifest_json: Record<string, unknown>
  package_sha256?: string
  package_built_at?: string
  changelog: string
  created_by: number
  created_at: string
}

export interface SkillAdminLog {
  id: number
  admin_id: number
  skill_id?: number
  action: string
  details: Record<string, unknown>
  created_at: string
}

// ============================================================================
// Request params
// ============================================================================

export interface ListSkillsParams {
  status?: SkillStatus
  category?: string
  q?: string
  page?: number
  page_size?: number
}

export interface CreateSkillRequest {
  slug: string
  name: string
  description: string
  category: string
  tags?: string[]
  monetization_type: MonetizationType
  price_usd?: number
}

export interface UpdateSkillRequest {
  slug?: string
  name?: string
  description?: string
  category?: string
  tags?: string[]
  monetization_type?: MonetizationType
  price_usd?: number
}

export interface FeaturedRequest {
  featured_flag: boolean
  featured_rank: number
}

export interface UploadVersionRequest {
  version: string
  skill_md_content: string
  manifest_json: Record<string, unknown>
  changelog?: string
}

export interface UpdateVersionRequest {
  skill_md_content?: string
  manifest_json?: Record<string, unknown>
  changelog?: string
}

// ============================================================================
// API response envelopes — {success, message?, data?}, same shape as
// features/channels (see controller/admin_marketplace.go's gin.H{...} wrapper)
// ============================================================================

export interface ListSkillsResponse {
  success: boolean
  message?: string
  data?: {
    skills: SkillSummary[]
    total: number
  }
}

export interface SkillResponse {
  success: boolean
  message?: string
  data?: Skill
}

// GetSkill (unlike Create/Update/Publish/…) returns the joined SkillSummary
// shape — see internal/skill-marketplace/service/admin_skill.go GetSkill.
export interface SkillDetailResponse {
  success: boolean
  message?: string
  data?: SkillSummary
}

export interface SkillVersionResponse {
  success: boolean
  message?: string
  data?: SkillVersion
}

export interface SkillVersionsResponse {
  success: boolean
  message?: string
  data?: SkillVersion[]
}

export interface SkillLogsResponse {
  success: boolean
  message?: string
  data?: SkillAdminLog[]
}

export interface DeleteResponse {
  success: boolean
  message?: string
}

// ============================================================================
// Dialog state (list page)
// ============================================================================

export type SkillsDialogType = 'create' | 'delete'
