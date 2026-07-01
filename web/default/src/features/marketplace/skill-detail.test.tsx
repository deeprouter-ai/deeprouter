import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  DownloadSkillError,
  downloadSkillPackage,
  getMarketplaceSkill,
} from './api'
import { SkillDetail } from './skill-detail'
import type { PublicSkillDetail } from './types'

// Navigation + auth.reset are captured so we can assert the AUTH_REQUIRED path
// signs out + redirects (and never the other error branches).
const {
  navigateMock,
  resetMock,
  mockGetTelemetryConsent,
  mockUpdateTelemetryConsent,
} = vi.hoisted(() => ({
  navigateMock: vi.fn(),
  resetMock: vi.fn(),
  mockGetTelemetryConsent: vi.fn(),
  mockUpdateTelemetryConsent: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  // SkillDetail calls useRouterState({ select: (s) => s.location.href }).
  useRouterState: (opts: { select: (s: unknown) => unknown }) =>
    opts.select({ location: { href: '/skills/my-skill' } }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('@/stores/auth-store', () => ({
  // Component only uses useAuthStore.getState().auth.reset().
  useAuthStore: { getState: () => ({ auth: { reset: resetMock } }) },
}))

vi.mock('@/features/profile/api', () => ({
  getTelemetryConsent: mockGetTelemetryConsent,
  updateTelemetryConsent: mockUpdateTelemetryConsent,
}))

// Define DownloadSkillError inside the mock so `error instanceof DownloadSkillError`
// in the component matches the errors the mocked downloadSkillPackage throws.
vi.mock('./api', () => {
  class DownloadSkillError extends Error {
    code: string
    constructor(code: string) {
      super(code)
      this.name = 'DownloadSkillError'
      this.code = code
    }
  }
  return {
    DownloadSkillError,
    getMarketplaceSkill: vi.fn(),
    downloadSkillPackage: vi.fn(),
    purchaseSkill: vi.fn(),
    recordMarketplaceSkillEvent: vi.fn().mockResolvedValue(undefined),
  }
})

const detail: PublicSkillDetail = {
  id: '1',
  slug: 'my-skill',
  name: 'My Skill',
  category: 'writing',
  short_description: 'short',
  description: 'A test skill',
  required_plan: 'free',
  status: 'published',
  is_kids_safe: false,
  is_kids_exclusive: false,
  ai_disclosure_required: false,
  requires_deeprouter_key: true,
  download_cta: {
    url: '/api/v1/marketplace/skills/my-skill/download',
    method: 'GET',
  },
  instructions: {
    download_instructions: 'Extract the zip to .claude/skills/.',
    usage_instructions: 'Run the Skill from your local assistant.',
  },
}

function renderDetail() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <SkillDetail slug='my-skill' />
    </QueryClientProvider>
  )
}

async function findDownloadButton() {
  return screen.findByRole('button', { name: /Download/ })
}

beforeEach(() => {
  vi.clearAllMocks()
  window.localStorage.clear()
  vi.mocked(getMarketplaceSkill).mockResolvedValue(detail)
  vi.mocked(downloadSkillPackage).mockResolvedValue(undefined)
  mockGetTelemetryConsent.mockResolvedValue({
    success: true,
    data: { tier2_telemetry_consent: true },
  })
  mockUpdateTelemetryConsent.mockResolvedValue({
    success: true,
    data: { tier2_telemetry_consent: true },
  })
})

describe('SkillDetail page', () => {
  it('renders the runtime-key copy and a Download CTA, and no Enable toggle (A1/A2)', async () => {
    renderDetail()
    // A1: static runtime-dependency copy.
    expect(
      await screen.findByText(
        'Running this Skill requires a DeepRouter API key; it routes its work through DeepRouter.'
      )
    ).not.toBeNull()
    // A2: a Download CTA, and NOT an Enable/Disable toggle.
    expect(await findDownloadButton()).not.toBeNull()
    expect(screen.queryByRole('button', { name: /Enable/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Disable/ })).toBeNull()
  })

  it('shows an ErrorBanner when the detail fails to load', async () => {
    vi.mocked(getMarketplaceSkill).mockRejectedValue(new Error('boom'))
    renderDetail()
    expect(await screen.findByText('boom')).not.toBeNull()
    expect(screen.queryByRole('button', { name: /^Download$/ })).toBeNull()
  })

  it('download happy path shows a success toast', async () => {
    renderDetail()
    fireEvent.click(await findDownloadButton())
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(
        'Download started. Extract the zip to .claude/skills/ to use it.'
      )
    )
    // Download must go through the backend-provided download_cta.url + slug (A2), exactly once.
    expect(downloadSkillPackage).toHaveBeenCalledWith(
      '/api/v1/marketplace/skills/my-skill/download',
      'my-skill'
    )
    expect(downloadSkillPackage).toHaveBeenCalledTimes(1)
    expect(navigateMock).not.toHaveBeenCalled()
    expect(resetMock).not.toHaveBeenCalled()
  })

  it('prompts to enable telemetry consent before the first download', async () => {
    mockGetTelemetryConsent.mockResolvedValue({
      success: true,
      data: { tier2_telemetry_consent: false },
    })

    renderDetail()
    await userEvent.click(await findDownloadButton())

    expect(
      await screen.findByText('Enable Skill and Runner usage details?')
    ).toBeInTheDocument()
    expect(downloadSkillPackage).not.toHaveBeenCalled()

    await userEvent.click(
      screen.getByRole('button', { name: 'Enable and download' })
    )

    await waitFor(() =>
      expect(mockUpdateTelemetryConsent).toHaveBeenCalledWith({
        tier2_telemetry_consent: true,
      })
    )
    await waitFor(() => expect(downloadSkillPackage).toHaveBeenCalledTimes(1))
  })

  it('allows downloading without enabling telemetry consent', async () => {
    mockGetTelemetryConsent.mockResolvedValue({
      success: true,
      data: { tier2_telemetry_consent: false },
    })

    renderDetail()
    await userEvent.click(await findDownloadButton())
    await userEvent.click(
      await screen.findByRole('button', {
        name: 'Download without usage details',
      })
    )

    expect(mockUpdateTelemetryConsent).not.toHaveBeenCalled()
    await waitFor(() => expect(downloadSkillPackage).toHaveBeenCalledTimes(1))
  })

  it('AUTH_REQUIRED signs out and redirects to /sign-in (not add-key)', async () => {
    vi.mocked(downloadSkillPackage).mockRejectedValue(
      new DownloadSkillError('AUTH_REQUIRED')
    )
    renderDetail()
    fireEvent.click(await findDownloadButton())
    await waitFor(() => expect(resetMock).toHaveBeenCalledTimes(1))
    expect(toast.error).toHaveBeenCalledWith(
      'Your session has expired. Please sign in again.'
    )
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/sign-in',
      search: { redirect: '/skills/my-skill' },
    })
    // Must NOT surface a plan / unavailable / generic inline error for an auth failure.
    expect(
      screen.queryByText(
        'This Skill requires a higher plan. Upgrade to download it.'
      )
    ).toBeNull()
    expect(
      screen.queryByText('Download is unavailable for this Skill right now.')
    ).toBeNull()
    expect(screen.queryByText('Download failed. Please try again.')).toBeNull()
  })

  it('SKILL_AUTH_REQUIRED is treated the same as AUTH_REQUIRED', async () => {
    vi.mocked(downloadSkillPackage).mockRejectedValue(
      new DownloadSkillError('SKILL_AUTH_REQUIRED')
    )
    renderDetail()
    fireEvent.click(await findDownloadButton())
    await waitFor(() => expect(resetMock).toHaveBeenCalledTimes(1))
    expect(toast.error).toHaveBeenCalledWith(
      'Your session has expired. Please sign in again.'
    )
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/sign-in',
      search: { redirect: '/skills/my-skill' },
    })
    // Symmetric with AUTH_REQUIRED: no plan / unavailable / generic inline copy.
    expect(
      screen.queryByText(
        'This Skill requires a higher plan. Upgrade to download it.'
      )
    ).toBeNull()
    expect(
      screen.queryByText('Download is unavailable for this Skill right now.')
    ).toBeNull()
    expect(screen.queryByText('Download failed. Please try again.')).toBeNull()
  })

  it('SKILL_PLAN_REQUIRED opens the paywall and does not navigate', async () => {
    vi.mocked(downloadSkillPackage).mockRejectedValue(
      new DownloadSkillError('SKILL_PLAN_REQUIRED')
    )
    renderDetail()
    fireEvent.click(await findDownloadButton())
    expect(await screen.findByText('Paywall')).not.toBeNull()
    expect(navigateMock).not.toHaveBeenCalled()
    expect(resetMock).not.toHaveBeenCalled()
  })

  it('DOWNLOAD_UNAVAILABLE shows the unavailable copy', async () => {
    vi.mocked(downloadSkillPackage).mockRejectedValue(
      new DownloadSkillError('DOWNLOAD_UNAVAILABLE')
    )
    renderDetail()
    fireEvent.click(await findDownloadButton())
    expect(
      await screen.findByText(
        'Download is unavailable for this Skill right now.'
      )
    ).not.toBeNull()
    expect(navigateMock).not.toHaveBeenCalled()
    expect(resetMock).not.toHaveBeenCalled()
  })

  it('an unknown / non-DownloadSkillError failure maps to the generic message', async () => {
    vi.mocked(downloadSkillPackage).mockRejectedValue(new Error('network'))
    renderDetail()
    fireEvent.click(await findDownloadButton())
    expect(
      await screen.findByText('Download failed. Please try again.')
    ).not.toBeNull()
    expect(navigateMock).not.toHaveBeenCalled()
    expect(resetMock).not.toHaveBeenCalled()
  })
})
