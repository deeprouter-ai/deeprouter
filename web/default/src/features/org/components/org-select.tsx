// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type OrgSelectProps<Value extends number | string> = {
  /** The id the field's label points at. */
  id: string
  options: { value: Value; label: string }[]
  value: Value
  disabled?: boolean
  onChange: (value: Value) => void
}

/**
 * The dropdown of the organization dialogs: one role, department or scope out
 * of a short list. It is the themed select, not the browser's own — that one
 * opens a white list on the dark theme.
 */
export function OrgSelect<Value extends number | string>({
  id,
  options,
  value,
  disabled,
  onChange,
}: OrgSelectProps<Value>) {
  return (
    <Select
      items={options}
      value={value}
      disabled={disabled}
      onValueChange={(next) => next !== null && onChange(next)}
    >
      <SelectTrigger id={id} className='w-full'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
