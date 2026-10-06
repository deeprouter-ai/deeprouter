// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { OrgMembersPage } from '../index'
import type {
  OrgDepartment,
  OrgInvite,
  OrgMember,
  OrgMembership,
  OrgRole,
} from '../types'

// Enterprise Org P3 (meta-repo docs/enterprise-org-prd.md): the "Members &
// departments" page. The backend decides what is allowed; these tests pin
// what the page offers and what it asks the backend for.

const api = vi.hoisted(() => ({
  fetchOrgMembership: vi.fn(),
  fetchOrgMembers: vi.fn(),
  fetchOrgDepartments: vi.fn(),
  fetchOrgRoles: vi.fn(),
  fetchOrgInvites: vi.fn(),
  createOrgDepartment: vi.fn(),
  renameOrgDepartment: vi.fn(),
  deleteOrgDepartment: vi.fn(),
  updateOrgMember: vi.fn(),
  createOrgServiceAccount: vi.fn(),
  createOrgInvite: vi.fn(),
  revokeOrgInvite: vi.fn(),
}))
const mockToast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  ...api,
}))
vi.mock('sonner', () => ({ toast: mockToast }))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: { id: 1 } } }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) =>
      key.replace(/{{(\w+)}}/g, (_, name: string) =>
        String(values?.[name] ?? '')
      ),
  }),
}))
// The layout pulls in the whole console shell; the page only needs its slots.
vi.mock('@/components/layout', () => {
  const Slot = ({ children }: { children?: ReactNode }) => <>{children}</>
  const Layout = ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  )
  return {
    SectionPageLayout: Object.assign(Layout, {
      Title: Slot,
      Actions: Slot,
      Content: Slot,
    }),
  }
})

const roles: OrgRole[] = [
  { id: 1, name: 'owner', scope: 'org', permissions: [], is_preset: true },
  { id: 2, name: 'admin', scope: 'org', permissions: [], is_preset: true },
  { id: 3, name: 'manager', scope: 'dept', permissions: [], is_preset: true },
  { id: 4, name: 'staff', scope: 'self', permissions: [], is_preset: true },
  { id: 5, name: 'readonly', scope: 'org', permissions: [], is_preset: true },
]
const departments: OrgDepartment[] = [
  { id: 10, name: 'General', is_default: true, member_count: 3 },
  { id: 11, name: 'Sales', is_default: false, member_count: 1 },
]
const members: OrgMember[] = [
  {
    id: 1,
    username: 'founder',
    display_name: 'Fiona Founder',
    email: 'fiona@acme.test',
    role_id: 1,
    role: 'owner',
    department_id: 10,
    is_owner: true,
    is_service: false,
  },
  {
    id: 2,
    username: 'adam',
    display_name: '',
    email: '',
    role_id: 2,
    role: 'admin',
    department_id: 10,
    is_owner: false,
    is_service: false,
  },
  {
    id: 3,
    username: 'sally',
    display_name: '',
    email: '',
    role_id: 4,
    role: 'staff',
    department_id: 11,
    is_owner: false,
    is_service: false,
  },
  {
    id: 4,
    username: 'svc-abc123def456',
    display_name: 'CI Pipeline',
    email: '',
    role_id: 4,
    role: 'staff',
    department_id: 10,
    is_owner: false,
    is_service: true,
  },
]
const invites: OrgInvite[] = [
  {
    id: 7,
    code: 'CODE7',
    role_id: 4,
    role: 'staff',
    department_id: 11,
    expires_time: 1791000000,
  },
]

const ownerMembership: OrgMembership = {
  org_id: 1,
  org_name: 'Acme',
  is_owner: true,
  is_admin: false,
  role: 'owner',
  role_scope: 'org',
  permissions: [],
  department_id: 10,
}
const adminMembership: OrgMembership = {
  ...ownerMembership,
  is_owner: false,
  is_admin: true,
  role: 'admin',
}

/** Renders the page as the given member and waits for its lists. */
async function renderPage(membership: OrgMembership = ownerMembership) {
  api.fetchOrgMembership.mockResolvedValue(membership)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <OrgMembersPage />
    </QueryClientProvider>
  )
  await screen.findByRole('tab', { name: /Members/ })
}

/** The table row that mentions the given text. */
function rowOf(text: string): HTMLElement {
  const row = screen.getByText(text).closest('tr')
  if (!row) throw new Error(`no table row for ${text}`)
  return row
}

/** The dropdown of the field with the given label. */
function dropdown(label: string): HTMLElement {
  return screen.getByRole('combobox', { name: label })
}

