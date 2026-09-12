/*
Copyright (C) 2026 DeepRouter

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
*/
// Coverage: PRD §13 — a reference listing's detail page swaps the whole
// Download/Buy action for an external-link button, and drops the parts of
// the page (Price badge, "needs a DR key" notice) that only make sense for
// a packaged skill DR actually runs.
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SkillDetailPage } from '../skill-detail'
import type { MarketplaceSkill, MarketplaceSkillDetail } from '../types'

const {
  mockFetchMarketplaceSkill,
  mockFetchMySkills,
  mockFetchMyPurchases,
  mockDownload,
  mockNavigate,
} = vi.hoisted(() => ({
  mockFetchMarketplaceSkill: vi.fn(),
  mockFetchMySkills: vi.fn(),
  mockFetchMyPurchases: vi.fn(),
  mockDownload: vi.fn(),
  mockNavigate: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) =>
      opts ? key.replace(/\{\{(\w+)\}\}/g, (_, k) => String(opts[k])) : key,
  }),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
  useNavigate: () => mockNavigate,
  // Real useRouterState(select?) either returns the whole state or
  // runs `select` over it — PublicHeader (inside PublicLayout) calls it
  // with no select at all, this page calls it with one.
  useRouterState: (opts?: { select?: (s: unknown) => unknown }) => {
    const state = {
      location: {
        href: '/marketplace/test-skill',
        pathname: '/marketplace/test-skill',
      },
    }
    return opts?.select ? opts.select(state) : state
  },
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: () => ({ auth: { user: null } }),
}))

// PublicLayout/Footer carry their own header chrome (language switcher,
// notifications, …) with their own hook dependencies fully covered
// elsewhere — stubbing them isolates this page's own branching, the same
// way skill-edit-page.test.tsx stubs its children.
vi.mock('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}))
vi.mock('@/components/layout/components/footer', () => ({
  Footer: () => <div data-testid='footer' />,
}))

vi.mock('../api', () => ({
  fetchMarketplaceSkill: mockFetchMarketplaceSkill,
  fetchMySkills: mockFetchMySkills,
  fetchMyPurchases: mockFetchMyPurchases,
  marketplaceQueryKeys: {
    detail: (slug: string) => ['marketplace', 'detail', slug],
    mySkills: () => ['marketplace', 'my-skills'],
    myPurchases: () => ['marketplace', 'my-purchases'],
  },
}))

vi.mock('../hooks/use-skill-download', () => ({
  useSkillDownload: () => ({ download: mockDownload, pendingSlug: null }),
}))

function makeSkill(
  overrides: Partial<MarketplaceSkillDetail>
): MarketplaceSkillDetail {
  const base: MarketplaceSkill = {
    id: 1,
    slug: 'test-skill',
    name: 'Test Skill',
    description: 'Does things.',
    category: 'video',
    tags: [],
    status: 'published',
    monetization_type: 'free',
    price_usd: 0,
    featured_flag: false,
    featured_rank: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    version: '',
    listing_type: 'hosted',
  }
  return { ...base, changelog: '', ...overrides }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <SkillDetailPage slug='test-skill' />
    </QueryClientProvider>
  )
}

describe('SkillDetailPage — reference listings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockFetchMySkills.mockResolvedValue({
      skills: [],
      total: 0,
      page: 1,
      limit: 100,
    })
    mockFetchMyPurchases.mockResolvedValue({
      purchases: [],
      total: 0,
      page: 1,
      limit: 100,
    })
  })

  it('renders "View on GitHub" linking to source_url instead of a Download button', async () => {
    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({
        listing_type: 'reference',
        source_url: 'https://github.com/owner/repo',
      })
    )
    renderPage()

    const link = await screen.findByRole('button', { name: /View on GitHub/i })
    expect(link).toHaveAttribute('href', 'https://github.com/owner/repo')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    expect(
      screen.queryByRole('button', { name: /Download|Sign in|Buy/i })
    ).not.toBeInTheDocument()
  })

  it('does not show the Price badge for a reference listing', async () => {
    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({
        listing_type: 'reference',
        source_url: 'https://github.com/owner/repo',
      })
    )
    renderPage()

    await screen.findByRole('button', { name: /View on GitHub/i })
    expect(screen.queryByText('Free')).not.toBeInTheDocument()
  })

  it('does not show the "needs a DeepRouter API Key" notice for a reference listing', async () => {
    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({
        listing_type: 'reference',
        source_url: 'https://github.com/owner/repo',
      })
    )
    renderPage()

    await screen.findByRole('button', { name: /View on GitHub/i })
    expect(
      screen.queryByText(/needs a DeepRouter API Key/i)
    ).not.toBeInTheDocument()
  })

  it('does not require sign-in to see or use the GitHub link', async () => {
    // useAuthStore is mocked to an anonymous user throughout this suite —
    // a hosted skill would redirect anonymous users to sign-in instead of
    // rendering a working action at all.
    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({
        listing_type: 'reference',
        source_url: 'https://github.com/owner/repo',
      })
    )
    renderPage()

    const link = await screen.findByRole('button', { name: /View on GitHub/i })
    expect(link).toHaveAttribute('href', 'https://github.com/owner/repo')
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('copies source_url to the clipboard when the copy button is clicked', async () => {
    // jsdom has no real Clipboard API — stub the one method
    // lib/copy-to-clipboard.ts calls, so this actually exercises the button's
    // onClick -> useCopyToClipboard -> navigator.clipboard.writeText chain
    // instead of only checking the button renders.
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })

    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({
        listing_type: 'reference',
        source_url: 'https://github.com/owner/repo',
      })
    )
    renderPage()

    await screen.findByRole('button', { name: /View on GitHub/i })
    const copyButton = screen.getByRole('button', { name: /Copy Link/i })
    await userEvent.click(copyButton)

    expect(writeText).toHaveBeenCalledWith('https://github.com/owner/repo')
  })

  it('still shows the Download button, Price badge, and API-key notice for a hosted skill', async () => {
    mockFetchMarketplaceSkill.mockResolvedValue(
      makeSkill({ listing_type: 'hosted', version: '1.0.0' })
    )
    renderPage()

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: /Sign in to download/i })
      ).toBeInTheDocument()
    )
    expect(screen.getByText('Free')).toBeInTheDocument()
    expect(screen.getByText(/needs a DeepRouter API Key/i)).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /View on GitHub/i })
    ).not.toBeInTheDocument()
  })
})
