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
import { useCallback, useEffect, useState } from 'react'

type DocState =
  | { status: 'loading'; content: ''; reason: null }
  | { status: 'ready'; content: string; reason: null }
  | { status: 'error'; content: ''; reason: string }

export interface DocResult {
  status: DocState['status']
  content: string
  /** Human-readable failure cause when status is 'error' (e.g. "HTTP 404"). */
  reason: string | null
  /** Re-fetch the same slug — used by the error state's Retry button. */
  reload: () => void
}

/**
 * Fetches an integration markdown file from `public/docs/integrations/<slug>.md`
 * at runtime. Content is plain markdown served as a static asset, so there is no
 * bundler-specific import wiring.
 *
 * On failure the returned `reason` explains why (bad HTTP status, or a stale
 * server / cache that returned the SPA HTML shell instead of the file), so the UI
 * can surface a diagnosable message rather than a bare "could not be loaded".
 */
export function useDocContent(slug: string): DocResult {
  const [state, setState] = useState<DocState>({
    status: 'loading',
    content: '',
    reason: null,
  })
  // Bumping this re-runs the effect to retry the fetch.
  const [attempt, setAttempt] = useState(0)
  const reload = useCallback(() => setAttempt((n) => n + 1), [])

  useEffect(() => {
    let cancelled = false
    setState({ status: 'loading', content: '', reason: null })

    fetch(`/docs/integrations/${slug}.md`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.text()
      })
      .then((text) => {
        // A stale server / SPA fallback can return index.html instead of the
        // file; treat that as "not found (stale cache)" rather than markdown.
        if (/^\s*<!doctype html/i.test(text) || /^\s*<html/i.test(text)) {
          throw new Error('stale cache (got HTML, not markdown)')
        }
        if (!cancelled) setState({ status: 'ready', content: text, reason: null })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        const reason = err instanceof Error ? err.message : 'network error'
        setState({ status: 'error', content: '', reason })
      })

    return () => {
      cancelled = true
    }
  }, [slug, attempt])

  return { status: state.status, content: state.content, reason: state.reason, reload }
}