// The list of a dropdown appears a moment after the press that opens it, so
// both helpers wait for it instead of looking straight away.

/** Opens a dropdown, reads the choices it offers, and closes it again. */
async function optionsOf(label: string): Promise<string[]> {
  await userEvent.click(dropdown(label))
  const options = await screen.findAllByRole('option')
  const labels = options.map((option) => option.textContent ?? '')
  await userEvent.keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('option')).toBeNull())
  return labels
}

/** Picks one choice of a dropdown, the way a person would. */
async function choose(label: string, option: string) {
  await userEvent.click(dropdown(label))
  await userEvent.click(await screen.findByRole('option', { name: option }))
  await waitFor(() => expect(screen.queryByRole('option')).toBeNull())
}

/** The sentence under a department that cannot be chosen. */
const PINNED = 'The owner and admins always belong to “General”.'

beforeEach(() => {
  vi.clearAllMocks()
  api.fetchOrgMembers.mockResolvedValue(members)
  api.fetchOrgDepartments.mockResolvedValue(departments)
  api.fetchOrgRoles.mockResolvedValue(roles)
  api.fetchOrgInvites.mockResolvedValue(invites)
  for (const call of [
    api.createOrgDepartment,
    api.renameOrgDepartment,
    api.deleteOrgDepartment,
    api.updateOrgMember,
    api.createOrgServiceAccount,
    api.revokeOrgInvite,
  ]) {
    call.mockResolvedValue({ success: true })
  }
  api.createOrgInvite.mockResolvedValue({
    success: true,
    data: { ...invites[0], id: 8, code: 'NEWCODE' },
  })
})

describe('members', () => {
  it('lists everyone with their role and department', async () => {
    await renderPage()

    expect(screen.getByRole('tab', { name: /Members/ })).toHaveTextContent('4')
    expect(screen.getByText('Acme')).toBeInTheDocument()

    const founder = rowOf('Fiona Founder')
    expect(founder).toHaveTextContent('Owner')
    expect(founder).toHaveTextContent('founder · fiona@acme.test')
    expect(founder).toHaveTextContent('General')
    expect(rowOf('adam')).toHaveTextContent('Admin')
    expect(rowOf('sally')).toHaveTextContent('Staff')
    expect(rowOf('sally')).toHaveTextContent('Sales')
    // A service account is told apart from people, and shows the generated
    // username the company did not choose.
    const bot = rowOf('CI Pipeline')
    expect(bot).toHaveTextContent('Service account')
    expect(bot).toHaveTextContent('svc-abc123def456')
  })

  it('lets the owner appoint an admin, sending only what changed', async () => {
    await renderPage(ownerMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))

    // The owner role is never on offer; the admin role is, to the owner.
    expect(await optionsOf('Role')).toEqual([
      'Admin',
      'Manager',
      'Staff',
      'Read-only',
    ])
    expect(dropdown('Department')).toHaveTextContent('Sales')
    expect(screen.queryByText(PINNED)).toBeNull()

    // PRD D26: an admin sits in the default department. The form shows where
    // sally will be and leaves the moving to the backend.
    await choose('Role', 'Admin')
    expect(dropdown('Department')).toBeDisabled()
    expect(dropdown('Department')).toHaveTextContent('General')
    expect(screen.getByText(PINNED)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(3, { role_id: 2 })
    )
    expect(mockToast.success).toHaveBeenCalledWith('Member updated')
    // The list is fetched again, so the page shows what the backend now has.
    await waitFor(() => expect(api.fetchOrgMembers).toHaveBeenCalledTimes(2))
  })

  it('leaves the department of a new admin to the backend, whatever was picked first', async () => {
    await renderPage(ownerMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))

    await choose('Department', 'General')
    await choose('Role', 'Admin')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(3, { role_id: 2 })
    )
  })

  it('does not offer the admin role to an admin', async () => {
    await renderPage(adminMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))

    expect(await optionsOf('Role')).toEqual(['Manager', 'Staff', 'Read-only'])
  })

  it('moves an ordinary member to another department', async () => {
    await renderPage(adminMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))

    expect(await optionsOf('Department')).toEqual(['General', 'Sales'])
    await choose('Department', 'General')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(3, { department_id: 10 })
    )
  })

  it('keeps an admin from dismissing or moving another admin', async () => {
    await renderPage(adminMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit adam' }))

    expect(dropdown('Role')).toBeDisabled()
    expect(dropdown('Role')).toHaveTextContent('Admin')
    expect(
      screen.getByText('Only the owner can appoint or dismiss admins.')
    ).toBeInTheDocument()
    expect(dropdown('Department')).toBeDisabled()
    expect(dropdown('Department')).toHaveTextContent('General')
    expect(screen.getByText(PINNED)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByText('Edit member')).not.toBeInTheDocument()
    )
    expect(api.updateOrgMember).not.toHaveBeenCalled()
  })

  it('lets the owner dismiss an admin and place them in the same save', async () => {
    await renderPage(ownerMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Edit adam' }))
    expect(dropdown('Department')).toBeDisabled()

    // No longer an admin, so the department is theirs to choose again.
    await choose('Role', 'Staff')
    expect(dropdown('Department')).toBeEnabled()
    expect(screen.queryByText(PINNED)).toBeNull()
    await choose('Department', 'Sales')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(2, {
        role_id: 4,
        department_id: 11,
      })
    )
  })

  it("locks the owner's role and department, even for the owner", async () => {
    await renderPage(ownerMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Fiona Founder' })
    )

    expect(dropdown('Role')).toBeDisabled()
    expect(dropdown('Role')).toHaveTextContent('Owner')
    expect(
      screen.getByText("The owner's role cannot be changed.")
    ).toBeInTheDocument()
    expect(dropdown('Department')).toBeDisabled()
    expect(dropdown('Department')).toHaveTextContent('General')
    expect(screen.getByText(PINNED)).toBeInTheDocument()
  })

  it("locks a service account's role, not its department", async () => {
    await renderPage(ownerMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit CI Pipeline' })
    )

    expect(dropdown('Role')).toBeDisabled()
    expect(
      screen.getByText('Service accounts always have the Staff role.')
    ).toBeInTheDocument()
    expect(dropdown('Department')).toBeEnabled()
    expect(screen.queryByText(PINNED)).toBeNull()
  })

  it('saves nothing when nothing changed', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(screen.queryByText('Edit member')).not.toBeInTheDocument()
    )
    expect(api.updateOrgMember).not.toHaveBeenCalled()
  })
})

