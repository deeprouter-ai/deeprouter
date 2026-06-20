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
import { useEffect, useState } from 'react'

type DocState =
  | { status: 'loading'; content: '' }
  | { status: 'ready'; content: string }
  | { status: 'error'; content: '' }

/**
 * Fetches an integration markdown file from `public/docs/integrations/<slug>.md`
 * at runtime. Content is plain markdown served as a static asset, so there is no
 * bundler-specific import wiring.
 */
export function useDocContent(slug: string): DocState {
  const [state, setState] = useState<DocState>({ status: 'loading', content: '' })

  useEffect(() => {
    let cancelled = false
    setState({ status: 'loading', content: '' })

    fetch(`/docs/integrations/${slug}.md`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.text()
      })
      .then((text) => {
        // A SPA fallback can return index.html for a missing file; guard against it.
        if (/^\s*<!doctype html/i.test(text) || /^\s*<html/i.test(text)) {
          throw new Error('not found')
        }
        if (!cancelled) setState({ status: 'ready', content: text })
      })
      .catch(() => {
        if (!cancelled) setState({ status: 'error', content: '' })
      })

    return () => {
      cancelled = true
    }
  }, [slug])

  return state
}
