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
import { z } from 'zod'
import { quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import { type UserFormData, type User } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export const userFormSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  display_name: z.string().optional(),
  password: z.string().optional(),
  role: z.number().optional(),
  quota_dollars: z.number().min(0).optional(),
  group: z.string().optional(),
  remark: z.string().optional(),
  // Airbotix / DeepRouter tenant fields (update only)
  kids_mode: z.boolean().optional(),
  policy_profile: z.enum(['passthrough', 'adult', 'kid-safe']).optional(),
  billing_webhook_url: z.string().url().or(z.literal('')).optional(),
  custom_pricing_id: z.string().optional(),
  webhook_secret: z.string().optional(),
  // Auto top-up — quota units (1 USD = QuotaPerUnit units, default 500000)
  auto_topup_enabled: z.boolean().optional(),
  auto_topup_threshold: z.number().int().nonnegative().optional(),
  auto_topup_amount: z.number().int().nonnegative().optional(),
})

export type UserFormValues = z.infer<typeof userFormSchema>

// ============================================================================
// Form Defaults
// ============================================================================

export const USER_FORM_DEFAULT_VALUES: UserFormValues = {
  username: '',
  display_name: '',
  password: '',
  role: 1, // Default to common user
  quota_dollars: 0,
  group: DEFAULT_GROUP,
  remark: '',
  kids_mode: false,
  policy_profile: 'passthrough',
  billing_webhook_url: '',
  custom_pricing_id: '',
  webhook_secret: '',
  auto_topup_enabled: false,
  auto_topup_threshold: 0,
  auto_topup_amount: 0,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: UserFormValues,
  userId?: number
): UserFormData & { id?: number } {
  const payload: UserFormData & { id?: number } = {
    username: data.username,
    display_name: data.display_name || data.username,
    password: data.password || undefined,
  }

  // For create: only send required fields
  if (userId === undefined) {
    payload.role = data.role || 1 // Default to common user
  } else {
    // For update: quota is adjusted atomically via /api/user/manage, not sent here
    payload.group = data.group
    payload.remark = data.remark || undefined
    payload.id = userId
    // Airbotix tenant fields — only meaningful on update
    payload.kids_mode = data.kids_mode ?? false
    payload.policy_profile = data.policy_profile || 'passthrough'
    payload.billing_webhook_url = data.billing_webhook_url || ''
    payload.custom_pricing_id = data.custom_pricing_id || ''
    payload.webhook_secret = data.webhook_secret || ''
    payload.auto_topup_enabled = data.auto_topup_enabled ?? false
    payload.auto_topup_threshold = data.auto_topup_threshold ?? 0
    payload.auto_topup_amount = data.auto_topup_amount ?? 0
  }

  return payload
}

/**
 * Transform user data to form defaults
 */
export function transformUserToFormDefaults(user: User): UserFormValues {
  const profile = user.policy_profile
  const normalisedProfile =
    profile === 'kid-safe' || profile === 'adult' || profile === 'passthrough'
      ? profile
      : 'passthrough'
  return {
    username: user.username,
    display_name: user.display_name,
    password: '',
    role: user.role,
    quota_dollars: quotaUnitsToDollars(user.quota),
    group: user.group || DEFAULT_GROUP,
    remark: user.remark || '',
    kids_mode: user.kids_mode ?? false,
    policy_profile: normalisedProfile,
    billing_webhook_url: user.billing_webhook_url || '',
    custom_pricing_id: user.custom_pricing_id || '',
    webhook_secret: user.webhook_secret || '',
    auto_topup_enabled: user.auto_topup_enabled ?? false,
    auto_topup_threshold: user.auto_topup_threshold ?? 0,
    auto_topup_amount: user.auto_topup_amount ?? 0,
  }
}
