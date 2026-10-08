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
import type { OrgDepartment } from '../types'

type DepartmentsTableProps = {
  departments: OrgDepartment[]
  /** Leave both out for a viewer who may not change departments: the column goes with them. */
  onRename?: (department: OrgDepartment) => void
  onDelete?: (department: OrgDepartment) => void
}

/** The departments the viewer may see; the default one cannot be deleted. */
export function DepartmentsTable({
  departments,
  onRename,
  onDelete,
}: DepartmentsTableProps) {
  const { t } = useTranslation()

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Department')}</TableHead>
            <TableHead>{t('Members')}</TableHead>
            {onRename && onDelete && (
              <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {departments.map((department) => (
            <TableRow key={department.id}>
              <TableCell className='px-3'>
                <div className='flex flex-wrap items-center gap-2'>
                  <span className='font-medium'>{department.name}</span>
                  {department.is_default && (
                    <Badge variant='outline'>{t('Default')}</Badge>
                  )}
                </div>
              </TableCell>
              <TableCell className='tabular-nums'>
                {department.member_count}
              </TableCell>
              {onRename && onDelete && (
                <TableCell className='px-3 text-right'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => onRename(department)}
                    aria-label={t('Rename {{name}}', {
                      name: department.name,
                    })}
                  >
                    <Pencil aria-hidden='true' />
                    {t('Rename')}
                  </Button>
                  {/* The default department takes in whoever has no other
                      department, so it is the one that stays. */}
                  {!department.is_default && (
                    <Button
                      variant='ghost'
                      size='sm'
                      className='text-destructive hover:text-destructive'
                      onClick={() => onDelete(department)}
                      aria-label={t('Delete {{name}}', {
                        name: department.name,
                      })}
                    >
                      <Trash2 aria-hidden='true' />
                      {t('Delete')}
                    </Button>
                  )}
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
