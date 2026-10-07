// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { SearchIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { memberDepartments, memberMatches, memberRoles } from '../lib/members'
import { ANY, type PeopleFacet } from '../lib/people-filter'
import { orgRoleLabel } from '../lib/roles'
import { MembersTable, type MembersTableProps } from './members-table'
import { PeopleFacetSelect } from './people-facet-select'

/**
 * What a filter is set to, as long as that is still one of its choices. The
 * list changes under the filters — a member is removed, or moved — and a
 * choice that is gone leaves the filter open instead of emptying the table.
 */
function stillChosen(choices: PeopleFacet[], chosen: number): number {
  return choices.some((choice) => choice.id === chosen) ? chosen : ANY
}

/**
 * The members, findable: the table under a search field and two filters, by
 * department and by role (PRD D39). They narrow the list down the way the
 * "who is it for" list of a key does (D34), with two differences that come
 * with being a table: the search also looks in the username and the email,
 * which the rows show, and everyone who matches is listed, however many.
 */
export function MembersDirectory({
  members,
  departments,
  ...table
}: MembersTableProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [chosenDepartmentId, setChosenDepartmentId] = useState(ANY)
  const [chosenRoleId, setChosenRoleId] = useState(ANY)

  const departmentChoices = memberDepartments(members, departments)
  const roleChoices = memberRoles(members)
  // A filter with one choice narrows nothing down and is left out: a manager
  // of a single department gets none by department.
  const byDepartment = departmentChoices.length > 1
  const byRole = roleChoices.length > 1
  const departmentId = stillChosen(departmentChoices, chosenDepartmentId)
  const roleId = stillChosen(roleChoices, chosenRoleId)
  const matching = members.filter((member) =>
    memberMatches(member, { search, departmentId, roleId })
  )
  const searchLabel = t('Search by name, username or email')

  return (
    <div className='space-y-3'>
      {/* On a phone the search takes a line of its own and the filters share
          the next one. */}
      <div className='flex flex-wrap gap-2'>
        <InputGroup className='w-full sm:w-72'>
          <InputGroupInput
            id='org-members-search'
            type='search'
            aria-label={searchLabel}
            placeholder={searchLabel}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <InputGroupAddon>
            <HugeiconsIcon
              icon={SearchIcon}
              strokeWidth={2}
              className='size-4 shrink-0 opacity-50'
            />
          </InputGroupAddon>
        </InputGroup>
        {byDepartment && (
          <div className='min-w-0 flex-1 sm:w-48 sm:flex-none'>
            <PeopleFacetSelect
              id='org-members-department'
              label={t('Department')}
              any={t('All departments')}
              choices={departmentChoices}
              value={departmentId}
              onChange={setChosenDepartmentId}
            />
          </div>
        )}
        {byRole && (
          <div className='min-w-0 flex-1 sm:w-48 sm:flex-none'>
            <PeopleFacetSelect
              id='org-members-role'
              label={t('Role')}
              any={t('All roles')}
              choices={roleChoices.map((role) => ({
                id: role.id,
                name: orgRoleLabel(t, role.name),
              }))}
              value={roleId}
              onChange={setChosenRoleId}
            />
          </div>
        )}
      </div>
      {matching.length === 0 ? (
        <p className='bg-card text-muted-foreground rounded-xl border px-3 py-8 text-center text-sm'>
          {t('Nobody matches.')}
        </p>
      ) : (
        <MembersTable members={matching} departments={departments} {...table} />
      )}
    </div>
  )
}
