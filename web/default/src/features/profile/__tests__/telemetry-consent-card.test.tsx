import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TelemetryConsentCard } from '../components/telemetry-consent-card'
import type { UserProfile } from '../types'

const { mockUpdateTelemetryConsent, mockToast } = vi.hoisted(() => ({
  mockUpdateTelemetryConsent: vi.fn(),
  mockToast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, string>) =>
      values
        ? key.replace(/\{\{(\w+)\}\}/g, (_, name) => values[name] ?? '')
        : key,
  }),
}))

vi.mock('sonner', () => ({
  toast: mockToast,
}))

vi.mock('../api', () => ({
  updateTelemetryConsent: mockUpdateTelemetryConsent,
}))

const baseProfile: UserProfile = {
  id: 42,
  username: 'tao',
  display_name: 'Tao',
  role: 1,
  group: 'default',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  status: 1,
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 0,
}

describe('TelemetryConsentCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the current disabled consent state', () => {
    render(
      <TelemetryConsentCard
        profile={{ ...baseProfile, tier2_telemetry_consent: false }}
        loading={false}
        onProfileUpdate={vi.fn()}
      />
    )

    expect(screen.getByText('Privacy')).toBeInTheDocument()
    expect(screen.getByText('Tier 2 telemetry consent')).toBeInTheDocument()
    expect(screen.getByText('Disabled')).toBeInTheDocument()
    expect(
      screen.getByRole('switch', { name: 'Tier 2 telemetry consent' })
    ).not.toBeChecked()
  })

  it('enables consent and refreshes the profile', async () => {
    const onProfileUpdate = vi.fn()
    mockUpdateTelemetryConsent.mockResolvedValue({
      success: true,
      data: {
        tier2_telemetry_consent: true,
        tier2_telemetry_consented_at: '2026-07-01T00:00:00Z',
      },
    })

    render(
      <TelemetryConsentCard
        profile={{ ...baseProfile, tier2_telemetry_consent: false }}
        loading={false}
        onProfileUpdate={onProfileUpdate}
      />
    )

    await userEvent.click(
      screen.getByRole('switch', { name: 'Tier 2 telemetry consent' })
    )

    expect(mockUpdateTelemetryConsent).toHaveBeenCalledWith({
      tier2_telemetry_consent: true,
    })
    expect(onProfileUpdate).toHaveBeenCalledTimes(1)
    expect(mockToast.success).toHaveBeenCalledWith(
      'Tier 2 telemetry consent enabled'
    )
  })

  it('rolls back the switch when the update fails', async () => {
    mockUpdateTelemetryConsent.mockResolvedValue({
      success: false,
      message: 'failed',
    })

    render(
      <TelemetryConsentCard
        profile={{ ...baseProfile, tier2_telemetry_consent: true }}
        loading={false}
        onProfileUpdate={vi.fn()}
      />
    )

    const consentSwitch = screen.getByRole('switch', {
      name: 'Tier 2 telemetry consent',
    })
    expect(consentSwitch).toBeChecked()

    await userEvent.click(consentSwitch)

    expect(mockUpdateTelemetryConsent).toHaveBeenCalledWith({
      tier2_telemetry_consent: false,
    })
    expect(mockToast.error).toHaveBeenCalledWith(
      'Failed to update telemetry consent'
    )
    expect(consentSwitch).toBeChecked()
  })
})
