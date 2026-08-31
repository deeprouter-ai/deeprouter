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
import React, { useState, useCallback, useMemo, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import useDialogState from '@/hooks/use-dialog'
import { fetchTokenKey, fetchTokenKeysBatch, getApiKeys } from '../api'
import { API_KEY_STATUS, ERROR_MESSAGES } from '../constants'
import { type ApiKey, type ApiKeysDialogType } from '../types'

type ApiKeysContextType = {
  open: ApiKeysDialogType | null
  setOpen: (str: ApiKeysDialogType | null) => void
  currentRow: ApiKey | null
  setCurrentRow: React.Dispatch<React.SetStateAction<ApiKey | null>>
  refreshTrigger: number
  triggerRefresh: () => void
  setupKeys: ApiKey[]
  setupKeyId: number | null
  setSetupKeyId: (id: number) => void
  setupKey: ApiKey | null
  resolvedKey: string
  setResolvedKey: React.Dispatch<React.SetStateAction<string>>
  resolveRealKey: (id: number) => Promise<string | null>
  resolveRealKeysBatch: (ids: number[]) => Promise<Record<number, string>>
  resolvedKeys: Record<number, string>
  loadingKeys: Record<number, boolean>
  copiedKeyId: number | null
  markKeyCopied: (id: number) => void
}

const ApiKeysContext = React.createContext<ApiKeysContextType | null>(null)

export function ApiKeysProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useDialogState<ApiKeysDialogType>(null)
  const [currentRow, setCurrentRow] = useState<ApiKey | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const [resolvedKey, setResolvedKey] = useState('')

  const [resolvedKeys, setResolvedKeys] = useState<Record<number, string>>({})
  const [loadingKeys, setLoadingKeys] = useState<Record<number, boolean>>({})
  const pendingRequests = useRef<Record<number, Promise<string | null>>>({})

  const [copiedKeyId, setCopiedKeyId] = useState<number | null>(null)
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    return () => clearTimeout(copiedTimerRef.current)
  }, [])

  const markKeyCopied = useCallback((id: number) => {
    setCopiedKeyId(id)
    clearTimeout(copiedTimerRef.current)
    copiedTimerRef.current = setTimeout(() => setCopiedKeyId(null), 2000)
  }, [])

  const triggerRefresh = useCallback(() => {
    setRefreshTrigger((prev) => prev + 1)
  }, [])

  // Which key the setup card configures. It lives here rather than inside that
  // card because two surfaces act on it — the terminal commands and the
  // chat-app buttons — and while each kept its own idea of "the current key"
  // they could point at different ones: the terminal block took the newest key
  // and the chat buttons the newest *enabled* one, so a page whose newest key
  // was disabled configured two different keys without saying so (2026-08-28).
  const [setupKeys, setSetupKeys] = useState<ApiKey[]>([])
  const [setupKeyId, setSetupKeyId] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        // The list comes back newest-first, which is also the default pick:
        // creating a key and then setting it up is the common path. Disabled
        // keys are skipped — configuring a tool with one buys a 401 inside the
        // tool days later, with nothing on this page to explain it.
        const res = await getApiKeys({ p: 1, size: 50 })
        if (cancelled) return
        const items = res.data?.items ?? []
        setSetupKeys(items)
        setSetupKeyId((prev) =>
          prev !== null && items.some((k) => k.id === prev)
            ? prev
            : (items.find((k) => k.status === API_KEY_STATUS.ENABLED)?.id ??
              items[0]?.id ??
              null)
        )
      } catch {
        if (!cancelled) {
          setSetupKeys([])
          setSetupKeyId(null)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [refreshTrigger])

  const setupKey = useMemo(
    () => setupKeys.find((k) => k.id === setupKeyId) ?? null,
    [setupKeys, setupKeyId]
  )

  const resolveRealKey = useCallback(
    async (id: number): Promise<string | null> => {
      if (resolvedKeys[id]) return resolvedKeys[id]
      if (id in pendingRequests.current) return pendingRequests.current[id]

      const request = (async () => {
        setLoadingKeys((prev) => ({ ...prev, [id]: true }))
        try {
          const res = await fetchTokenKey(id)
          if (res.success && res.data?.key) {
            const fullKey = `sk-${res.data.key}`
            setResolvedKeys((prev) => ({ ...prev, [id]: fullKey }))
            return fullKey
          }
          toast.error(res.message || t(ERROR_MESSAGES.UNEXPECTED))
          return null
        } catch {
          toast.error(t(ERROR_MESSAGES.UNEXPECTED))
          return null
        } finally {
          delete pendingRequests.current[id]
          setLoadingKeys((prev) => {
            const next = { ...prev }
            delete next[id]
            return next
          })
        }
      })()

      pendingRequests.current[id] = request
      return request
    },
    [resolvedKeys, t]
  )

  const resolveRealKeysBatch = useCallback(
    async (ids: number[]): Promise<Record<number, string>> => {
      const uncachedIds = ids.filter((id) => !resolvedKeys[id])
      if (uncachedIds.length === 0) {
        const result: Record<number, string> = {}
        for (const id of ids) result[id] = resolvedKeys[id]
        return result
      }

      for (const id of uncachedIds) {
        setLoadingKeys((prev) => ({ ...prev, [id]: true }))
      }

      try {
        const res = await fetchTokenKeysBatch(uncachedIds)
        if (res.success && res.data?.keys) {
          const newKeys: Record<number, string> = {}
          for (const [idStr, key] of Object.entries(res.data.keys)) {
            newKeys[Number(idStr)] = `sk-${key}`
          }
          setResolvedKeys((prev) => ({ ...prev, ...newKeys }))

          const result: Record<number, string> = { ...newKeys }
          for (const id of ids) {
            if (resolvedKeys[id]) result[id] = resolvedKeys[id]
          }
          return result
        }
        toast.error(res.message || t(ERROR_MESSAGES.UNEXPECTED))
        return {}
      } catch {
        toast.error(t(ERROR_MESSAGES.UNEXPECTED))
        return {}
      } finally {
        for (const id of uncachedIds) {
          setLoadingKeys((prev) => {
            const next = { ...prev }
            delete next[id]
            return next
          })
        }
      }
    },
    [resolvedKeys, t]
  )

  return (
    <ApiKeysContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
        setupKeys,
        setupKeyId,
        setSetupKeyId,
        setupKey,
        resolvedKey,
        setResolvedKey,
        resolveRealKey,
        resolveRealKeysBatch,
        resolvedKeys,
        loadingKeys,
        copiedKeyId,
        markKeyCopied,
      }}
    >
      {children}
    </ApiKeysContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useApiKeys = () => {
  const apiKeysContext = React.useContext(ApiKeysContext)

  if (!apiKeysContext) {
    throw new Error('useApiKeys has to be used within <ApiKeysContext>')
  }

  return apiKeysContext
}
