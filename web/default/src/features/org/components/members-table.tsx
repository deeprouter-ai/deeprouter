// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { Bot, Pencil } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { orgRoleLabel } from '../lib/roles'
import type { OrgDepartment, OrgMember } from '../types'

type MembersTableProps = {
  members: OrgMember[]
  departments: OrgDepartment[]
  onEdit: (member: OrgMember) => void
}

/** Everyone in the organization: people and service accounts. */
export function MembersTable({
  members,
  departments,
  onEdit,
}: MembersTableProps) {
  const { t } = useTranslation()
  const departmentName = new Map(
    departments.map((department) => [department.id, department.name])
  )

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Member')}</TableHead>
            <TableHead>{t('Role')}</TableHead>
            <TableHead>{t('Department')}</TableHead>
            <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.map((member) => {
            const name = member.display_name || member.username
            // The username is how a person signs in (and, for a service
            // account, the generated id), so it is shown under the name —
            // unless it is the name, as it is for anyone who never set one.
            const detail = [
              member.username === name ? '' : member.username,
              member.email,
            ]
              .filter(Boolean)
              .join(' · ')
            return (
              <TableRow key={member.id}>
                <TableCell className='px-3'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='font-medium'>{name}</span>
                    {member.is_service && (
                      <Badge variant='outline'>
                        <Bot aria-hidden='true' />
                        {t('Service account')}
                      </Badge>
                    )}
                  </div>
                  {detail && (
                    <div className='text-muted-foreground text-xs'>
                      {detail}
                    </div>
                  )}
                </TableCell>
                <TableCell>
                  {member.is_owner ? (
                    <Badge>{orgRoleLabel(t, member.role)}</Badge>
                  ) : member.role === 'admin' ? (
                    <Badge variant='outline'>
                      {orgRoleLabel(t, member.role)}
                    </Badge>
                  ) : (
                    orgRoleLabel(t, member.role)
                  )}
                </TableCell>
                <TableCell>
                  {departmentName.get(member.department_id) ?? '—'}
                </TableCell>
                <TableCell className='px-3 text-right'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => onEdit(member)}
                    aria-label={t('Edit {{name}}', { name })}
                  >
                    <Pencil aria-hidden='true' />
                    {t('Edit')}
                  </Button>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
