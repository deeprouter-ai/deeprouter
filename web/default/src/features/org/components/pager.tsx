// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

type PagerProps = {
  /** The page on show, counted from 1. */
  page: number
  pageSize: number
  /** How many items there are over all pages. */
  total: number
  onPageChange: (page: number) => void
}

/**
 * Steps through a list the backend pages — alerts, audit records — and says
 * which stretch of it is on show. When the list shrinks under the page on
 * show, as it does when the last alert of a page is dealt with, it steps back
 * to the last page there is.
 */
export function Pager({ page, pageSize, total, onPageChange }: PagerProps) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(total / pageSize))
  useEffect(() => {
    if (page > pages) onPageChange(pages)
  }, [page, pages, onPageChange])
  if (total <= pageSize) return null
  const first = (page - 1) * pageSize + 1
  const last = Math.min(page * pageSize, total)
  return (
    <div className='flex flex-wrap items-center justify-between gap-2'>
      <p className='text-muted-foreground text-sm tabular-nums'>
        {t('{{first}}–{{last}} of {{total}}', { first, last, total })}
      </p>
      <div className='flex gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          {t('Previous page')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          disabled={page >= pages}
          onClick={() => onPageChange(page + 1)}
        >
          {t('Next page')}
        </Button>
      </div>
    </div>
  )
}
