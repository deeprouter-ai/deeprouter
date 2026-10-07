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
} from '../types'
import { memberOf, membershipOf, presetRoles } from './fixtures'

// Enterprise Org P3, P4 and P6 (meta-repo docs/enterprise-org-prd.md): the
// "Members & departments" page. The backend decides what is allowed; these
// tests pin what the page offers each kind of member and what it asks the
// backend for.

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
  removeOrgMember: vi.fn(),
  createOrgServiceAccount: vi.fn(),
  createOrgInvite: vi.fn(),
  revokeOrgInvite: vi.fn(),
}))
const mockToast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
/** Who is signed in: Fiona the founder unless a test says otherwise. */
const viewer = vi.hoisted(() => ({ id: 1 }))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  ...api,
}))
vi.mock('sonner', () => ({ toast: mockToast }))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: { id: viewer.id } } }),
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

const roles = presetRoles
const departments: OrgDepartment[] = [
  { id: 10, name: 'General', is_default: true, member_count: 3 },
  { id: 11, name: 'Sales', is_default: false, member_count: 1 },
]
const members: OrgMember[] = [
  memberOf({
    id: 1,
    username: 'founder',
    display_name: 'Fiona Founder',
    email: 'fiona@acme.test',
    role_id: 1,
    role: 'owner',
    is_owner: true,
  }),
  memberOf({ id: 2, username: 'adam', role_id: 2, role: 'admin' }),
  memberOf({ id: 3, username: 'sally', department_id: 11 }),
  memberOf({
    id: 4,
    username: 'svc-abc123def456',
    display_name: 'CI Pipeline',
    is_service: true,
  }),
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

const ownerMembership = membershipOf('owner')
const adminMembership = membershipOf('admin')

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

/**
 * Whether a checkbox is locked. Its root is not a form control, so the lock
 * shows as aria-disabled and not as the disabled attribute.
 */
function locked(name: string): boolean {
  const box = screen.getByRole('checkbox', { name })
  return box.getAttribute('aria-disabled') === 'true'
}

/** The sentence under a department that cannot be chosen. */
const PINNED = 'The owner and admins always belong to “General”.'

beforeEach(() => {
  vi.clearAllMocks()
  viewer.id = 1
  api.removeOrgMember.mockResolvedValue({
    success: true,
    data: { reclaimed_keys: 0 },
  })
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

describe('finding a member', () => {
  // PRD D39: the list is found in the way the "who is it for" list of a key
  // is (D34) — by typing, by department, by role. Product and two more people
  // are there so that each of the three has something to tell apart.
  const withProduct: OrgDepartment[] = [
    ...departments,
    { id: 12, name: 'Product', is_default: false, member_count: 1 },
  ]
  const mona = memberOf({
    id: 5,
    username: 'mona',
    role_id: 3,
    role: 'manager',
    department_id: 11,
    managed_department_ids: [11],
  })
  const paula = memberOf({
    id: 6,
    username: 'paula',
    email: 'paula@acme.test',
    department_id: 12,
  })
  const company = [...members, mona, paula]

  /** The names on the rows of the members table, top to bottom. */
  function listed(): string[] {
    const [, ...rows] = screen.getAllByRole('row')
    return rows.map((row) => row.querySelector('td span')?.textContent ?? '')
  }
  const search = () =>
    screen.getByRole('searchbox', { name: 'Search by name, username or email' })
  /** Removes a member through their row and the question that follows. */
  async function removeThroughThePage(name: string) {
    await userEvent.click(
      screen.getByRole('button', {
        name: `Remove ${name} from the organization`,
      })
    )
    await userEvent.click(
      within(screen.getByRole('alertdialog')).getByRole('button', {
        name: 'Remove from organization',
      })
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
  }

  beforeEach(() => {
    api.fetchOrgDepartments.mockResolvedValue(withProduct)
    api.fetchOrgMembers.mockResolvedValue(company)
  })

  it('finds a member by a name, a username or an email, whatever the case', async () => {
    await renderPage()
    expect(listed()).toEqual([
      'Fiona Founder',
      'adam',
      'sally',
      'CI Pipeline',
      'mona',
      'paula',
    ])

    await userEvent.type(search(), 'FIONA')
    expect(listed()).toEqual(['Fiona Founder'])
    // What a service account is told apart by: the generated username.
    await userEvent.clear(search())
    await userEvent.type(search(), 'svc-')
    expect(listed()).toEqual(['CI Pipeline'])
    await userEvent.clear(search())
    await userEvent.type(search(), 'paula@')
    expect(listed()).toEqual(['paula'])
    // The number on the tab is the organization's, not the search's.
    expect(screen.getByRole('tab', { name: /Members/ })).toHaveTextContent('6')
  })

  it('narrows the list down to a department, to a role, and to both', async () => {
    await renderPage()
    // What there is to narrow down to is read off the people themselves.
    expect(await optionsOf('Department')).toEqual([
      'All departments',
      'General',
      'Sales',
      'Product',
    ])
    expect(await optionsOf('Role')).toEqual([
      'All roles',
      'Owner',
      'Admin',
      'Manager',
      'Staff',
    ])

    await choose('Department', 'Sales')
    expect(listed()).toEqual(['sally', 'mona'])
    await choose('Role', 'Staff')
    expect(listed()).toEqual(['sally'])

    await choose('Department', 'All departments')
    expect(listed()).toEqual(['sally', 'CI Pipeline', 'paula'])
    // What is typed, on top of a filter.
    await userEvent.type(search(), 'p')
    expect(listed()).toEqual(['CI Pipeline', 'paula'])
  })

  it('says so when nobody matches, and lists everyone again without the search', async () => {
    await renderPage()

    await userEvent.type(search(), 'zzz')
    expect(screen.getByText('Nobody matches.')).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()

    await userEvent.clear(search())
    expect(screen.queryByText('Nobody matches.')).toBeNull()
    expect(listed()).toHaveLength(6)
  })

  it('leaves out a filter that has one choice', async () => {
    // A manager of Sales sees Sales alone: nothing to tell apart by department.
    api.fetchOrgDepartments.mockResolvedValue([departments[1]])
    api.fetchOrgMembers.mockResolvedValue([mona, members[2]])
    await renderPage(
      membershipOf('manager', {
        department_id: 11,
        managed_department_ids: [11],
      })
    )

    expect(screen.queryByRole('combobox', { name: 'Department' })).toBeNull()
    expect(await optionsOf('Role')).toEqual(['All roles', 'Manager', 'Staff'])
    expect(search()).toBeInTheDocument()
  })

  it('offers the search alone where everyone has one role and one department', async () => {
    api.fetchOrgDepartments.mockResolvedValue([departments[1]])
    api.fetchOrgMembers.mockResolvedValue([
      members[2],
      memberOf({ id: 7, username: 'sam', department_id: 11 }),
    ])
    await renderPage(membershipOf('readonly'))

    expect(screen.queryByRole('combobox')).toBeNull()
    await userEvent.type(search(), 'sam')
    expect(listed()).toEqual(['sam'])
  })

  it('keeps what was chosen when the list changes under it', async () => {
    await renderPage()
    await choose('Department', 'Sales')
    expect(listed()).toEqual(['sally', 'mona'])

    api.fetchOrgMembers.mockResolvedValue(
      company.filter((member) => member !== mona)
    )
    await removeThroughThePage('mona')

    await waitFor(() => expect(listed()).toEqual(['sally']))
    expect(dropdown('Department')).toHaveTextContent('Sales')
  })

  it('opens a filter again when what it was set to is gone', async () => {
    await renderPage()
    await choose('Department', 'Product')
    expect(listed()).toEqual(['paula'])

    // The last person of Product leaves, and Product with them: left on it,
    // the table would be empty with no way of saying why.
    api.fetchOrgMembers.mockResolvedValue(
      company.filter((member) => member !== paula)
    )
    await removeThroughThePage('paula')

    await waitFor(() =>
      expect(listed()).toEqual([
        'Fiona Founder',
        'adam',
        'sally',
        'CI Pipeline',
        'mona',
      ])
    )
    expect(dropdown('Department')).toHaveTextContent('All departments')
    expect(await optionsOf('Department')).toEqual([
      'All departments',
      'General',
      'Sales',
    ])
  })
})

describe('removing a member', () => {
  /** The button that removes the member of that name, if the row offers it. */
  const removeButton = (name: string) =>
    screen.queryByRole('button', {
      name: `Remove ${name} from the organization`,
    })
  /** A second admin, for looking at one admin through the eyes of another. */
  const anna = memberOf({ id: 6, username: 'anna', role_id: 2, role: 'admin' })

  it('lets the owner remove anyone but themselves', async () => {
    await renderPage()

    expect(removeButton('Fiona Founder')).toBeNull()
    for (const name of ['adam', 'sally', 'CI Pipeline']) {
      expect(removeButton(name)).toBeEnabled()
    }
  })

  it('keeps an admin from removing the owner, another admin, or themselves', async () => {
    // Adam the admin is signed in. Dismissing an admin is the owner's alone.
    viewer.id = 2
    api.fetchOrgMembers.mockResolvedValue([...members, anna])
    await renderPage(adminMembership)

    for (const name of ['Fiona Founder', 'adam', 'anna']) {
      expect(removeButton(name)).toBeNull()
    }
    expect(removeButton('sally')).toBeEnabled()
    expect(removeButton('CI Pipeline')).toBeEnabled()
    // Editing is still offered on every row.
    expect(screen.getByRole('button', { name: 'Edit anna' })).toBeEnabled()
  })

  it('offers it to a role that may remove members, and nothing else on the row', async () => {
    // The HR pack: member.remove across the organization, no power to edit.
    // Hana of HR is signed in, and is on the list like everyone else.
    viewer.id = 9
    api.fetchOrgMembers.mockResolvedValue([
      ...members,
      memberOf({ id: 9, username: 'hana', role_id: 20, role: 'HR Ops' }),
      memberOf({ id: 8, username: 'hugo', role_id: 20, role: 'HR Ops' }),
    ])
    await renderPage(
      membershipOf('staff', {
        role: 'HR Ops',
        role_scope: 'org',
        permissions: ['member.read', 'member.invite', 'member.remove'],
      })
    )

    expect(removeButton('sally')).toBeEnabled()
    expect(removeButton('CI Pipeline')).toBeEnabled()
    expect(removeButton('Fiona Founder')).toBeNull()
    expect(removeButton('adam')).toBeNull()
    // Not themselves — although a colleague with the very same role is theirs
    // to remove.
    expect(removeButton('hana')).toBeNull()
    expect(removeButton('hugo')).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Edit sally' })).toBeNull()
  })

  it('offers it to nobody who may not remove members', async () => {
    for (const membership of [
      membershipOf('manager'),
      membershipOf('readonly'),
    ]) {
      api.fetchOrgMembership.mockResolvedValue(membership)
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
      const view = render(
        <QueryClientProvider client={queryClient}>
          <OrgMembersPage />
        </QueryClientProvider>
      )
      await screen.findByRole('tab', { name: /Members/ })
      expect(removeButton('sally')).toBeNull()
      expect(screen.queryByRole('columnheader', { name: 'Actions' })).toBeNull()
      view.unmount()
    }
  })

  it('says what removing a person means, and removes them only on confirm', async () => {
    // Acceptance: 成员被移出组织时，其名下的组织 key 全部按回收处理（冻结并挂回 owner
    // 名下），历史用量保留.
    api.fetchOrgMembers.mockResolvedValue([
      members[0],
      members[1],
      { ...members[2], key_count: 2 },
      members[3],
    ])
    api.removeOrgMember.mockResolvedValue({
      success: true,
      data: { reclaimed_keys: 2 },
    })
    await renderPage()
    await userEvent.click(removeButton('sally') as HTMLElement)

    const confirm = screen.getByRole('alertdialog')
    expect(confirm).toHaveTextContent('Remove sally from the organization?')
    expect(confirm).toHaveTextContent(
      'Their account is deleted: they can no longer sign in, and nobody can sign up with the same username or email again.'
    )
    expect(confirm).toHaveTextContent(
      'The 2 key(s) held under this account are taken back: they stop working at once, get a new value and go back under the owner, frozen.'
    )
    expect(confirm).toHaveTextContent(
      'What it has spent stays in the usage records. This cannot be undone.'
    )
    expect(api.removeOrgMember).not.toHaveBeenCalled()

    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Remove from organization' })
    )
    await waitFor(() => expect(api.removeOrgMember).toHaveBeenCalledWith(3))
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith(
        'sally is no longer in the organization',
        { description: '2 key(s) were taken back.' }
      )
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    await waitFor(() => expect(api.fetchOrgMembers).toHaveBeenCalledTimes(2))
  })

  it('says a member without keys holds none', async () => {
    await renderPage()
    await userEvent.click(removeButton('sally') as HTMLElement)

    const confirm = screen.getByRole('alertdialog')
    expect(confirm).toHaveTextContent('This account holds no keys.')
    expect(confirm).not.toHaveTextContent('are taken back')
    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Remove from organization' })
    )
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith(
        'sally is no longer in the organization',
        undefined
      )
    )
  })

  it('deletes a service account in words of its own', async () => {
    api.fetchOrgMembers.mockResolvedValue([
      ...members.slice(0, 3),
      { ...members[3], key_count: 1 },
    ])
    await renderPage()
    await userEvent.click(removeButton('CI Pipeline') as HTMLElement)

    const confirm = screen.getByRole('alertdialog')
    expect(confirm).toHaveTextContent('Delete the service account CI Pipeline?')
    expect(confirm).toHaveTextContent('The service account is deleted.')
    expect(confirm).not.toHaveTextContent('sign in')
    expect(confirm).toHaveTextContent(
      'The 1 key(s) held under this account are taken back'
    )
    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Delete' })
    )
    await waitFor(() => expect(api.removeOrgMember).toHaveBeenCalledWith(4))
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith(
        'CI Pipeline is no longer in the organization',
        undefined
      )
    )
  })

  it('removes nobody when the question is answered with no', async () => {
    await renderPage()
    await userEvent.click(removeButton('sally') as HTMLElement)
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(api.removeOrgMember).not.toHaveBeenCalled()
  })

  it('claims nothing when the backend refuses, or the request fails', async () => {
    api.removeOrgMember.mockResolvedValueOnce({
      success: false,
      message: 'refused',
    })
    await renderPage()
    await userEvent.click(removeButton('sally') as HTMLElement)
    const confirm = screen.getByRole('alertdialog')
    const button = within(confirm).getByRole('button', {
      name: 'Remove from organization',
    })
    await userEvent.click(button)
    await waitFor(() => expect(api.removeOrgMember).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(button).toBeEnabled())
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()

    // A 403 reaches the page as a rejection; the interceptor has said why.
    api.removeOrgMember.mockRejectedValueOnce(new Error('403'))
    await userEvent.click(button)
    await waitFor(() => expect(api.removeOrgMember).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(button).toBeEnabled())
    expect(mockToast.success).not.toHaveBeenCalled()
    expect(api.fetchOrgMembers).toHaveBeenCalledTimes(1)
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

describe('departments a member manages', () => {
  // PRD D28: a department-scoped role manages the member's own department,
  // plus any an owner or admin ticks. Product exists so there is one to tick.
  const withProduct: OrgDepartment[] = [
    ...departments,
    { id: 12, name: 'Product', is_default: false, member_count: 0 },
  ]
  const mona = memberOf({
    id: 5,
    username: 'mona',
    role_id: 3,
    role: 'manager',
    department_id: 11,
    managed_department_ids: [11, 12],
  })

  beforeEach(() => {
    api.fetchOrgDepartments.mockResolvedValue(withProduct)
    api.fetchOrgMembers.mockResolvedValue([...members, mona])
  })

  it('says in the list what a manager manages besides their own department', async () => {
    await renderPage()

    expect(rowOf('mona')).toHaveTextContent('Sales')
    expect(rowOf('mona')).toHaveTextContent('Also manages: Product')
    // Nobody else manages anything, and the list does not pretend otherwise.
    expect(rowOf('sally')).not.toHaveTextContent('Also manages')
    expect(rowOf('adam')).not.toHaveTextContent('Also manages')
  })

  it('offers the departments only for a role that manages some', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))

    expect(screen.queryByText('Departments they manage')).toBeNull()
    await choose('Role', 'Manager')
    expect(screen.getByText('Departments they manage')).toBeInTheDocument()
    // Their own department is ticked and cannot be unticked.
    expect(screen.getByRole('checkbox', { name: 'Sales' })).toBeChecked()
    expect(locked('Sales')).toBe(true)
    expect(screen.getByRole('checkbox', { name: 'Product' })).not.toBeChecked()
    expect(locked('Product')).toBe(false)

    await choose('Role', 'Read-only')
    expect(screen.queryByText('Departments they manage')).toBeNull()
  })

  it('makes a manager without sending departments nobody ticked', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))
    await choose('Role', 'Manager')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    // The backend makes a new manager manage their own department by itself.
    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(3, { role_id: 3 })
    )
  })

  it('sends the whole list when a department is ticked', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit sally' }))
    await choose('Role', 'Manager')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Product' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(3, {
        role_id: 3,
        managed_department_ids: [11, 12],
      })
    )
  })

  it('starts from what a manager already manages and takes a department away', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit mona' }))

    expect(locked('Sales')).toBe(true)
    expect(screen.getByRole('checkbox', { name: 'Product' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'General' })).not.toBeChecked()

    await userEvent.click(screen.getByRole('checkbox', { name: 'Product' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(5, {
        managed_department_ids: [11],
      })
    )
  })

  it('moves a manager without touching what was added for them', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit mona' }))

    // Moved to General: that is now the one they cannot untick, Sales is
    // theirs to tick again, and Product stays as it was — so only the move
    // is sent.
    await choose('Department', 'General')
    expect(locked('General')).toBe(true)
    expect(screen.getByRole('checkbox', { name: 'General' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Sales' })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Product' })).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgMember).toHaveBeenCalledWith(5, { department_id: 10 })
    )
  })
})

