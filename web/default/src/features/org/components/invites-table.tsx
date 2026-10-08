// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { Ban } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { CopyButton } from '@/components/copy-button'
import { orgInviteLink, orgRoleLabel } from '../lib/roles'
import type { OrgDepartment, OrgInvite } from '../types'

type InvitesTableProps = {
  invites: OrgInvite[]
  departments: OrgDepartment[]
  onRevoke: (invite: OrgInvite) => void
}

/** The invite links that still admit new members. */
export function InvitesTable({
  invites,
  departments,
  onRevoke,
}: InvitesTableProps) {
  const { t } = useTranslation()
  const departmentName = new Map(
    departments.map((department) => [department.id, department.name])
  )

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Role')}</TableHead>
            <TableHead>{t('Department')}</TableHead>
            <TableHead>{t('Expires')}</TableHead>
            <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {invites.map((invite) => (
            <TableRow key={invite.id}>
              <TableCell className='px-3 font-medium'>
                {orgRoleLabel(t, invite.role)}
              </TableCell>
              <TableCell>
                {departmentName.get(invite.department_id) ?? '—'}
              </TableCell>
              <TableCell className='tabular-nums'>
                {formatTimestampToDate(invite.expires_time)}
              </TableCell>
              <TableCell className='px-3 text-right'>
                <CopyButton
                  value={orgInviteLink(invite.code)}
                  size='sm'
                  aria-label={t('Copy link')}
                >
                  {t('Copy link')}
                </CopyButton>
                <Button
                  variant='ghost'
                  size='sm'
                  className='text-destructive hover:text-destructive'
                  onClick={() => onRevoke(invite)}
                >
                  <Ban aria-hidden='true' />
                  {t('Revoke')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
