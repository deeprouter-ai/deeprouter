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
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { DataTableColumnHeader } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { SKILL_STATUS_VARIANTS, getSkillStatusOptions } from '../constants'
import type { SkillSummary } from '../types'
import { DataTableRowActions } from './data-table-row-actions'
import { FeaturedCell } from './skills-featured-cell'

export function useSkillsColumns(): ColumnDef<SkillSummary>[] {
  const { t } = useTranslation()
  const statusOptions = getSkillStatusOptions(t)

  return [
    {
      accessorKey: 'slug',
      meta: { label: t('Slug'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Slug')} />
      ),
      cell: ({ row }) => (
        <div className='max-w-[160px] truncate font-mono text-sm'>
          {row.getValue('slug')}
        </div>
      ),
    },
    {
      accessorKey: 'name',
      meta: { label: t('Name'), mobileTitle: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Name')} />
      ),
      cell: ({ row }) => (
        <div className='max-w-[180px] truncate font-medium'>
          {row.getValue('name')}
        </div>
      ),
    },
    {
      accessorKey: 'status',
      meta: { label: t('Status'), mobileBadge: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as SkillSummary['status']
        const option = statusOptions.find((o) => o.value === status)
        return (
          <StatusBadge
            label={option?.label ?? status}
            variant={SKILL_STATUS_VARIANTS[status]}
            showDot
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
    },
    {
      accessorKey: 'category',
      meta: { label: t('Category'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Category')} />
      ),
      cell: ({ row }) => (
        <span className='text-sm'>{row.getValue('category')}</span>
      ),
    },
    {
      id: 'monetization',
      meta: { label: t('Price') },
      header: t('Price'),
      cell: ({ row }) => {
        const skill = row.original
        return skill.monetization_type === 'free' ? (
          <StatusBadge label={t('Free')} variant='neutral' copyable={false} />
        ) : (
          <StatusBadge
            label={`$${skill.price_usd.toFixed(2)}`}
            variant='info'
            copyable={false}
          />
        )
      },
    },
    {
      id: 'active_version',
      meta: { label: t('Active Version'), mobileHidden: true },
      header: t('Active Version'),
      cell: ({ row }) => {
        const version = row.original.active_version
        return version ? (
          <span className='font-mono text-sm'>{version}</span>
        ) : (
          <span className='text-muted-foreground text-sm'>
            {t('No active version')}
          </span>
        )
      },
    },
    {
      id: 'featured',
      meta: { label: t('Featured'), mobileHidden: true },
      header: t('Featured'),
      cell: ({ row }) => <FeaturedCell skill={row.original} />,
      enableSorting: false,
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
    },
  ]
}
