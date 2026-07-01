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
import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { Activity, AlertCircle, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDateTimeStr, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getUserSkillUsage } from '../../api'
import type {
  User,
  UserSkillUsageDownloadRow,
  UserSkillUsageResponse,
  UserSkillUsageTimelineRow,
} from '../../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user?: User
}

const usdFormatter = new Intl.NumberFormat(undefined, {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 4,
  maximumFractionDigits: 4,
})

function formatUSD(value: number | null | undefined) {
  if (value == null || Number.isNaN(value)) return '-'
  return usdFormatter.format(value)
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return formatDateTimeStr(date)
}

function successLabel(t: (key: string) => string, success?: boolean) {
  if (success === true) return t('Success')
  if (success === false) return t('Failed')
  return t('Unknown')
}

function skillUsageErrorMessage(error: unknown) {
  if (isAxiosError(error)) {
    const body = error.response?.data as
      | { error?: { message?: string }; message?: string }
      | undefined
    return body?.error?.message || body?.message || error.message
  }
  if (error instanceof Error) return error.message
  return 'Failed to load Skill usage'
}

function StatusPill({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'neutral' | 'success' | 'warning' | 'danger'
}) {
  return (
    <Badge
      variant='outline'
      className={cn(
        'h-6 rounded-full px-2.5 font-semibold',
        tone === 'success' &&
          'border-emerald-600/20 bg-emerald-600/10 text-emerald-700',
        tone === 'warning' &&
          'border-amber-600/20 bg-amber-600/10 text-amber-700',
        tone === 'danger' &&
          'border-destructive/20 bg-destructive/10 text-destructive'
      )}
    >
      {children}
    </Badge>
  )
}

