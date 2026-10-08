// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { Label } from '@/components/ui/label'
import { ANY, type PeopleFacet } from '../lib/people-filter'
import { OrgSelect } from './org-select'

type PeopleFacetSelectProps = {
  id: string
  /** What the filter narrows down by. Read out by a screen reader, not shown. */
  label: string
  /** The words for leaving the filter open. */
  any: string
  choices: PeopleFacet[]
  value: number
  onChange: (id: number) => void
}

/**
 * One way to narrow a list of people down: left open, or set to one of its
 * choices.
 */
export function PeopleFacetSelect({
  id,
  label,
  any,
  choices,
  value,
  onChange,
}: PeopleFacetSelectProps) {
  return (
    <>
      <Label htmlFor={id} className='sr-only'>
        {label}
      </Label>
      <OrgSelect
        id={id}
        options={[
          { value: ANY, label: any },
          ...choices.map((choice) => ({
            value: choice.id,
            label: choice.name,
          })),
        ]}
        value={value}
        onChange={onChange}
      />
    </>
  )
}
