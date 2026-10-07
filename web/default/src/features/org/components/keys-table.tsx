// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import type { TFunction } from 'i18next'
import { Bot, Pencil, RefreshCw, Snowflake, Sun, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  handPickedModels,
  isSwitchedOn,
  keyModelsSummary,
  keyState,
  keyStateLabel,
  maskedKey,
  type OrgKeyState,
} from '../lib/keys'
import type { OrgKey } from '../types'

type KeysTableProps = {
  keys: OrgKey[]
  /**
   * What the viewer may do with a key. Leave one out and its button goes; leave
   * all out — a read-only member — and the column goes with them.
   */
  onEdit?: (key: OrgKey) => void
  onRotate?: (key: OrgKey) => void
  onFreeze?: (key: OrgKey) => void
  onUnfreeze?: (key: OrgKey) => void
  onDelete?: (key: OrgKey) => void
  /** The key an action is running on; its buttons wait. */
  busyKeyId?: number | null
}

const STATE_VARIANT: Record<OrgKeyState, StatusVariant> = {
  active: 'success',
  frozen: 'neutral',
  expired: 'danger',
  exhausted: 'warning',
}

/** The limits of a key that are set, in one line; a dash when none is. */
function limitsOf(key: OrgKey, t: TFunction): string {
  const limits = [
    key.rpm_limit > 0 && `${key.rpm_limit} ${t('requests/min')}`,
    key.tpm_limit > 0 && `${key.tpm_limit} ${t('tokens/min')}`,
    key.monthly_limit > 0 && `${key.monthly_limit} ${t('requests/month')}`,
  ].filter(Boolean)
  return limits.length > 0 ? limits.join(' · ') : '—'
}

/**
 * The organization's keys the viewer may see (Enterprise Org PRD §3). The
 * value column is always the masked form: nothing on this page can show a
 * key, and nothing on it asks the backend for one.
 */
export function KeysTable({
  keys,
  onEdit,
  onRotate,
  onFreeze,
  onUnfreeze,
  onDelete,
  busyKeyId,
}: KeysTableProps) {
  const { t } = useTranslation()
  const acts = Boolean(onEdit || onRotate || onFreeze || onUnfreeze || onDelete)
  // Read once: whether a key has expired is judged against the moment the
  // table appeared, and the list is fetched again after every action.
  const [now] = useState(() => Math.floor(Date.now() / 1000))

  return (
    <div className='bg-card overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Name')}</TableHead>
            <TableHead>{t('Held by')}</TableHead>
            <TableHead>{t('Key')}</TableHead>
            <TableHead>{t('May call')}</TableHead>
            <TableHead>{t('Quota left')}</TableHead>
            <TableHead>{t('Limits')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            {acts && (
              <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {keys.map((key) => {
            const state = keyState(key, now)
            const busy = busyKeyId === key.id
            const picked = handPickedModels(key)
            return (
              <TableRow key={key.id}>
                <TableCell className='px-3 font-medium'>{key.name}</TableCell>
                <TableCell>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span>{key.holder || t('Someone who has left')}</span>
                    {key.holder_is_service && (
                      <Badge variant='outline'>
                        <Bot aria-hidden='true' />
                        {t('Service account')}
                      </Badge>
                    )}
                  </div>
                  {key.department && (
                    <div className='text-muted-foreground text-xs'>
                      {key.department}
                    </div>
                  )}
                </TableCell>
                <TableCell className='text-muted-foreground font-mono text-xs'>
                  {maskedKey(key)}
                </TableCell>
                <TableCell>
                  {keyModelsSummary(t, key)}
                  {/* A count says nothing about which models: a hand-picked
                      list is spelled out, for the members who cannot open the
                      form as well. */}
                  {picked.length > 0 && (
                    <div
                      className='text-muted-foreground line-clamp-2 max-w-56 text-xs break-words'
                      title={picked.join('\n')}
                    >
                      {picked.join(t(', '))}
                    </div>
                  )}
                </TableCell>
                <TableCell className='tabular-nums'>
                  {key.unlimited_quota ? (
                    t('No limit')
                  ) : (
                    <>
                      <div>{formatQuota(key.remain_quota)}</div>
                      <div className='text-muted-foreground text-xs'>
                        {t('Used {{amount}}', {
                          amount: formatQuota(key.used_quota),
                        })}
                      </div>
                    </>
                  )}
                </TableCell>
                <TableCell className='text-muted-foreground text-xs'>
                  {limitsOf(key, t)}
                </TableCell>
                <TableCell>
                  <StatusBadge
                    label={keyStateLabel(t, state)}
                    variant={STATE_VARIANT[state]}
                    copyable={false}
                  />
                  {key.expired_time !== -1 && (
                    <div className='text-muted-foreground text-xs'>
                      {t('Expires {{date}}', {
                        date: formatTimestampToDate(key.expired_time),
                      })}
                    </div>
                  )}
                </TableCell>
                {acts && (
                  <TableCell className='px-3 text-right whitespace-nowrap'>
                    {onEdit && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onEdit(key)}
                        aria-label={t('Edit {{name}}', { name: key.name })}
                      >
                        <Pencil aria-hidden='true' />
                        {t('Edit')}
                      </Button>
                    )}
                    {onRotate && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onRotate(key)}
                        aria-label={t('Replace the value of {{name}}', {
                          name: key.name,
                        })}
                      >
                        <RefreshCw aria-hidden='true' />
                        {t('New value')}
                      </Button>
                    )}
                    {onFreeze && isSwitchedOn(key) && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onFreeze(key)}
                        aria-label={t('Freeze {{name}}', { name: key.name })}
                      >
                        <Snowflake aria-hidden='true' />
                        {t('Freeze')}
                      </Button>
                    )}
                    {onUnfreeze && !isSwitchedOn(key) && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onUnfreeze(key)}
                        aria-label={t('Unfreeze {{name}}', { name: key.name })}
                      >
                        <Sun aria-hidden='true' />
                        {t('Unfreeze')}
                      </Button>
                    )}
                    {onDelete && (
                      <Button
                        variant='ghost'
                        size='sm'
                        className='text-destructive hover:text-destructive'
                        disabled={busy}
                        onClick={() => onDelete(key)}
                        aria-label={t('Delete {{name}}', { name: key.name })}
                      >
                        <Trash2 aria-hidden='true' />
                        {t('Delete')}
                      </Button>
                    )}
                  </TableCell>
                )}
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
