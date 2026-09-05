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
import type { TFunction } from 'i18next'
import { SKILL_SLUG_PATTERN } from '../constants'

// Shared by the list page's create drawer and the edit page's metadata form.
export function getCreateSkillFormSchema(t: TFunction) {
  return z
    .object({
      slug: z
        .string()
        .min(1, t('Slug is required'))
        .max(100, t('Slug must be 100 characters or fewer'))
        .regex(
          SKILL_SLUG_PATTERN,
          t('Slug must be lowercase letters, numbers and hyphens only')
        ),
      name: z.string().min(1, t('Name is required')).max(200),
      description: z.string().min(1, t('Description is required')),
      category: z.string().min(1, t('Category is required')),
      tags: z.string().optional(),
      monetization_type: z.enum(['free', 'paid']),
      price_usd: z.number().min(0).optional(),
    })
    .refine(
      (data) => data.monetization_type === 'free' || (data.price_usd ?? 0) > 0,
      {
        message: t('Price must be greater than 0 for a paid skill'),
        path: ['price_usd'],
      }
    )
}

export type CreateSkillFormValues = z.infer<
  ReturnType<typeof getCreateSkillFormSchema>
>

export const CREATE_SKILL_FORM_DEFAULT_VALUES: CreateSkillFormValues = {
  slug: '',
  name: '',
  description: '',
  category: '',
  tags: '',
  monetization_type: 'free',
  price_usd: 0,
}

// tags is a comma-separated string in the form, string[] on the wire.
export function parseTagsInput(value?: string): string[] {
  if (!value) return []
  return value
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

// tags is typed string[] but the API boundary can still hand back null
// (a prior backend bug did exactly that for untagged skills, and crashed
// this form outright) — guard it here rather than trust the wire.
export function formatTagsInput(tags: string[] | null | undefined): string {
  return (tags ?? []).join(', ')
}

// The edit page's metadata form. `slug` is only ever submitted while the
// skill is still draft (AC-9) — the form disables/hides the field once the
// skill has been published, but the schema validates it unconditionally so
// a draft-state edit gets the same format check CreateSkill uses.
export function getUpdateSkillFormSchema(t: TFunction) {
  return z
    .object({
      slug: z
        .string()
        .min(1, t('Slug is required'))
        .max(100, t('Slug must be 100 characters or fewer'))
        .regex(
          SKILL_SLUG_PATTERN,
          t('Slug must be lowercase letters, numbers and hyphens only')
        ),
      name: z.string().min(1, t('Name is required')).max(200),
      description: z.string().min(1, t('Description is required')),
      category: z.string().min(1, t('Category is required')),
      tags: z.string().optional(),
      monetization_type: z.enum(['free', 'paid']),
      price_usd: z.number().min(0).optional(),
    })
    .refine(
      (data) => data.monetization_type === 'free' || (data.price_usd ?? 0) > 0,
      {
        message: t('Price must be greater than 0 for a paid skill'),
        path: ['price_usd'],
      }
    )
}

export type UpdateSkillFormValues = z.infer<
  ReturnType<typeof getUpdateSkillFormSchema>
>

// The version upload drawer.
export function getUploadVersionFormSchema(t: TFunction) {
  return z.object({
    version: z
      .string()
      .regex(/^\d+\.\d+\.\d+$/, t('Version must be in X.Y.Z format')),
    skill_md_content: z.string().min(1, t('SKILL.md content is required')),
    manifest_json: z.string().refine((value) => {
      try {
        JSON.parse(value)
        return true
      } catch {
        return false
      }
    }, t('manifest.json must be valid JSON')),
    changelog: z.string().optional(),
  })
}

export type UploadVersionFormValues = z.infer<
  ReturnType<typeof getUploadVersionFormSchema>
>

// The version edit drawer — same shape as upload minus `version`, which is
// immutable once a version exists (PUT /versions/:vid never accepts it;
// bumping the version number means uploading a new one).
export function getUpdateVersionFormSchema(t: TFunction) {
  return z.object({
    skill_md_content: z.string().min(1, t('SKILL.md content is required')),
    manifest_json: z.string().refine((value) => {
      try {
        JSON.parse(value)
        return true
      } catch {
        return false
      }
    }, t('manifest.json must be valid JSON')),
    changelog: z.string().optional(),
  })
}

export type UpdateVersionFormValues = z.infer<
  ReturnType<typeof getUpdateVersionFormSchema>
>
