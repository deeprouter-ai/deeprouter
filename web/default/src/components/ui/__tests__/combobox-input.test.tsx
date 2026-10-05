/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ComboboxInput } from '../combobox-input'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// Mirrors the channel-type field: numeric values, allowCustomValue, and a
// selected value ("1") whose digit appears in some other ids but not all —
// the exact shape that made the open-time pre-filter look like missing rows.
const OPTIONS = [
  { value: '1', label: 'OpenAI' },
  { value: '21', label: 'AI Proxy Library' },
  { value: '35', label: 'MiniMax' },
  { value: '24', label: 'Gemini' },
]

function Harness({
  initial = '1',
  allowCustomValue = true,
  onValueChange,
}: {
  initial?: string
  allowCustomValue?: boolean
  onValueChange?: (v: string) => void
}) {
  const [value, setValue] = useState(initial)
  return (
    <ComboboxInput
      options={OPTIONS}
      value={value}
      onValueChange={(v) => {
        setValue(v)
        onValueChange?.(v)
      }}
      placeholder='pick one'
      allowCustomValue={allowCustomValue}
    />
  )
}

describe('ComboboxInput', () => {
  it('shows ALL options on focus, not just those matching the current value', async () => {
    // Regression (2026-10-04): the seeded value "1" acted as a filter, so the
    // list silently shrank to ids containing "1" and MiniMax (35) vanished.
    render(<Harness />)

    await userEvent.click(screen.getByRole('combobox'))

    expect(await screen.findByText('MiniMax')).toBeInTheDocument()
    expect(screen.getByText('Gemini')).toBeInTheDocument()
    expect(screen.getAllByRole('option')).toHaveLength(OPTIONS.length)
  })

  it('displays the selected option label when closed, not the raw value', () => {
    render(<Harness />)
    expect(screen.getByRole('combobox')).toHaveValue('OpenAI')
  })

  it('filters once the user types, and selecting writes the option value back', async () => {
    const onValueChange = vi.fn()
    render(<Harness onValueChange={onValueChange} />)

    const input = screen.getByRole('combobox')
    await userEvent.click(input)
    // The seeded text is selected on focus, so typing replaces it. ("max"
    // rather than "mini" — "mini" also matches Gemini.) The select-all runs
    // in a requestAnimationFrame after focus; wait for it, or under load the
    // frame lands after the first keystroke, selects the "m" and the next key
    // replaces it ("ax" → two matches).
    await waitFor(() => {
      const el = input as HTMLInputElement
      expect(el.selectionStart).toBe(0)
      expect(el.selectionEnd).toBe(el.value.length)
    })
    await userEvent.keyboard('max')

    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(1)
    expect(options[0]).toHaveTextContent('MiniMax')

    await userEvent.click(options[0])
    expect(onValueChange).toHaveBeenLastCalledWith('35')
    // Closed again: the input shows the new selection's label.
    expect(input).toHaveValue('MiniMax')
  })

  it('keeps free-text ergonomics: the current value stays editable in place', async () => {
    render(<Harness />)

    const input = screen.getByRole('combobox')
    await userEvent.click(input)

    // Seeded with the raw value for in-place editing (not the label) — and
    // the full list stays visible until a key is typed.
    expect(input).toHaveValue('1')
    expect(screen.getAllByRole('option')).toHaveLength(OPTIONS.length)
  })

  it('without allowCustomValue the search starts empty and the list is full', async () => {
    render(<Harness initial='35' allowCustomValue={false} />)

    const input = screen.getByRole('combobox')
    expect(input).toHaveValue('MiniMax')

    await userEvent.click(input)
    expect(input).toHaveValue('')
    expect(screen.getAllByRole('option')).toHaveLength(OPTIONS.length)
  })
})
