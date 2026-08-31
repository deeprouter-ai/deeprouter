/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

// Unified config: globals/setup + jsdom for RTL component tests, plus the
// @vitejs/plugin-react needed to render them. `@` alias mirrors tsconfig
// paths via resolve.alias.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-utils/setup.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/features/user-home/**'],
      reporter: ['text', 'json-summary'],
      reportsDirectory: './coverage',
    },
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
    },
  },
})
