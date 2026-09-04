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
import type { TFunction } from 'i18next'
import type { StatusVariant } from '@/components/status-badge'
import type { SkillStatus, SkillVersionStatus } from './types'

// PRD §5.1 suggested initial category list — free text in the DB, this is
// just the dropdown's suggestion list (Admin can still type a new value).
export const SKILL_CATEGORIES = [
  'writing',
  'translation',
  'code',
  'data-analysis',
  'research',
  'legal',
  'finance',
] as const

export const SKILL_STATUS_VARIANTS: Record<SkillStatus, StatusVariant> = {
  draft: 'neutral',
  published: 'success',
  deprecated: 'danger',
}

export const SKILL_VERSION_STATUS_VARIANTS: Record<
  SkillVersionStatus,
  StatusVariant
> = {
  draft: 'neutral',
  active: 'success',
  archived: 'info',
}

export function getSkillStatusOptions(t: TFunction) {
  return [
    { label: t('Draft'), value: 'draft' },
    { label: t('Published'), value: 'published' },
    { label: t('Deprecated'), value: 'deprecated' },
  ]
}

export function getSkillCategoryOptions(t: TFunction) {
  const labels: Record<(typeof SKILL_CATEGORIES)[number], string> = {
    writing: t('Writing'),
    translation: t('Translation'),
    code: t('Code'),
    'data-analysis': t('Data Analysis'),
    research: t('Research'),
    legal: t('Legal'),
    finance: t('Finance'),
  }
  return SKILL_CATEGORIES.map((value) => ({ label: labels[value], value }))
}

export const SKILL_SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
