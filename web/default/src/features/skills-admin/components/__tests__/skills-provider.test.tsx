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
// Coverage: useSkills() throwing outside its provider (the guard every other
// component in this feature silently depends on), plus the state it hands
// out actually updating.
import { act, render, renderHook, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SkillsProvider, useSkills } from '../skills-provider'

describe('useSkills', () => {
  it('throws when called outside SkillsProvider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => renderHook(() => useSkills())).toThrow(
      'useSkills has to be used within <SkillsProvider>'
    )
    spy.mockRestore()
  })

  it('provides open/currentRow/refreshTrigger state that updates', () => {
    function Probe() {
      const { open, setOpen, refreshTrigger, triggerRefresh } = useSkills()
      return (
        <div>
          <span data-testid='open'>{open ?? 'null'}</span>
          <span data-testid='refresh'>{refreshTrigger}</span>
          <button onClick={() => setOpen('create')}>open-create</button>
          <button onClick={triggerRefresh}>refresh</button>
        </div>
      )
    }

    render(
      <SkillsProvider>
        <Probe />
      </SkillsProvider>
    )

    expect(screen.getByTestId('open')).toHaveTextContent('null')
    expect(screen.getByTestId('refresh')).toHaveTextContent('0')

    act(() => screen.getByText('open-create').click())
    expect(screen.getByTestId('open')).toHaveTextContent('create')

    act(() => screen.getByText('refresh').click())
    act(() => screen.getByText('refresh').click())
    expect(screen.getByTestId('refresh')).toHaveTextContent('2')
  })
})
