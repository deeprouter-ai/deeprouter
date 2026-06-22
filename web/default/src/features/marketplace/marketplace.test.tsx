import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Marketplace } from './index'

// useNavigate is captured so we can assert the card click navigates to detail.
const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// A skill whose BACKEND availability.cta is "upgrade" — before the P1 fix the
// card rendered an "Upgrade" button that merely opened the detail page.
vi.mock('./api', () => ({
  getMarketplaceSkills: vi.fn().mockResolvedValue({
    data: [
      {
        id: '1',
        slug: 'my-skill',
        name: 'My Skill',
        category: 'writing',
        short_description: 'desc',
        required_plan: 'pro',
        status: 'published',
        availability: { cta: 'upgrade', locked: true },
      },
    ],
  }),
}))

function renderMarketplace() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Marketplace />
    </QueryClientProvider>
  )
}

describe('Marketplace list card CTA (P1)', () => {
  it('shows "View" regardless of backend availability.cta, and never the raw action label', async () => {
    renderMarketplace()
    const cta = await screen.findByRole('button', { name: /View/ })
    expect(cta).not.toBeNull()
    // The misleading backend label must not be shown on the card.
    expect(screen.queryByText('Upgrade')).toBeNull()
  })

  it('navigates to the detail route when the card CTA is clicked', async () => {
    renderMarketplace()
    const cta = await screen.findByRole('button', { name: /View/ })
    fireEvent.click(cta)
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/skills/$slug',
      params: { slug: 'my-skill' },
    })
  })
})
