// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { Pencil, Trash2 } from 'lucide-react'
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
import { permissionLabel, scopeLabel } from '../lib/permissions'
import type { OrgRole } from '../types'

type CustomRolesTableProps = {
  roles: OrgRole[]
  /**
   * How many members hold each role, by role id. Leave out for a viewer who
   * does not see every member: a count of the few they see would mislead.
   */
  holders?: Map<number, number>
  /** Leave both out for a viewer who may not change roles: the column goes with them. */
  onEdit?: (role: OrgRole) => void
  onDelete?: (role: OrgRole) => void
}

/** The organization's own roles, each with what it grants and how far it reaches. */
export function CustomRolesTable({
  roles,
  holders,
  onEdit,
  onDelete,
}: CustomRolesTableProps) {
  const { t } = useTranslation()

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Role')}</TableHead>
            <TableHead>{t('Reach')}</TableHead>
            <TableHead>{t('Permissions')}</TableHead>
            {holders && <TableHead>{t('Members')}</TableHead>}
            {onEdit && onDelete && (
              <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {roles.map((role) => (
            <TableRow key={role.id}>
              <TableCell className='px-3 font-medium'>{role.name}</TableCell>
              <TableCell>{scopeLabel(t, role.scope)}</TableCell>
              <TableCell className='whitespace-normal'>
                <div className='flex flex-wrap gap-1'>
                  {role.permissions.map((primitive) => (
                    <Badge key={primitive} variant='outline'>
                      {permissionLabel(t, primitive)}
                    </Badge>
                  ))}
                </div>
              </TableCell>
              {holders && (
                <TableCell className='tabular-nums'>
                  {holders.get(role.id) ?? 0}
                </TableCell>
              )}
              {onEdit && onDelete && (
                <TableCell className='px-3 text-right'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => onEdit(role)}
                    aria-label={t('Edit {{name}}', { name: role.name })}
                  >
                    <Pencil aria-hidden='true' />
                    {t('Edit')}
                  </Button>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='text-destructive hover:text-destructive'
                    onClick={() => onDelete(role)}
                    aria-label={t('Delete {{name}}', { name: role.name })}
                  >
                    <Trash2 aria-hidden='true' />
                    {t('Delete')}
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
