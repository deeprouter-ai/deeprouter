/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useId, type CSSProperties } from 'react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'

type CountVisualVariant = 'users' | 'runs'

interface MetricCardProps {
  title: string
  value: string | null
  description: string
  icon: LucideIcon
  loading?: boolean
  trackingFailed?: boolean
  accentIndex?: number
  progressValue?: number | null
  progressColor?: string
  countValue?: number | null
  countVariant?: CountVisualVariant
  className?: string
}

const MAX_COUNT_MARKS = 8

const DEFAULT_PROGRESS_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
]

export function MetricCard({
  title,
  value,
  description,
  icon: Icon,
  loading,
  trackingFailed,
  accentIndex = 0,
  progressValue,
  progressColor,
  countValue,
  countVariant,
  className,
}: MetricCardProps) {
  const { t } = useTranslation()
  const gaugeId = useId().replace(/:/g, '')
  const gaugeColor =
    progressColor ??
    DEFAULT_PROGRESS_COLORS[accentIndex % DEFAULT_PROGRESS_COLORS.length]
  const gradientId = `metric-gradient-${gaugeId}`
  const clipId = `metric-clip-${gaugeId}`
  const progressPercent =
    progressValue == null || Number.isNaN(progressValue)
      ? null
      : Math.max(0, Math.min(100, progressValue * 100))
  const normalizedCount =
    countValue == null || Number.isNaN(countValue)
      ? null
      : Math.max(0, Math.floor(countValue))
  const visibleCount =
    normalizedCount == null ? 0 : Math.min(normalizedCount, MAX_COUNT_MARKS)
  const overflowCount =
    normalizedCount == null ? 0 : normalizedCount - visibleCount

  const displayValue = trackingFailed || value === null ? '—' : value

  const displayDesc = trackingFailed
    ? t('Tracking unavailable')
    : value === null
      ? t('No data in this period')
      : description

  return (
    <div
      className={cn(
        'bg-background/60 flex min-h-32 flex-col justify-between gap-3 overflow-hidden rounded-xl border p-3',
        className
      )}
    >
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
        <Icon
          className='text-muted-foreground/60 size-3.5 shrink-0'
          aria-hidden='true'
        />
        <span className='line-clamp-2 leading-snug'>{title}</span>
      </div>

      {loading ? (
        <div className='flex flex-col gap-1.5'>
          <Skeleton className='h-7 w-24' />
          <Skeleton className='h-3.5 w-32' />
        </div>
      ) : (
        <div className='flex flex-col gap-1'>
          <div
            className={cn(
              'font-mono text-2xl font-semibold tracking-tight break-all tabular-nums',
              displayValue === '—'
                ? 'text-muted-foreground/40'
                : 'text-foreground'
            )}
          >
            {displayValue}
          </div>
          <p className='text-muted-foreground/60 text-xs leading-relaxed'>
            {displayDesc}
          </p>
        </div>
      )}

      {progressPercent !== null && !trackingFailed ? (
        <svg
          viewBox='0 0 100 52'
          className='h-12 w-full'
          aria-label={t('Metric percentage progress')}
          role='meter'
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(progressPercent)}
          data-testid='metric-card-progress'
        >
          <defs>
            <clipPath id={clipId}>
              <path d='M8 50 A42 42 0 0 1 92 50 L8 50 Z' />
            </clipPath>
            <linearGradient id={gradientId} x1='0%' x2='100%' y1='0%' y2='0%'>
              <stop offset='0%' stopColor={gaugeColor} stopOpacity='0.22' />
              <stop offset='100%' stopColor={gaugeColor} stopOpacity='1' />
            </linearGradient>
          </defs>
          <path
            d='M8 50 A42 42 0 0 1 92 50 L8 50 Z'
            fill='currentColor'
            className='text-muted-foreground/15'
          />
          <g clipPath={`url(#${clipId})`}>
            <rect
              x={8}
              y={8}
              width={(84 * progressPercent) / 100}
              height={42}
              fill={`url(#${gradientId})`}
            />
          </g>
        </svg>
      ) : normalizedCount !== null && !trackingFailed ? (
        <div
          className='flex h-12 items-center gap-1.5'
          role='img'
          aria-label={t('Metric count visualization')}
          data-testid='metric-card-count-visual'
        >
          {Array.from({ length: visibleCount }, (_, index) => {
            const opacity =
              visibleCount <= 1 ? 1 : 0.28 + (index / (visibleCount - 1)) * 0.72
            return (
              <span
                key={index}
                className={cn(
                  'shrink-0 text-[color:var(--metric-color)]',
                  countVariant === 'runs'
                    ? 'h-0 w-0 border-y-[6px] border-l-[10px] border-y-transparent border-l-current'
                    : 'size-3.5 rounded-full bg-current'
                )}
                style={
                  {
                    '--metric-color': gaugeColor,
                    opacity,
                  } as CSSProperties
                }
              />
            )
          })}
          {overflowCount > 0 ? (
            <span className='text-muted-foreground rounded-full border px-1.5 py-0.5 text-[10px] font-semibold tabular-nums'>
              +{overflowCount}
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
