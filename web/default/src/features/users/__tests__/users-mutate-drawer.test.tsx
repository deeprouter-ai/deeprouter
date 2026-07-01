import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { UsersMutateDrawer } from '../components/users-mutate-drawer'

const { mockCreateUser, mockGetGroups, mockUseUsers, mockToast } = vi.hoisted(
  () => ({
    mockCreateUser: vi.fn(),
    mockGetGroups: vi.fn(),
    mockUseUsers: vi.fn(),
    mockToast: {
      success: vi.fn(),
      error: vi.fn(),
    },
  })
)

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('sonner', () => ({
  toast: mockToast,
}))

vi.mock('../api', () => ({
  createUser: mockCreateUser,
  updateUser: vi.fn(),
  getUser: vi.fn(),
  getGroups: mockGetGroups,
}))

vi.mock('../components/users-provider', () => ({
  useUsers: () => mockUseUsers(),
}))

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

async function submitCreateUserForm(password?: string) {
  await userEvent.type(screen.getByLabelText('Username'), 'new-user')
  if (password) {
    await userEvent.type(screen.getByLabelText('Password'), password)
  }
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
}

describe('UsersMutateDrawer create validation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetGroups.mockResolvedValue({ success: true, data: ['default'] })
    mockUseUsers.mockReturnValue({
      triggerRefresh: vi.fn(),
    })
  })

  it.each([
    ['empty', undefined],
    ['short', 'short'],
  ])('blocks create requests when the password is %s', async (_label, password) => {
    renderWithQuery(
      <UsersMutateDrawer open onOpenChange={vi.fn()} currentRow={undefined} />
    )

    await submitCreateUserForm(password)

    expect(
      await screen.findByText('Password must be 8-20 characters')
    ).toBeInTheDocument()
    expect(mockCreateUser).not.toHaveBeenCalled()
  })

  it('submits create requests when the password satisfies backend length rules', async () => {
    const onOpenChange = vi.fn()
    const triggerRefresh = vi.fn()
    mockUseUsers.mockReturnValue({ triggerRefresh })
    mockCreateUser.mockResolvedValue({ success: true })

    renderWithQuery(
      <UsersMutateDrawer
        open
        onOpenChange={onOpenChange}
        currentRow={undefined}
      />
    )

    await submitCreateUserForm('password123')

    await waitFor(() => {
      expect(mockCreateUser).toHaveBeenCalledWith({
        username: 'new-user',
        display_name: 'new-user',
        password: 'password123',
        role: 1,
      })
    })
    expect(triggerRefresh).toHaveBeenCalledTimes(1)
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
