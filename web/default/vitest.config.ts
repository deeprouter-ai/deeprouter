import react from '@vitejs/plugin-react'
import tsconfigPaths from 'vite-tsconfig-paths'
import { defineConfig } from 'vitest/config'

// Vitest runs independently of the rsbuild production build. jsdom is required
// because download-utils (isSafeDownloadUrl) reads window.location and the
// component tests render React. tsconfigPaths mirrors the `@/*` alias.
export default defineConfig({
  plugins: [react(), tsconfigPaths()],
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
