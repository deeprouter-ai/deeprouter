// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { SearchIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { ScrollText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestamp } from '@/lib/format'
import { useDebounce } from '@/hooks/use-debounce'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { fetchOrgAuditLogs, orgQueryKeys, type OrgAuditParams } from '../api'
import {
  AUDIT_TARGET_TYPES,
  auditActionLabel,
  auditChangeText,
  auditChanges,
  auditTargetLabel,
} from '../lib/audit'
import { type OrgPeriod, periodBounds, periodIsValid } from '../lib/usage'
import type { OrgAuditLog } from '../types'
import { OrgSelect } from './org-select'
import { Pager } from './pager'
import { PeriodSelect } from './period-select'

/** How many records a page of the log holds. */
const RECORDS_PER_PAGE = 20

/** The choice of the target filter that narrows nothing down. */
const ANY_TARGET = ''

/** How long the name search waits after the last keystroke before it asks. */
const SEARCH_DELAY_MS = 300

/** The records of one page: who did what to what, when and from where. */
function AuditTable({ records }: { records: OrgAuditLog[] }) {
  const { t } = useTranslation()
  return (
    <div className='bg-card overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Time')}</TableHead>
            <TableHead>{t('Done by')}</TableHead>
            <TableHead>{t('What was done')}</TableHead>
            <TableHead>{t('Done to')}</TableHead>
            <TableHead>{t('Details')}</TableHead>
            <TableHead className='px-3'>{t('IP')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {records.map((record) => (
            <TableRow key={record.id}>
              <TableCell className='text-muted-foreground px-3 text-xs whitespace-nowrap tabular-nums'>
                {formatTimestamp(record.created_time)}
              </TableCell>
              <TableCell>
                {record.actor || t('An account that no longer exists')}
              </TableCell>
              <TableCell className='font-medium'>
                {auditActionLabel(t, record.action)}
              </TableCell>
              <TableCell>
                {/* What has no name of its own — an invite link, the alert
                    settings — is called by what it is. */}
                <div>
                  {record.target || auditTargetLabel(t, record.target_type)}
                </div>
                {record.target && (
                  <div className='text-muted-foreground text-xs'>
                    {auditTargetLabel(t, record.target_type)}
                  </div>
                )}
              </TableCell>
              <TableCell className='max-w-md min-w-56 whitespace-normal'>
                <ul className='text-muted-foreground grid gap-0.5 text-xs break-words'>
                  {auditChanges(t, record).map((change) => (
                    <li key={change.label}>{auditChangeText(t, change)}</li>
                  ))}
                </ul>
              </TableCell>
              <TableCell className='text-muted-foreground px-3 font-mono text-xs'>
                {record.ip || '—'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

/**
 * The audit log of the organization (Enterprise Org PRD D17): every change
 * someone made to its keys, members, roles, departments and settings — who,
 * what, when and from where — to read and to search, never to change. It
 * belongs to the organization as a whole, so only a role that reads it across
 * all of it gets this far.
 */
export function AuditLog() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [targetType, setTargetType] = useState(ANY_TARGET)
  const [period, setPeriod] = useState<OrgPeriod>({ preset: 'all' })
  const [page, setPage] = useState(1)
  // The name is searched for on the server, so it waits for the typing to
  // pause instead of asking on every keystroke.
  const actor = useDebounce(search.trim(), SEARCH_DELAY_MS)

  const params: OrgAuditParams = {
    p: page,
    page_size: RECORDS_PER_PAGE,
    ...(actor ? { actor } : {}),
    ...(targetType !== ANY_TARGET ? { target_type: targetType } : {}),
    ...periodBounds(period, new Date()),
  }
  const query = useQuery({
    queryKey: orgQueryKeys.auditLogs(params),
    queryFn: () => fetchOrgAuditLogs(params),
    enabled: periodIsValid(period),
    placeholderData: keepPreviousData,
  })
  const records = query.data?.items ?? []
  const narrowed =
    actor !== '' || targetType !== ANY_TARGET || period.preset !== 'all'
  const searchLabel = t('Search by who did it')

  return (
    <div className='grid gap-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <InputGroup className='w-full sm:w-64'>
          <InputGroupInput
            id='org-audit-actor'
            type='search'
            aria-label={searchLabel}
            placeholder={searchLabel}
            value={search}
            onChange={(event) => {
              setSearch(event.target.value)
              setPage(1)
            }}
          />
          <InputGroupAddon>
            <HugeiconsIcon
              icon={SearchIcon}
              strokeWidth={2}
              className='size-4 shrink-0 opacity-50'
            />
          </InputGroupAddon>
        </InputGroup>
        <div className='min-w-0 flex-1 sm:w-48 sm:flex-none'>
          <Label htmlFor='org-audit-target' className='sr-only'>
            {t('Done to')}
          </Label>
          <OrgSelect
            id='org-audit-target'
            options={[
              { value: ANY_TARGET, label: t('Everything') },
              ...AUDIT_TARGET_TYPES.map((type) => ({
                value: type,
                label: auditTargetLabel(t, type),
              })),
            ]}
            value={targetType}
            onChange={(next) => {
              setTargetType(next)
              setPage(1)
            }}
          />
        </div>
        <PeriodSelect
          id='org-audit-period'
          presets={['all', 'today', 'last7', 'last30', 'custom']}
          value={period}
          onChange={(next) => {
            setPeriod(next)
            setPage(1)
          }}
        />
      </div>

      {query.isError ? (
        <ErrorState
          description={t('Could not load the audit log.')}
          onRetry={() => void query.refetch()}
        />
      ) : !query.data ? (
        <div className='space-y-3'>
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className='h-12 rounded-xl' />
          ))}
        </div>
      ) : records.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title={narrowed ? t('No record matches') : t('Nothing recorded yet')}
          description={
            narrowed
              ? t('Try another name, another kind of thing or a longer period.')
              : t(
                  'Every change made to the organization’s keys, members, roles, departments and settings is recorded here.'
                )
          }
          bordered
        />
      ) : (
        <AuditTable records={records} />
      )}
      <Pager
        page={page}
        pageSize={RECORDS_PER_PAGE}
        total={query.data?.total ?? 0}
        onPageChange={setPage}
      />
    </div>
  )
}
