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
import { useCallback, useEffect, useState } from 'react'

export type ApiKeyFormMode = 'simple' | 'advanced'

const STORAGE_KEY = 'api_key_form_mode'
const DEFAULT_MODE: ApiKeyFormMode = 'simple'

function readStoredMode(): ApiKeyFormMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw === 'simple' || raw === 'advanced') return raw
  } catch {
    /* ignore */
  }
  return DEFAULT_MODE
}

function writeStoredMode(mode: ApiKeyFormMode) {
  try {
    localStorage.setItem(STORAGE_KEY, mode)
  } catch {
    /* ignore */
  }
}

/**
 * API Key create-drawer mode toggle persisted in localStorage. Defaults to
 * 'simple' for first-time users (PRD docs/tasks/api-key-simple-advanced-prd.md
 * §3.1) and stays in sync across tabs via the storage event.
 */
export function useApiKeyFormMode(): [
  ApiKeyFormMode,
  (mode: ApiKeyFormMode) => void,
] {
  const [mode, setModeState] = useState<ApiKeyFormMode>(() => readStoredMode())

  const setMode = useCallback((next: ApiKeyFormMode) => {
    setModeState(next)
    writeStoredMode(next)
  }, [])

  useEffect(() => {
    const handleStorage = (e: StorageEvent) => {
      if (e.key !== STORAGE_KEY) return
      const next = e.newValue
      if (next === 'simple' || next === 'advanced') setModeState(next)
    }
    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [])

  return [mode, setMode]
}
