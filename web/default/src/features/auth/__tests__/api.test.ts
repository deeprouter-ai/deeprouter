// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import i18n from 'i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { register } from '../api'

// Enterprise Org P3: the server names a new organization's starter
// departments in the language of the sign-up request. That has to be the
// language of the page — the browser's own Accept-Language is whatever it was
// installed with, and someone reading the site in Chinese on an English
// browser would get English departments.

const { mockPost } = vi.hoisted(() => ({ mockPost: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { post: mockPost } }))

const pageLanguage = i18n.language

afterEach(() => {
  i18n.language = pageLanguage
  vi.clearAllMocks()
})

describe('register', () => {
  // A language picked in the switcher is "zh" or "en"; one detected from the
  // browser keeps its region, which the server tells apart (zh-TW).
  it.each(['zh', 'en', 'zh-TW'])(
    'tells the server the page is in %s',
    async (language) => {
      i18n.language = language
      mockPost.mockResolvedValue({ data: { success: true } })

      await register({
        username: 'founder',
        password: 'password123',
        org_name: 'Acme',
      })

      expect(mockPost).toHaveBeenCalledWith(
        '/api/user/register',
        expect.objectContaining({ org_name: 'Acme' }),
        expect.objectContaining({ headers: { 'Accept-Language': language } })
      )
    }
  )
})