describe('what each role is offered', () => {
  // Acceptance: 部门作用域硬检查. The backend sends a manager only the people
  // and departments of the departments they manage; the page must then offer
  // only what a manager may do with them.
  const managerMembership = membershipOf('manager', {
    department_id: 11,
    managed_department_ids: [11],
  })
  const salesOnly = [departments[1]]
  const salesPeople = [
    memberOf({
      id: 5,
      username: 'mona',
      role_id: 3,
      role: 'manager',
      department_id: 11,
      managed_department_ids: [11],
    }),
    members[2],
  ]

  it('gives a manager their people, invite links and nothing to manage', async () => {
    api.fetchOrgDepartments.mockResolvedValue(salesOnly)
    api.fetchOrgMembers.mockResolvedValue(salesPeople)
    await renderPage(managerMembership)

    expect(
      screen.getByText('You see the members of the departments you manage.')
    ).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /Members/ })).toHaveTextContent('2')
    expect(rowOf('sally')).toHaveTextContent('Sales')
    // Inviting is theirs; roles, departments and service accounts are not.
    expect(screen.getByRole('button', { name: 'Invite members' })).toBeEnabled()
    expect(
      screen.queryByRole('button', { name: 'Add service account' })
    ).toBeNull()
    expect(screen.queryByRole('button', { name: 'Edit sally' })).toBeNull()
    expect(screen.queryByRole('columnheader', { name: 'Actions' })).toBeNull()

    await userEvent.click(screen.getByRole('tab', { name: /Departments/ }))
    expect(screen.getByText('Sales')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'New department' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Rename Sales' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Delete Sales' })).toBeNull()

    // Their invite links are listed, and they can revoke one.
    await userEvent.click(screen.getByRole('tab', { name: /Invite links/ }))
    expect(
      within(rowOf('Staff')).getByRole('button', { name: 'Revoke' })
    ).toBeInTheDocument()
  })

  it('lets a manager invite Staff, into their departments only', async () => {
    api.fetchOrgDepartments.mockResolvedValue(salesOnly)
    api.fetchOrgMembers.mockResolvedValue(salesPeople)
    await renderPage(managerMembership)
    await userEvent.click(
      screen.getByRole('button', { name: 'Invite members' })
    )

    // Acceptance: 只能邀请本部门的 staff.
    expect(await optionsOf('Role')).toEqual(['Staff'])
    expect(await optionsOf('Department')).toEqual(['Sales'])
    await userEvent.click(screen.getByRole('button', { name: 'Create link' }))

    await waitFor(() =>
      expect(api.createOrgInvite).toHaveBeenCalledWith({
        role_id: 4,
        department_id: 11,
      })
    )
  })

  it('shows a read-only member everything and offers them nothing', async () => {
    await renderPage(membershipOf('readonly'))

    expect(screen.getByRole('tab', { name: /Members/ })).toHaveTextContent('4')
    expect(rowOf('Fiona Founder')).toHaveTextContent('Owner')
    expect(
      screen.queryByText('You see the members of the departments you manage.')
    ).toBeNull()
    for (const name of [
      'Invite members',
      'Add service account',
      'Edit sally',
    ]) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
    // A link is a way in: the tab is not there and the list is never asked for.
    expect(screen.queryByRole('tab', { name: /Invite links/ })).toBeNull()
    expect(api.fetchOrgInvites).not.toHaveBeenCalled()

    await userEvent.click(screen.getByRole('tab', { name: /Departments/ }))
    expect(screen.getByText('Sales')).toBeInTheDocument()
    for (const name of ['New department', 'Rename Sales', 'Delete Sales']) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
  })

  it('keeps every button for an admin', async () => {
    await renderPage(adminMembership)

    for (const name of [
      'Invite members',
      'Add service account',
      'Edit sally',
    ]) {
      expect(screen.getByRole('button', { name })).toBeEnabled()
    }
    expect(screen.getByRole('tab', { name: /Invite links/ })).toBeVisible()
    await userEvent.click(screen.getByRole('tab', { name: /Departments/ }))
    for (const name of ['New department', 'Rename Sales', 'Delete Sales']) {
      expect(screen.getByRole('button', { name })).toBeEnabled()
    }
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
