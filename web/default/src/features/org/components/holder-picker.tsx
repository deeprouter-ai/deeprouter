// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useMemo, useState } from 'react'
import { UnfoldMoreIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Bot } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  holderDepartments,
  holderMatches,
  holderRoles,
  holdersInOrder,
} from '../lib/holders'
import { keyHolderLabel } from '../lib/keys'
import { ANY } from '../lib/people-filter'
import { orgRoleLabel } from '../lib/roles'
import type { OrgKeyHolder } from '../types'
import { PeopleFacetSelect } from './people-facet-select'

/**
 * How many people the list shows at once. A company can have thousands, and
 * nobody reads that far: past this the list says how many there are and asks
 * to be narrowed down.
 */
const MAX_ROWS = 100

type HolderPickerProps = {
  /** The id the field's label points at. */
  id: string
  holders: OrgKeyHolder[]
  /** The id of whoever is chosen; one that is nobody's leaves the field open. */
  value: number
  onChange: (holderId: number) => void
  /** What the field says while nobody is chosen. */
  placeholder?: string
}

/**
 * Who a key is for, out of everyone it can be made out to. In a company of a
 * few people this is a short list; in a large one it is found by typing a
 * name, or narrowed down to a department or a role first.
 */
export function HolderPicker({
  id,
  holders,
  value,
  onChange,
  placeholder,
}: HolderPickerProps) {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [departmentId, setDepartmentId] = useState(ANY)
  const [roleId, setRoleId] = useState(ANY)

  const ordered = useMemo(
    () => holdersInOrder(holders, i18n.language),
    [holders, i18n.language]
  )
  const departments = useMemo(() => holderDepartments(holders), [holders])
  const roles = useMemo(() => holderRoles(holders), [holders])
  // A filter with one choice narrows nothing down and is left out: a team in
  // a single department gets none by department, a viewer who is told
  // nobody's role none by role.
  const byDepartment = departments.length > 1
  const byRole = roles.length > 1
  const matching = ordered.filter((holder) =>
    holderMatches(holder, { search, departmentId, roleId })
  )
  // The first hundred, and one more row for whoever the form is on when they
  // are further down: that row is where the list opens and what Enter picks
  // straight away, so it has to be there, tick and all.
  const beyond = matching.slice(MAX_ROWS).find((holder) => holder.id === value)
  const shown = beyond
    ? [...matching.slice(0, MAX_ROWS), beyond]
    : matching.slice(0, MAX_ROWS)
  const chosen = holders.find((holder) => holder.id === value)
  // The row Enter picks: the one the arrow keys or the mouse went to last.
  // Once a filter or the search has taken that row off the list it is the
  // first row, so that Enter always picks someone.
  const [reached, setReached] = useState('')
  const highlighted = shown.some((holder) => String(holder.id) === reached)
    ? reached
    : String(shown[0]?.id ?? '')

  // Every opening starts on whoever is chosen: Enter straight away then
  // changes nothing.
  const toggle = (next: boolean) => {
    if (next) setReached(String(value))
    setOpen(next)
  }

  const choose = (holderId: number) => {
    // The panel takes a moment to fade out and still has the keyboard while
    // it does. A second Enter in that moment must not pick again.
    if (!open) return
    onChange(holderId)
    setOpen(false)
    setSearch('')
  }

  return (
    <Popover open={open} onOpenChange={toggle}>
      <PopoverTrigger
        render={
          <button
            id={id}
            type='button'
            role='combobox'
            aria-expanded={open}
            // Drawn as the selects it sits among.
            className='border-input focus-visible:border-ring focus-visible:ring-ring/50 dark:bg-input/30 dark:hover:bg-input/50 flex h-8 w-full items-center justify-between gap-1.5 rounded-lg border bg-transparent py-2 pr-2 pl-2.5 text-sm whitespace-nowrap transition-colors outline-none select-none focus-visible:ring-3'
          />
        }
      >
        <span className={cn('truncate', !chosen && 'text-muted-foreground')}>
          {chosen ? keyHolderLabel(t, chosen) : (placeholder ?? '')}
        </span>
        <HugeiconsIcon
          icon={UnfoldMoreIcon}
          strokeWidth={2}
          className='text-muted-foreground pointer-events-none size-4 shrink-0'
        />
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-(--anchor-width) min-w-64 gap-0 p-0'
      >
        <Command
          shouldFilter={false}
          value={highlighted}
          onValueChange={setReached}
        >
          <CommandInput
            placeholder={t('Search by name')}
            value={search}
            onValueChange={setSearch}
          />
          {(byDepartment || byRole) && (
            <div
              className={cn(
                'grid gap-1 px-1 pt-1',
                byDepartment && byRole && 'grid-cols-2'
              )}
              // The list takes the arrow keys and Enter from anywhere in the
              // panel. Pressed on a filter they are the filter's; only Escape
              // goes on, to close the panel.
              onKeyDown={(event) => {
                if (event.key !== 'Escape') event.stopPropagation()
              }}
            >
              {byDepartment && (
                <PeopleFacetSelect
                  id={`${id}-department`}
                  label={t('Department')}
                  any={t('All departments')}
                  choices={departments}
                  value={departmentId}
                  onChange={setDepartmentId}
                />
              )}
              {byRole && (
                <PeopleFacetSelect
                  id={`${id}-role`}
                  label={t('Role')}
                  any={t('All roles')}
                  choices={roles.map((role) => ({
                    id: role.id,
                    name: orgRoleLabel(t, role.name),
                  }))}
                  value={roleId}
                  onChange={setRoleId}
                />
              )}
            </div>
          )}
          <CommandList>
            <CommandEmpty>{t('Nobody matches.')}</CommandEmpty>
            {shown.length > 0 && (
              <CommandGroup>
                {shown.map((holder) => (
                  <CommandItem
                    key={holder.id}
                    value={String(holder.id)}
                    data-checked={holder.id === value}
                    onSelect={() => choose(holder.id)}
                    // Enter picks the highlighted row, so it has to show. On
                    // the dark theme the list's own highlight is the colour
                    // of the panel; this is the one the selects use.
                    className='dark:data-[selected=true]:bg-accent'
                  >
                    <HolderRow holder={holder} />
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
          </CommandList>
          {matching.length > MAX_ROWS && (
            <p className='text-muted-foreground border-t px-3 py-2 text-xs'>
              {t(
                'Showing the first {{shown}} of {{total}}. Search or filter to narrow it down.',
                { shown: MAX_ROWS, total: matching.length }
              )}
            </p>
          )}
        </Command>
      </PopoverContent>
    </Popover>
  )
}

/**
 * One person in the list: the name, what makes them a special case, and under
 * it the department and — for a viewer who is told it — the role.
 */
function HolderRow({ holder }: { holder: OrgKeyHolder }) {
  const { t } = useTranslation()
  // The owner is marked as such for everyone, told the roles or not.
  const role = holder.is_owner ? 'owner' : holder.role
  const detail = [holder.department, role && orgRoleLabel(t, role)]
    .filter(Boolean)
    .join(' · ')

  return (
    <span className='grid min-w-0 flex-1 gap-0.5'>
      <span className='flex flex-wrap items-center gap-x-2 gap-y-1'>
        <span className='truncate font-medium'>{holder.name}</span>
        {holder.is_owner && (
          <Badge variant='secondary'>{t('Not handed out yet')}</Badge>
        )}
        {holder.is_service && (
          <Badge variant='outline'>
            <Bot aria-hidden='true' />
            {t('Service account')}
          </Badge>
        )}
      </span>
      <span className='text-muted-foreground truncate text-xs'>{detail}</span>
    </span>
  )
}