describe('service accounts', () => {
  it('adds one to the default department unless told otherwise', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Add service account' })
    )

    const add = screen.getByRole('button', { name: 'Add' })
    expect(add).toBeDisabled() // a name is required
    await userEvent.type(screen.getByLabelText('Name'), '  CI Pipeline  ')
    await userEvent.click(add)

    await waitFor(() =>
      expect(api.createOrgServiceAccount).toHaveBeenCalledWith({
        name: 'CI Pipeline',
        department_id: 10,
      })
    )
    expect(mockToast.success).toHaveBeenCalledWith('Service account added')
  })

  it('adds one to the department that was chosen', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Add service account' })
    )
    await userEvent.type(screen.getByLabelText('Name'), 'Deploy bot')
    await choose('Department', 'Sales')
    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(api.createOrgServiceAccount).toHaveBeenCalledWith({
        name: 'Deploy bot',
        department_id: 11,
      })
    )
  })
})

describe('departments', () => {
  async function openDepartments() {
    await renderPage()
    await userEvent.click(screen.getByRole('tab', { name: /Departments/ }))
  }

  it('lists them with head counts and never offers to delete the default one', async () => {
    await openDepartments()

    const general = rowOf('General')
    expect(general).toHaveTextContent('Default')
    expect(general).toHaveTextContent('3')
    expect(
      within(general).getByRole('button', { name: 'Rename General' })
    ).toBeInTheDocument()
    expect(
      within(general).queryByRole('button', { name: 'Delete General' })
    ).toBeNull()
    expect(
      within(rowOf('Sales')).getByRole('button', { name: 'Delete Sales' })
    ).toBeInTheDocument()
  })

  it('creates a department', async () => {
    await openDepartments()
    await userEvent.click(
      screen.getByRole('button', { name: 'New department' })
    )
    await userEvent.type(screen.getByLabelText('Department name'), ' Research ')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.createOrgDepartment).toHaveBeenCalledWith('Research')
    )
    expect(api.renameOrgDepartment).not.toHaveBeenCalled()
  })

  it('renames a department, starting from its current name', async () => {
    await openDepartments()
    await userEvent.click(screen.getByRole('button', { name: 'Rename Sales' }))

    const name = screen.getByLabelText('Department name')
    expect(name).toHaveValue('Sales')
    await userEvent.clear(name)
    await userEvent.type(name, 'Revenue')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.renameOrgDepartment).toHaveBeenCalledWith(11, 'Revenue')
    )
    expect(api.createOrgDepartment).not.toHaveBeenCalled()
  })

  it('says where the members go before deleting, and deletes only on confirm', async () => {
    await openDepartments()
    await userEvent.click(screen.getByRole('button', { name: 'Delete Sales' }))

    expect(
      screen.getByText(
        '“Sales” will be deleted. Its members and unused invite links move to “General”.'
      )
    ).toBeInTheDocument()
    expect(api.deleteOrgDepartment).not.toHaveBeenCalled()

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() =>
      expect(api.deleteOrgDepartment).toHaveBeenCalledWith(11)
    )
    expect(mockToast.success).toHaveBeenCalledWith('Department deleted')
  })

  it('leaves the department alone when the deletion is cancelled', async () => {
    await openDepartments()
    await userEvent.click(screen.getByRole('button', { name: 'Delete Sales' }))
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() =>
      expect(screen.queryByText('Delete this department?')).toBeNull()
    )
    expect(api.deleteOrgDepartment).not.toHaveBeenCalled()
  })
})