export function UserSkillUsageDialog(props: Props) {
  const { t } = useTranslation()
  const userID = props.user?.id
  const query = useQuery({
    queryKey: ['admin-user-skill-usage', userID],
    queryFn: async () => {
      if (!userID) throw new Error('Missing user')
      const response = await getUserSkillUsage(userID)
      if (!response.data) {
        throw new Error('Failed to load Skill usage')
      }
      return response.data
    },
    enabled: props.open && Boolean(userID),
    retry: false,
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[88vh] max-w-[1120px] overflow-hidden p-0'>
        <DialogHeader className='border-border border-b px-6 py-5'>
          <DialogTitle className='flex items-center gap-2'>
            <Activity className='text-muted-foreground size-5' />
            {t('Skill usage')}
          </DialogTitle>
          <DialogDescription>
            {props.user
              ? t('Downloaded Skills and token usage for {{username}}.', {
                  username: props.user.username,
                })
              : t('No user selected')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[calc(88vh-96px)]'>
          <div className='space-y-5 p-6'>
            {query.isLoading && <LoadingState />}
            {query.isError && (
              <StateMessage
                icon={<AlertCircle className='size-4' />}
                title={t('Failed to load Skill usage')}
                description={t(skillUsageErrorMessage(query.error))}
              />
            )}
            {query.data && <UsageContent usage={query.data} />}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

function LoadingState() {
  return (
    <div className='space-y-4' data-testid='skill-usage-loading'>
      <div className='grid gap-3 md:grid-cols-3'>
        {[0, 1, 2].map((item) => (
          <div
            key={item}
            className='border-border bg-card h-20 animate-pulse rounded-xl border'
          />
        ))}
      </div>
      <div className='border-border bg-card h-52 animate-pulse rounded-xl border' />
    </div>
  )
}

function UsageContent({ usage }: { usage: UserSkillUsageResponse }) {
  const { t } = useTranslation()

  if (!usage.consent_granted) {
    return (
      <StateMessage
        icon={<ShieldCheck className='size-4' />}
        title={t('Skill usage unavailable')}
        description={t(
          'Tier 2 telemetry consent is not enabled for this user, so per-user Skill usage details are hidden.'
        )}
      />
    )
  }

  const hasDownloads = usage.downloads.length > 0
  const hasTimeline = usage.usage_timeline.length > 0

  return (
    <div className='space-y-5'>
      <div className='grid gap-3 md:grid-cols-3'>
        <SummaryCard
          label={t('Telemetry consent')}
          value={t('Granted')}
          tone='success'
        />
        <SummaryCard
          label={t('Kids protection')}
          value={usage.kids_protected ? t('Protected') : t('Not protected')}
          tone={usage.kids_protected ? 'warning' : 'neutral'}
        />
        <SummaryCard
          label={t('Downloaded Skills')}
          value={formatNumber(usage.downloads.length)}
        />
      </div>

      <section className='border-border bg-card rounded-xl border'>
        <div className='border-border flex items-center justify-between gap-3 border-b px-4 py-3'>
          <div>
            <h3 className='text-sm font-semibold'>{t('Downloaded Skills')}</h3>
            <p className='text-muted-foreground text-xs'>
              {t('Per-Skill token totals and estimated USD cost.')}
            </p>
          </div>
        </div>
        {hasDownloads ? (
          <DownloadsTable rows={usage.downloads} />
        ) : (
          <InlineEmpty
            title={t('No downloaded Skills')}
            description={t('This user has no downloaded Skill records yet.')}
          />
        )}
      </section>

      <section className='border-border bg-card rounded-xl border'>
        <div className='border-border flex items-center justify-between gap-3 border-b px-4 py-3'>
          <div>
            <h3 className='text-sm font-semibold'>{t('Usage timeline')}</h3>
            <p className='text-muted-foreground text-xs'>
              {t('Recent Skill usage events without raw prompts or payloads.')}
            </p>
          </div>
        </div>
        {hasTimeline ? (
          <TimelineTable rows={usage.usage_timeline} />
        ) : (
          <InlineEmpty
            title={t('No usage events')}
            description={t(
              'This user has no Skill usage events in the timeline.'
            )}
          />
        )}
      </section>
    </div>
  )
}

function SummaryCard({
  label,
  value,
  tone,
}: {
  label: string
  value: string
  tone?: 'neutral' | 'success' | 'warning'
}) {
  return (
    <div className='border-border bg-card rounded-xl border p-4'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-2 flex items-center justify-between gap-2'>
        <div className='text-foreground text-lg font-semibold tabular-nums'>
          {value}
        </div>
        {tone && <StatusPill tone={tone}>{value}</StatusPill>}
      </div>
    </div>
  )
}

function DownloadsTable({ rows }: { rows: UserSkillUsageDownloadRow[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Skill')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Enabled at')}</TableHead>
          <TableHead>{t('Last update')}</TableHead>
          <TableHead className='text-right'>{t('Input tokens')}</TableHead>
          <TableHead className='text-right'>{t('Output tokens')}</TableHead>
          <TableHead className='text-right'>{t('Total tokens')}</TableHead>
          <TableHead className='text-right'>{t('Cost')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow key={row.skill_id}>
            <TableCell>
              <div className='min-w-[170px]'>
                <div className='font-semibold'>{row.skill_name || '-'}</div>
                <div className='text-muted-foreground text-xs'>
                  {row.skill_slug || row.skill_id}
                </div>
              </div>
            </TableCell>
            <TableCell>
              <StatusPill tone={row.enabled ? 'success' : 'neutral'}>
                {row.enabled ? t('Enabled') : t('Disabled')}
              </StatusPill>
            </TableCell>
            <TableCell className='tabular-nums'>
              {formatDateTime(row.enabled_at)}
            </TableCell>
            <TableCell className='tabular-nums'>
              {formatDateTime(row.last_update_time)}
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatNumber(row.input_tokens)}
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatNumber(row.output_tokens)}
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatNumber(row.total_tokens)}
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatUSD(row.cost_usd)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function TimelineTable({ rows }: { rows: UserSkillUsageTimelineRow[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('Event')}</TableHead>
          <TableHead>{t('Skill')}</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead className='text-right'>{t('Total tokens')}</TableHead>
          <TableHead className='text-right'>{t('Cost')}</TableHead>
          <TableHead>{t('Result')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow key={row.event_id}>
            <TableCell className='tabular-nums'>
              {formatDateTime(row.occurred_at)}
            </TableCell>
            <TableCell>{row.event_type}</TableCell>
            <TableCell>
              <div className='min-w-[150px]'>
                <div className='font-semibold'>{row.skill_name || '-'}</div>
                <div className='text-muted-foreground text-xs'>
                  {row.skill_slug || row.skill_id || '-'}
                </div>
              </div>
            </TableCell>
            <TableCell>{row.model || '-'}</TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatNumber(row.total_tokens)}
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatUSD(row.cost_usd)}
            </TableCell>
            <TableCell>
              <StatusPill
                tone={
                  row.success === true
                    ? 'success'
                    : row.success === false
                      ? 'danger'
                      : 'neutral'
                }
              >
                {successLabel(t, row.success)}
              </StatusPill>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function StateMessage({
  icon,
  title,
  description,
}: {
  icon: ReactNode
  title: string
  description: string
}) {
  return (
    <Empty className='border-border bg-card min-h-72 border'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>{icon}</EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

function InlineEmpty({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <div className='px-4 py-8'>
      <Empty className='border-border/70 bg-background/40 border'>
        <EmptyHeader>
          <EmptyTitle>{title}</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}
