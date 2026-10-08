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
// A skill saved before tags were normalized can still hold "Writing"; its
// detail-page badge links here with ?tags=Writing. The filter chips compare
// against lowercase values, so without this no chip lit up and clicking
// "Writing" selected a second, duplicate value.
import { describe, expect, it } from 'vitest'
import { normalizeTagParam } from '../lib/tags'

describe('normalizeTagParam', () => {
  it('wraps a single value in an array', () => {
    expect(normalizeTagParam('writing')).toEqual(['writing'])
  })

  it('lowercases, trims, and drops blanks and duplicates', () => {
    expect(normalizeTagParam([' Writing', 'writing', '', 'CODE'])).toEqual([
      'writing',
      'code',
    ])
  })
})