describe('invite links', () => {
  it('creates a link for a role and a department and shows it once made', async () => {
    await renderPage(ownerMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Invite members' })
    )

    // Starts on Staff, in the default department.
    expect(dropdown('Role')).toHaveTextContent('Staff')
    expect(dropdown('Department')).toHaveTextContent('General')
    await choose('Role', 'Manager')
    await choose('Department', 'Sales')
    await userEvent.click(screen.getByRole('button', { name: 'Create link' }))

    await waitFor(() =>
      expect(api.createOrgInvite).toHaveBeenCalledWith({
        role_id: 3,
        department_id: 11,
      })
    )
    expect(await screen.findByLabelText('Invite link')).toHaveValue(
      `${window.location.origin}/sign-up?org_invite=NEWCODE`
    )
  })

  it('makes an admin link lead to the default department only', async () => {
    await renderPage(ownerMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Invite members' })
    )

    // Whatever department was picked first, choosing Admin overrides it.
    await choose('Department', 'Sales')
    await choose('Role', 'Admin')
    expect(dropdown('Department')).toBeDisabled()
    expect(dropdown('Department')).toHaveTextContent('General')
    expect(screen.getByText(PINNED)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Create link' }))

    await waitFor(() =>
      expect(api.createOrgInvite).toHaveBeenCalledWith({
        role_id: 2,
        department_id: 10,
      })
    )
  })

  it('does not let an admin invite admins', async () => {
    await renderPage(adminMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Invite members' })
    )

    expect(await optionsOf('Role')).toEqual(['Manager', 'Staff', 'Read-only'])
  })

  it('lists usable links and revokes one on confirm', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('tab', { name: /Invite links/ }))

    const row = rowOf('Sales')
    expect(row).toHaveTextContent('Staff')
    await userEvent.click(within(row).getByRole('button', { name: 'Revoke' }))
    expect(api.revokeOrgInvite).not.toHaveBeenCalled()

    // The confirm dialog's button, not the row's.
    const dialog = screen.getByRole('alertdialog')
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Revoke' })
    )
    await waitFor(() => expect(api.revokeOrgInvite).toHaveBeenCalledWith(7))
    expect(mockToast.success).toHaveBeenCalledWith('Invite link revoked')
  })

  it('explains itself when there are no links yet', async () => {
    api.fetchOrgInvites.mockResolvedValue([])
    await renderPage()
    await userEvent.click(screen.getByRole('tab', { name: /Invite links/ }))

    expect(screen.getByText('No invite links yet')).toBeInTheDocument()
  })
})

describe('loading failures', () => {
  it('shows an error with a retry instead of an empty organization', async () => {
    api.fetchOrgMembership.mockResolvedValue(ownerMembership)
    api.fetchOrgMembers.mockRejectedValue(new Error('network'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrgMembersPage />
      </QueryClientProvider>
    )

    expect(
      await screen.findByText('Could not load your organization.')
    ).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: /Members/ })).toBeNull()
    expect(
      screen.getByRole('button', { name: 'Invite members' })
    ).toBeDisabled()

    api.fetchOrgMembers.mockResolvedValue(members)
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('tab', { name: /Members/ })).toBeVisible()
  })

  it('says so when it cannot tell who is asking, rather than loading forever', async () => {
    api.fetchOrgMembership.mockRejectedValue(new Error('rate limited'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrgMembersPage />
      </QueryClientProvider>
    )

    expect(
      await screen.findByText('Could not load your organization.')
    ).toBeInTheDocument()

    api.fetchOrgMembership.mockResolvedValue(ownerMembership)
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('tab', { name: /Members/ })).toBeVisible()
  })
})
