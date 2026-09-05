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
import { describe, expect, it } from 'vitest'
import {
  DownloadSkillError,
  extractDownloadError,
  filenameFromContentDisposition,
  sanitizeFilename,
} from './download-utils'

describe('sanitizeFilename', () => {
  it('strips path separators from the name', () => {
    expect(sanitizeFilename('a/b\\c.zip', 'slug')).toBe('abc.zip')
  })

  it('strips surrounding quotes', () => {
    expect(sanitizeFilename('"x.zip"', 'slug')).toBe('x.zip')
  })

  it('falls back to <slug>.zip when name is empty', () => {
    expect(sanitizeFilename(undefined, 'my-skill')).toBe('my-skill.zip')
    expect(sanitizeFilename('', 'my-skill')).toBe('my-skill.zip')
  })

  it('sanitizes the fallback slug too', () => {
    expect(sanitizeFilename(undefined, 'a/b')).toBe('ab.zip')
  })

  it('uses "skill" when the fallback is empty', () => {
    expect(sanitizeFilename(undefined, '')).toBe('skill.zip')
  })
})

describe('filenameFromContentDisposition', () => {
  it('prefers RFC 5987 filename*= and decodes it', () => {
    expect(
      filenameFromContentDisposition(
        "attachment; filename*=UTF-8''%E4%B8%AD.zip",
        'slug'
      )
    ).toBe('中.zip')
  })

  it('falls back to filename=', () => {
    expect(
      filenameFromContentDisposition('attachment; filename=foo.zip', 'slug')
    ).toBe('foo.zip')
  })

  it('falls back to the slug when the header is missing', () => {
    expect(filenameFromContentDisposition(undefined, 'my-skill')).toBe(
      'my-skill.zip'
    )
  })
})

describe('extractDownloadError', () => {
  const envelope = (over: object = {}) =>
    JSON.stringify({
      success: false,
      message: 'insufficient balance, please top up in Wallet',
      data: { price_usd: 2, quota_needed: 1_000_000 },
      ...over,
    })

  it('maps 402 to INSUFFICIENT_BALANCE with the price details (Blob body)', async () => {
    const err = await extractDownloadError(402, new Blob([envelope()]))
    expect(err).toBeInstanceOf(DownloadSkillError)
    expect(err.code).toBe('INSUFFICIENT_BALANCE')
    expect(err.priceUsd).toBe(2)
    expect(err.quotaNeeded).toBe(1_000_000)
    expect(err.message).toContain('insufficient balance')
  })

  it('parses string bodies too', async () => {
    const err = await extractDownloadError(402, envelope())
    expect(err.code).toBe('INSUFFICIENT_BALANCE')
    expect(err.quotaNeeded).toBe(1_000_000)
  })

  it('maps 404 to NOT_FOUND', async () => {
    const err = await extractDownloadError(
      404,
      JSON.stringify({ success: false, message: 'skill not found' })
    )
    expect(err.code).toBe('NOT_FOUND')
  })

  it('survives non-JSON (gateway HTML) bodies', async () => {
    const err = await extractDownloadError(500, '<html>bad gateway</html>')
    expect(err.code).toBe('DOWNLOAD_FAILED')
  })

  it('survives an unreadable body entirely', async () => {
    const err = await extractDownloadError(undefined, undefined)
    expect(err.code).toBe('DOWNLOAD_FAILED')
  })
})
