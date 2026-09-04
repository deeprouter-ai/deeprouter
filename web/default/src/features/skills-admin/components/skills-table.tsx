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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage } from '@/components/data-table'
import { listSkills } from '../api'
import { getSkillStatusOptions } from '../constants'
import type { SkillStatus } from '../types'
import { useSkillsColumns } from './skills-columns'
import { useSkills } from './skills-provider'

const route = getRouteApi('/_authenticated/admin/skills/')

export function SkillsTable() {
  const { t } = useTranslation()
  const columns = useSkillsColumns()
  const { refreshTrigger } = useSkills()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [{ columnId: 'status', searchKey: 'status', type: 'array' }],
  })

  const statusFilter = (columnFilters.find((f) => f.id === 'status')?.value ??
    []) as SkillStatus[]
  const singleStatus = statusFilter.length === 1 ? statusFilter[0] : undefined

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'admin-skills',
      pagination.pageIndex + 1,
      pagination.pageSize,
      singleStatus,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await listSkills({
        status: singleStatus,
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      return {
        skills: result.data?.skills || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const skills = data?.skills || []

  const table = useReactTable({
    data: skills,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      globalFilter,
      pagination,
    },
    enableRowSelection: false,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    globalFilterFn: (row, _columnId, filterValue) => {
      const name = String(row.getValue('name')).toLowerCase()
      const slug = String(row.getValue('slug')).toLowerCase()
      const searchValue = String(filterValue).toLowerCase()
      return name.includes(searchValue) || slug.includes(searchValue)
    },
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    // status filtering happens server-side (single value) — client-side
    // filterFn on the status column still applies when multiple are chosen.
    manualPagination: !globalFilter,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  const statusOptions = useMemo(() => getSkillStatusOptions(t), [t])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Skills Found')}
      emptyDescription={t(
        'No skills yet. Create your first skill to get started.'
      )}
      skeletonKeyPrefix='skills-admin-skeleton'
      toolbarProps={{
        searchPlaceholder: t('Filter by name or slug...'),
        filters: [
          { columnId: 'status', title: t('Status'), options: statusOptions },
        ],
      }}
    />
  )
}
