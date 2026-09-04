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
// Coverage: the comma-separated <-> string[] round trip for `tags`, and the
// manifest_json validator wired into the upload form's zod schema.
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  formatTagsInput,
  getUploadVersionFormSchema,
  parseTagsInput,
} from './skill-form'

const t = ((key: string) => key) as TFunction

describe('parseTagsInput', () => {
  it('splits, trims and drops empty entries', () => {
    expect(parseTagsInput('code, review,  , translation')).toEqual([
      'code',
      'review',
      'translation',
    ])
  })

  it('returns an empty array for undefined/empty input', () => {
    expect(parseTagsInput(undefined)).toEqual([])
    expect(parseTagsInput('')).toEqual([])
  })
})

describe('formatTagsInput', () => {
  it('joins tags with ", " so parseTagsInput can round-trip it', () => {
    const tags = ['code', 'review']
    expect(formatTagsInput(tags)).toBe('code, review')
    expect(parseTagsInput(formatTagsInput(tags))).toEqual(tags)
  })

  // Regression: a backend bug once sent `tags: null` for any untagged
  // skill (a plain []string has no driver.Valuer for Postgres text[]) and
  // this crashed SkillMetadataForm outright — `tags` is typed string[] but
  // must not be trusted at the API boundary.
  it('treats null/undefined as no tags instead of throwing', () => {
    expect(formatTagsInput(null)).toBe('')
    expect(formatTagsInput(undefined)).toBe('')
  })
})

describe('getUploadVersionFormSchema manifest_json validation', () => {
  const schema = getUploadVersionFormSchema(t)
  const base = {
    version: '1.0.0',
    skill_md_content: '# x',
    changelog: '',
  }

  it('rejects invalid JSON', () => {
    const result = schema.safeParse({ ...base, manifest_json: '{not json' })
    expect(result.success).toBe(false)
  })

  it('accepts valid JSON regardless of shape — field-level checks are the backend’s job', () => {
    const result = schema.safeParse({
      ...base,
      manifest_json: '{"slug":"x","version":"1.0.0"}',
    })
    expect(result.success).toBe(true)
  })

  it('rejects a version that is not semver X.Y.Z', () => {
    const result = schema.safeParse({
      ...base,
      version: '1.0',
      manifest_json: '{}',
    })
    expect(result.success).toBe(false)
  })
})
