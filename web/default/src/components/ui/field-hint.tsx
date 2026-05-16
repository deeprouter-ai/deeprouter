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
import { HelpCircle } from 'lucide-react'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useIsCasual } from '@/hooks/use-casual'
import { cn } from '@/lib/utils'

type FieldHintProps = {
  children: React.ReactNode
  /**
   * Force expanded (inline visible) regardless of persona. Use for hints
   * critical for every user (security warnings, "this is destructive").
   */
  forceExpanded?: boolean
  /**
   * Force collapsed (popover only) regardless of persona. Use when the
   * hint is only useful for advanced users digging into details.
   */
  forceCollapsed?: boolean
  /** Override the container className when expanded. */
  className?: string
  /**
   * Tone — visual color of the inline hint. `info` is the default soft
   * gray; `warning` (amber) is for things like "don't share this key".
   */
  tone?: 'info' | 'warning'
}

/**
 * Casual-aware contextual hint next to a config field.
 *
 * - Casual persona: renders as an always-visible inline line (💡 prefix)
 *   immediately below the field. Density-on-by-default for non-tech
 *   users who need hand-holding.
 * - Dev/team/admin: renders as a `?` icon that opens a popover on
 *   hover/click. Density-off-by-default for power users who don't need
 *   the explanation cluttering their screen.
 *
 * See docs/tasks/casual-ux-prd.md §2.1 for the design rationale.
 */
export function FieldHint({
  children,
  forceExpanded,
  forceCollapsed,
  className,
  tone = 'info',
}: FieldHintProps) {
  const casual = useIsCasual()
  const expanded = forceExpanded || (casual && !forceCollapsed)

  const toneClass =
    tone === 'warning'
      ? 'text-amber-700 dark:text-amber-400'
      : 'text-muted-foreground'

  if (expanded) {
    return (
      <p
        className={cn(
          'mt-1 text-xs leading-snug',
          toneClass,
          className
        )}
      >
        <span className='mr-1' aria-hidden>
          💡
        </span>
        {children}
      </p>
    )
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            className='text-muted-foreground/60 hover:text-foreground inline-flex h-4 w-4 items-center justify-center rounded-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30'
            aria-label='Help'
          />
        }
      >
        <HelpCircle className='h-3.5 w-3.5' />
      </PopoverTrigger>
      <PopoverContent
        className={cn('max-w-xs text-xs leading-snug', toneClass)}
      >
        <span className='mr-1' aria-hidden>
          💡
        </span>
        {children}
      </PopoverContent>
    </Popover>
  )
}
