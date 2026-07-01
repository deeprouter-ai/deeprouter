/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: MetricCard component — loading / no-data / tracking-failed / normal states
import { render, screen } from '@testing-library/react'
import { Users } from 'lucide-react'
import { describe, it, expect, vi } from 'vitest'
import { MetricCard } from '../components/metric-card'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

describe('MetricCard', () => {
  const base = {
    title: 'Weekly Active Skill Users',
    value: '1,234' as string | null,
    description: 'Users who ran at least one skill call during the period',
    icon: Users,
  }

  it('renders title', () => {
    render(<MetricCard {...base} />)
    expect(screen.getByText('Weekly Active Skill Users')).toBeInTheDocument()
  })

  it('renders value and description in normal state', () => {
    render(<MetricCard {...base} />)
    expect(screen.getByText('1,234')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Users who ran at least one skill call during the period'
      )
    ).toBeInTheDocument()
  })

  it('shows skeleton elements when loading=true (no value rendered)', () => {
    const { container } = render(<MetricCard {...base} loading />)
    // Skeletons are divs with "animate-pulse" style — value text absent
    expect(screen.queryByText('1,234')).not.toBeInTheDocument()
    // at least one skeleton element present
    expect(
      container.querySelectorAll('[class*="skeleton"], [class*="animate"]')
        .length
    ).toBeGreaterThan(0)
  })

  it('shows "—" and "No data in this period" when value is null', () => {
    render(<MetricCard {...base} value={null} />)
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.getByText('No data in this period')).toBeInTheDocument()
  })

  it('shows "—" when trackingFailed=true regardless of value', () => {
    render(<MetricCard {...base} value='999' trackingFailed />)
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.queryByText('999')).not.toBeInTheDocument()
  })

  it('shows "Tracking unavailable" description when trackingFailed=true', () => {
    render(<MetricCard {...base} trackingFailed />)
    expect(screen.getByText('Tracking unavailable')).toBeInTheDocument()
  })

  it('trackingFailed overrides value=null — shows tracking desc not no-data desc', () => {
    render(<MetricCard {...base} value={null} trackingFailed />)
    expect(screen.getByText('Tracking unavailable')).toBeInTheDocument()
    expect(screen.queryByText('No data in this period')).not.toBeInTheDocument()
  })

  it('does not render a graph when no real progress value is provided', () => {
    render(<MetricCard {...base} />)
    expect(screen.queryByTestId('metric-card-progress')).not.toBeInTheDocument()
  })

  it('renders a real percentage progress meter when progressValue is provided', () => {
    const { container } = render(
      <MetricCard {...base} value='50.0%' progressValue={0.5} />
    )
    const meter = screen.getByTestId('metric-card-progress')
    expect(meter).toHaveAttribute('role', 'meter')
    expect(meter).toHaveAttribute('aria-valuenow', '50')
    expect(container.querySelector('linearGradient')).toBeInTheDocument()
    expect(container.querySelector('clipPath path')).toHaveAttribute(
      'd',
      'M8 50 A42 42 0 0 1 92 50 L8 50 Z'
    )
  })

  it('uses one provided color with light-to-deep opacity stops for percentage meters', () => {
    const { container } = render(
      <MetricCard
        {...base}
        value='50.0%'
        progressValue={0.5}
        progressColor='var(--chart-2)'
      />
    )

    const stops = container.querySelectorAll('stop')
    expect(stops[0]).toHaveAttribute('stop-color', 'var(--chart-2)')
    expect(stops[0]).toHaveAttribute('stop-opacity', '0.22')
    expect(stops[1]).toHaveAttribute('stop-color', 'var(--chart-2)')
    expect(stops[1]).toHaveAttribute('stop-opacity', '1')
  })

  it('renders count unit icons for count metrics', () => {
    render(
      <MetricCard {...base} value='7' countValue={7} countVariant='runs' />
    )

    expect(screen.getByTestId('metric-card-count-visual')).toBeInTheDocument()
    expect(screen.queryByTestId('metric-card-progress')).not.toBeInTheDocument()
  })
})
