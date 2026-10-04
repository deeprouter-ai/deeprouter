// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'

const TRIGGER_PX = 70

/**
 * Native-style pull-to-refresh for the window scroller: dragging down from
 * the very top past a threshold runs `onRefresh`. Returns how far the user
 * has pulled (for the indicator) and whether a refresh is running.
 */
export function usePullToRefresh(onRefresh: () => Promise<unknown>) {
  const [pull, setPull] = useState(0)
  const [refreshing, setRefreshing] = useState(false)
  const startY = useRef<number | null>(null)
  const pullRef = useRef(0)
  const refreshRef = useRef(onRefresh)
  useEffect(() => {
    refreshRef.current = onRefresh
  })

  useEffect(() => {
    const onStart = (e: TouchEvent) => {
      startY.current = window.scrollY <= 0 ? e.touches[0].clientY : null
    }
    const onMove = (e: TouchEvent) => {
      if (startY.current === null) return
      const delta = e.touches[0].clientY - startY.current
      pullRef.current = delta > 0 ? Math.min(delta * 0.5, TRIGGER_PX * 1.5) : 0
      setPull(pullRef.current)
    }
    const onEnd = () => {
      if (startY.current === null) return
      startY.current = null
      const shouldRefresh = pullRef.current >= TRIGGER_PX
      pullRef.current = 0
      setPull(0)
      if (!shouldRefresh) return
      setRefreshing(true)
      void refreshRef.current().finally(() => setRefreshing(false))
    }
    window.addEventListener('touchstart', onStart, { passive: true })
    window.addEventListener('touchmove', onMove, { passive: true })
    window.addEventListener('touchend', onEnd)
    return () => {
      window.removeEventListener('touchstart', onStart)
      window.removeEventListener('touchmove', onMove)
      window.removeEventListener('touchend', onEnd)
    }
  }, [])

  return { pull, refreshing, ready: pull >= TRIGGER_PX }
}
