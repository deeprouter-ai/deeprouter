// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { ArrowDownLeft, Loader2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { getUserLogs } from '@/features/usage-logs/api'
import { LOG_TYPE_ENUM } from '@/features/usage-logs/constants'
import { usePullToRefresh } from '../hooks/use-pull-to-refresh'
import { findPurpose, purposeFromKeyName } from '../lib/purposes'

type Row = {
  id: number
  type: number
  quota: number
  created_at: number
  token_name?: string
}

/** Top-ups and spend, newest first; `null` when the request failed. */
async function fetchRows(): Promise<Row[] | null> {
  try {
    const res = await getUserLogs({ p: 1, page_size: 50 })
    return ((res.data?.items ?? []) as unknown as Row[]).filter(
      (r) => r.type === LOG_TYPE_ENUM.CONSUME || r.type === LOG_TYPE_ENUM.TOPUP
    )
  } catch {
    return null
  }
}

/**
 * Spending in plain words, like a banking app statement: what it was for,
 * when, and how much — top-ups as income, usage as spend. No model names,
 * token counts or request ids (those stay in the professional console).
 */
export function SimpleRecords() {
  const { t, i18n } = useTranslation()
  const [rows, setRows] = useState<Row[] | null>(null)
  const [failed, setFailed] = useState(false)

  const apply = useCallback((result: Row[] | null) => {
    if (result) {
      setRows(result)
      setFailed(false)
    } else {
      setFailed(true)
      setRows((prev) => prev ?? [])
    }
  }, [])

  const load = useCallback(() => fetchRows().then(apply), [apply])

  useEffect(() => {
    let cancelled = false
    void fetchRows().then((result) => {
      if (!cancelled) apply(result)
    })
    return () => {
      cancelled = true
    }
  }, [apply])

  const { pull, refreshing } = usePullToRefresh(load)

  const when = (seconds: number) =>
    new Intl.DateTimeFormat(i18n.language, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(seconds * 1000))

  return (
    <div className='flex flex-col px-4 pt-4'>
      <div
        className='flex items-center justify-center overflow-hidden'
        style={{ height: refreshing ? 28 : pull * 0.4 }}
      >
        {refreshing && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>
      <h1 className='px-1 pb-3 text-2xl font-semibold'>{t('Records')}</h1>

      {rows === null ? (
        <div className='flex justify-center py-16'>
          <Loader2 className='text-muted-foreground size-6 animate-spin' />
        </div>
      ) : rows.length === 0 ? (
        <p className='text-muted-foreground py-16 text-center text-sm'>
          {failed
            ? t('Could not load your records. Pull down to try again.')
            : t('Nothing yet. What you spend on AI will show up here.')}
        </p>
      ) : (
        <ul className='bg-card border-border divide-border divide-y overflow-hidden rounded-2xl border'>
          {rows.map((row) => {
            const topup = row.type === LOG_TYPE_ENUM.TOPUP
            const purpose = findPurpose(
              purposeFromKeyName(row.token_name) ?? ''
            )
            const Icon = topup ? ArrowDownLeft : (purpose?.icon ?? Sparkles)
            const title = topup
              ? t('Added credit')
              : purpose
                ? t(purpose.title)
                : t('AI usage')
            return (
              <li key={row.id} className='flex items-center gap-3 px-4 py-3'>
                <span
                  className={cn(
                    'inline-flex size-10 shrink-0 items-center justify-center rounded-full',
                    topup
                      ? 'bg-emerald-500/10 text-emerald-600'
                      : 'bg-muted text-foreground'
                  )}
                >
                  <Icon className='size-5' />
                </span>
                <span className='min-w-0 flex-1'>
                  <span className='block truncate text-[15px] font-medium'>
                    {title}
                  </span>
                  <span className='text-muted-foreground block text-xs'>
                    {when(row.created_at)}
                  </span>
                </span>
                <span
                  className={cn(
                    'text-[15px] font-semibold tabular-nums',
                    topup && 'text-emerald-600'
                  )}
                >
                  {topup ? '+' : '−'}
                  {formatQuota(row.quota)}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
