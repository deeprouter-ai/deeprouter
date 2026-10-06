// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  groupByResource,
  permissionHint,
  permissionLabel,
  resourceLabel,
  scopeLabel,
} from '../lib/permissions'
import { orgRoleLabel } from '../lib/roles'
import type { OrgPermissionCatalog, OrgRole } from '../types'

type RoleMatrixProps = {
  /** The preset roles, in the order the backend lists them. */
  roles: OrgRole[]
  catalog: OrgPermissionCatalog
}

/**
 * What each preset role may do: one row per permission, one column per role
 * (Enterprise Org PRD §2). Every row and every tick comes from the backend —
 * the primitives and powers from the catalogue, the ticks from the roles — so
 * the table cannot say something the permission engine does not do.
 */
export function RoleMatrix({ roles, catalog }: RoleMatrixProps) {
  const { t } = useTranslation()

  const mark = (allowed: boolean) =>
    allowed ? (
      <>
        <Check aria-hidden='true' className='text-primary mx-auto size-4' />
        <span className='sr-only'>{t('Allowed')}</span>
      </>
    ) : (
      <>
        <span aria-hidden='true' className='text-muted-foreground/50'>
          —
        </span>
        <span className='sr-only'>{t('Not allowed')}</span>
      </>
    )

  const row = (name: string, allowed: (role: OrgRole) => boolean) => (
    <TableRow key={name}>
      <TableHead scope='row' className='h-auto px-3 py-2 whitespace-normal'>
        <div className='text-foreground font-medium'>
          {permissionLabel(t, name)}
        </div>
        <div className='text-muted-foreground text-xs font-normal'>
          {permissionHint(t, name)}
        </div>
      </TableHead>
      {roles.map((role) => (
        <TableCell key={role.id} className='text-center'>
          {mark(allowed(role))}
        </TableCell>
      ))}
    </TableRow>
  )

  // A band across the table that names the group of rows under it.
  const heading = (key: string, title: string, note?: string) => (
    <TableRow key={key} className='bg-muted/40 hover:bg-muted/40'>
      <TableCell
        colSpan={roles.length + 1}
        className='text-muted-foreground px-3 py-1.5 text-xs font-semibold whitespace-normal'
      >
        {title}
        {note && <span className='font-normal'> — {note}</span>}
      </TableCell>
    </TableRow>
  )

  return (
    <div className='bg-card overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='min-w-56 px-3'>{t('Permission')}</TableHead>
            {roles.map((role) => (
              <TableHead
                key={role.id}
                scope='col'
                className='h-auto min-w-24 py-2 text-center whitespace-normal'
              >
                <div className='text-foreground'>
                  {orgRoleLabel(t, role.name)}
                </div>
                <div className='text-muted-foreground text-xs font-normal'>
                  {scopeLabel(t, role.scope)}
                </div>
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {groupByResource(catalog.primitives).flatMap((group) => [
            heading(group.resource, resourceLabel(t, group.resource)),
            ...group.primitives.map((primitive) =>
              row(primitive, (role) => role.permissions.includes(primitive))
            ),
          ])}
          {heading(
            'powers',
            t('Running the organization'),
            t('comes with being the owner or an admin; no role can grant it')
          )}
          {catalog.powers.map((power) =>
            row(power.name, (role) => role.powers.includes(power.name))
          )}
        </TableBody>
      </Table>
    </div>
  )
}
