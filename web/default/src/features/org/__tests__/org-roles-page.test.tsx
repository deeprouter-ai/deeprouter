// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { OrgRolesPage } from '../roles'
import type { OrgMembership, OrgPermissionCatalog, OrgRole } from '../types'
import { catalog, memberOf, membershipOf, presetRoles } from './fixtures'

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md): the "Roles &
// permissions" page. The backend decides what is allowed and says what every
// role grants; these tests pin that the page shows exactly that, offers each
// kind of member what they may do, and asks the backend for the right thing.

const api = vi.hoisted(() => ({
  fetchOrgMembership: vi.fn(),
  fetchOrgRoles: vi.fn(),
  fetchOrgPermissions: vi.fn(),
  fetchOrgMembers: vi.fn(),
  createOrgRole: vi.fn(),
  updateOrgRole: vi.fn(),
  deleteOrgRole: vi.fn(),
  adoptOrgRolePack: vi.fn(),
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

const keyDesk: OrgRole = {
  id: 21,
  name: 'Key Desk',
  scope: 'org',
  permissions: ['key.read', 'key.freeze'],
  powers: [],
  is_preset: false,
}
const teamLead: OrgRole = {
  id: 22,
  name: 'Team Lead',
  scope: 'dept',
  permissions: ['member.read', 'member.invite'],
  powers: [],
  is_preset: false,
}
const members = [
  memberOf({ id: 1, username: 'founder', role_id: 1, role: 'owner' }),
  memberOf({ id: 2, username: 'kim', role_id: 21, role: 'Key Desk' }),
  memberOf({ id: 3, username: 'lee', role_id: 21, role: 'Key Desk' }),
]

/** Renders the page as the given member and waits for the matrix. */
async function renderPage(membership: OrgMembership = membershipOf('owner')) {
  api.fetchOrgMembership.mockResolvedValue(membership)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <OrgRolesPage />
    </QueryClientProvider>
  )
  await screen.findByRole('heading', { name: 'Preset roles' })
}

/** The matrix table, wherever its section stands on the page. */
function matrix(): HTMLElement {
  return within(screen.getByRole('region', { name: 'Preset roles' })).getByRole(
    'table'
  )
}

/** What the matrix answers for a permission, one word per role column. */
function ticks(permission: string): string[] {
  const row = within(matrix()).getByText(permission).closest('tr')
  if (!row) throw new Error(`no matrix row for ${permission}`)
  return within(row)
    .getAllByRole('cell')
    .map((cell) => cell.textContent?.replace('—', '').trim() ?? '')
}

/** The table row that mentions the given text. */
function rowOf(text: string): HTMLElement {
  const row = screen.getByText(text).closest('tr')
  if (!row) throw new Error(`no table row for ${text}`)
  return row
}

/** The card of a role pack. */
function pack(name: string): HTMLElement {
  return screen.getByRole('region', { name })
}

/** Whether a checkbox is locked; its root shows that as aria-disabled. */
function locked(name: RegExp): boolean {
  const box = screen.getByRole('checkbox', { name })
  return box.getAttribute('aria-disabled') === 'true'
}

const YES = 'Allowed'
const NO = 'Not allowed'

beforeEach(() => {
  vi.clearAllMocks()
  api.fetchOrgRoles.mockResolvedValue([...presetRoles, keyDesk, teamLead])
  api.fetchOrgPermissions.mockResolvedValue(catalog)
  api.fetchOrgMembers.mockResolvedValue(members)
  api.createOrgRole.mockResolvedValue({ success: true })
  api.updateOrgRole.mockResolvedValue({ success: true })
  api.deleteOrgRole.mockResolvedValue({ success: true })
  api.adoptOrgRolePack.mockResolvedValue({ success: true })
})

describe('layout', () => {
  it('puts the roles to manage first and the fixed presets last', async () => {
    await renderPage()

    const sections = screen
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent)
    expect(sections).toEqual(['Custom roles', 'Role packs', 'Preset roles'])
  })
})

describe('preset roles', () => {
  it('shows the five roles with how far each reaches', async () => {
    await renderPage()

    const headers = within(matrix())
      .getAllByRole('columnheader')
      .map((header) => header.textContent)
    expect(headers).toEqual([
      'Permission',
      'OwnerWhole organization',
      'AdminWhole organization',
      'ManagerDepartments they manage',
      'StaffOnly themselves',
      'Read-onlyWhole organization',
    ])
  })

  it('ticks exactly what each role grants', async () => {
    // Acceptance: 系统预设 owner/admin/manager/staff/readonly 五种角色，权限组合
    // 可查看. Columns: owner, admin, manager, staff, read-only (PRD §2).
    await renderPage()

    expect(ticks('View keys')).toEqual([YES, YES, YES, NO, YES])
    expect(ticks('Create keys')).toEqual([YES, YES, NO, NO, NO])
    expect(ticks('Assign keys')).toEqual([YES, YES, YES, NO, NO])
    expect(ticks('Invite members')).toEqual([YES, YES, YES, NO, NO])
    expect(ticks('Remove members')).toEqual([YES, YES, NO, NO, NO])
    expect(ticks('View usage')).toEqual([YES, YES, YES, NO, YES])
    expect(ticks('View the audit log')).toEqual([YES, YES, NO, NO, YES])
  })

  it('shows running the organization as the owner and admins alone', async () => {
    await renderPage()

    expect(screen.getByText('Running the organization')).toBeInTheDocument()
    expect(ticks('Manage roles')).toEqual([YES, YES, NO, NO, NO])
    expect(ticks('Manage service accounts')).toEqual([YES, YES, NO, NO, NO])
    expect(ticks('Appoint admins')).toEqual([YES, NO, NO, NO, NO])
    expect(ticks('Company wallet')).toEqual([YES, NO, NO, NO, NO])
  })

  it('draws every row and every tick from what the backend sent', async () => {
    // A primitive the page has never heard of, granted to the manager: it
    // shows up as its own row, under its own code, ticked for the manager.
    const extended: OrgPermissionCatalog = {
      ...catalog,
      primitives: [...catalog.primitives, 'key.share'],
    }
    api.fetchOrgPermissions.mockResolvedValue(extended)
    api.fetchOrgRoles.mockResolvedValue(
      presetRoles.map((role) =>
        role.name === 'manager'
          ? { ...role, permissions: [...role.permissions, 'key.share'] }
          : // …and a preset whose powers the backend changed is shown changed.
            role.name === 'admin'
            ? { ...role, powers: [...role.powers, 'admin.appoint'] }
            : role
      )
    )
    await renderPage()

    expect(ticks('key.share')).toEqual([NO, NO, YES, NO, NO])
    expect(ticks('Appoint admins')).toEqual([YES, YES, NO, NO, NO])
    // One row per primitive and per power, plus the header and group rows.
    expect(within(matrix()).getAllByRole('row')).toHaveLength(
      1 + extended.primitives.length + 5 + extended.powers.length + 1
    )
  })
})

describe('custom roles', () => {
  it('lists them with their reach, permissions and holders', async () => {
    await renderPage()

    const desk = rowOf('Key Desk')
    expect(desk).toHaveTextContent('Whole organization')
    expect(desk).toHaveTextContent('View keys')
    expect(desk).toHaveTextContent('Freeze keys')
    expect(within(desk).getAllByRole('cell')[3]).toHaveTextContent('2')
    const lead = rowOf('Team Lead')
    expect(lead).toHaveTextContent('Departments they manage')
    expect(within(lead).getAllByRole('cell')[3]).toHaveTextContent('0')
  })

  it('explains itself when there are none', async () => {
    api.fetchOrgRoles.mockResolvedValue(presetRoles)
    await renderPage()

    expect(screen.getByText('No custom roles yet')).toBeInTheDocument()
    expect(screen.getAllByRole('table')).toHaveLength(1)
  })

  it('builds a role from the permissions', async () => {
    // Acceptance: admin 能从权限原语清单自建自定义角色.
    await renderPage(membershipOf('admin'))
    await userEvent.click(
      screen.getAllByRole('button', { name: 'New role' })[0]
    )

    const save = screen.getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled() // a name and a permission are required
    await userEvent.type(screen.getByLabelText('Role name'), '  Support Lead  ')
    expect(save).toBeDisabled()
    await userEvent.click(screen.getByRole('checkbox', { name: /^View usage/ }))
    await userEvent.click(
      screen.getByRole('checkbox', { name: /^Assign keys/ })
    )
    await userEvent.click(save)

    // Sent in the backend's order, with the read the write brings.
    await waitFor(() =>
      expect(api.createOrgRole).toHaveBeenCalledWith({
        name: 'Support Lead',
        scope: 'org',
        permissions: ['key.read', 'key.assign', 'usage.read'],
      })
    )
    expect(mockToast.success).toHaveBeenCalledWith('Role created')
    await waitFor(() => expect(api.fetchOrgRoles).toHaveBeenCalledTimes(2))
  })

  it('ticks and locks the read a write brings', async () => {
    // PRD §2: 写权限自动包含同一资源的读权限.
    await renderPage()
    await userEvent.click(
      screen.getAllByRole('button', { name: 'New role' })[0]
    )

    const read = () => screen.getByRole('checkbox', { name: /^View members/ })
    expect(read()).not.toBeChecked()
    expect(locked(/^View members/)).toBe(false)

    await userEvent.click(
      screen.getByRole('checkbox', { name: /^Remove members/ })
    )
    expect(read()).toBeChecked()
    expect(locked(/^View members/)).toBe(true)
    expect(
      screen.getByText('Included with the other permissions you ticked.')
    ).toBeInTheDocument()

    // Taking the write back takes its read with it: nobody ticked that.
    await userEvent.click(
      screen.getByRole('checkbox', { name: /^Remove members/ })
    )
    expect(read()).not.toBeChecked()
    expect(locked(/^View members/)).toBe(false)
  })

  it('keeps the audit log for roles that reach the whole organization', async () => {
    await renderPage()
    await userEvent.click(
      screen.getAllByRole('button', { name: 'New role' })[0]
    )
    await userEvent.type(screen.getByLabelText('Role name'), 'Auditor')
    await userEvent.click(
      screen.getByRole('checkbox', { name: /^View the audit log/ })
    )
    await userEvent.click(screen.getByRole('checkbox', { name: /^View usage/ }))

    await userEvent.click(screen.getByRole('combobox', { name: 'Reach' }))
    await userEvent.click(
      await screen.findByRole('option', { name: 'Departments they manage' })
    )
    await waitFor(() => expect(screen.queryByRole('option')).toBeNull())

    expect(
      screen.getByRole('checkbox', { name: /^View the audit log/ })
    ).not.toBeChecked()
    expect(locked(/^View the audit log/)).toBe(true)
    expect(
      screen.getByText('Only for a role that reaches the whole organization.')
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.createOrgRole).toHaveBeenCalledWith({
        name: 'Auditor',
        scope: 'dept',
        permissions: ['usage.read'],
      })
    )
  })

  it('changes a role, starting from what it is', async () => {
    // Acceptance: 采用后可自由修改 / 修改角色权限即时生效.
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Key Desk' }))

    expect(screen.getByLabelText('Role name')).toHaveValue('Key Desk')
    expect(screen.getByRole('combobox', { name: 'Reach' })).toHaveTextContent(
      'Whole organization'
    )
    expect(screen.getByRole('checkbox', { name: /^Freeze keys/ })).toBeChecked()
    expect(
      screen.getByRole('checkbox', { name: /^Create keys/ })
    ).not.toBeChecked()
    expect(
      screen.getByText(
        'Changes apply at once to everyone who holds this role — they do not need to sign in again.'
      )
    ).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('checkbox', { name: /^Freeze keys/ })
    )
    await userEvent.click(
      screen.getByRole('checkbox', { name: /^Delete keys/ })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgRole).toHaveBeenCalledWith(21, {
        name: 'Key Desk',
        scope: 'org',
        permissions: ['key.read', 'key.delete'],
      })
    )
    expect(api.createOrgRole).not.toHaveBeenCalled()
    expect(mockToast.success).toHaveBeenCalledWith('Role updated')
  })

  it('says who is affected before deleting, and deletes only on confirm', async () => {
    // PRD D29: holders fall back to Staff.
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete Key Desk' })
    )

    expect(
      screen.getByText(
        '“Key Desk” will be deleted. The members who hold it (2) become Staff, and its unused invite links will bring in Staff.'
      )
    ).toBeInTheDocument()
    expect(api.deleteOrgRole).not.toHaveBeenCalled()

    const dialog = screen.getByRole('alertdialog')
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Delete' })
    )
    await waitFor(() => expect(api.deleteOrgRole).toHaveBeenCalledWith(21))
    expect(mockToast.success).toHaveBeenCalledWith('Role deleted')
  })

  it('says so when nobody holds the role being deleted', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete Team Lead' })
    )

    expect(
      screen.getByText(
        '“Team Lead” will be deleted. Nobody holds it; its unused invite links will bring in Staff.'
      )
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(api.deleteOrgRole).not.toHaveBeenCalled()
  })
})

describe('role packs', () => {
  it('shows the three packs with what each is for', async () => {
    await renderPage()

    expect(pack('IT Ops')).toHaveTextContent('Key administrator')
    expect(pack('IT Ops')).toHaveTextContent('Rotate keys')
    expect(pack('HR Ops')).toHaveTextContent('Remove members')
    expect(pack('Finance Ops')).toHaveTextContent('View usage')
    expect(pack('Finance Ops')).not.toHaveTextContent('View keys')
    for (const name of ['IT Ops', 'HR Ops', 'Finance Ops']) {
      expect(pack(name)).toHaveTextContent('Reach: Whole organization')
    }
  })

  it('adopts a pack in one click, under the name it is shown by', async () => {
    // Acceptance: 平台预置的岗位角色包可一键采用为组织的自定义角色.
    // The backend's own name for the pack is made different here, as it is in
    // any language but English: what gets sent is the name on the screen.
    api.fetchOrgPermissions.mockResolvedValue({
      ...catalog,
      role_packs: catalog.role_packs.map((p) =>
        p.key === 'finance' ? { ...p, name: 'finance (backend name)' } : p
      ),
    })
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Adopt Finance Ops' })
    )

    await waitFor(() =>
      expect(api.adoptOrgRolePack).toHaveBeenCalledWith(
        'finance',
        'Finance Ops'
      )
    )
    expect(mockToast.success).toHaveBeenCalledWith(
      '“Finance Ops” added to your roles'
    )
    await waitFor(() => expect(api.fetchOrgRoles).toHaveBeenCalledTimes(2))
  })

  it('marks a pack the organization already has', async () => {
    api.fetchOrgRoles.mockResolvedValue([
      ...presetRoles,
      { ...keyDesk, id: 30, name: 'HR Ops' },
    ])
    await renderPage()

    expect(
      within(pack('HR Ops')).getByRole('button', { name: 'Adopted' })
    ).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Adopt HR Ops' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Adopt IT Ops' })).toBeEnabled()
  })
})

describe('what each role is offered', () => {
  it('shows a read-only member everything and offers them nothing', async () => {
    await renderPage(membershipOf('readonly'))

    expect(ticks('Assign keys')).toEqual([YES, YES, YES, NO, NO])
    expect(rowOf('Key Desk')).toHaveTextContent('Freeze keys')
    expect(pack('IT Ops')).toBeInTheDocument()
    for (const name of [
      'New role',
      'Edit Key Desk',
      'Delete Key Desk',
      'Adopt IT Ops',
    ]) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
    expect(screen.queryByRole('columnheader', { name: 'Actions' })).toBeNull()
    // They see every member, so the head count is true and is shown.
    expect(within(rowOf('Key Desk')).getAllByRole('cell')[3]).toHaveTextContent(
      '2'
    )
  })

  it('leaves the head count out for a manager, who sees only some members', async () => {
    await renderPage(membershipOf('manager'))

    expect(rowOf('Key Desk')).toHaveTextContent('Freeze keys')
    expect(screen.queryByRole('columnheader', { name: 'Members' })).toBeNull()
    expect(api.fetchOrgMembers).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'New role' })).toBeNull()
  })

  it('gives an admin everything the owner has here', async () => {
    await renderPage(membershipOf('admin'))

    for (const name of ['Edit Key Desk', 'Delete Key Desk', 'Adopt IT Ops']) {
      expect(screen.getByRole('button', { name })).toBeEnabled()
    }
    expect(screen.getAllByRole('button', { name: 'New role' })[0]).toBeEnabled()
  })
})

describe('loading failures', () => {
  it('shows an error with a retry instead of an empty page', async () => {
    api.fetchOrgMembership.mockResolvedValue(membershipOf('owner'))
    api.fetchOrgPermissions.mockRejectedValue(new Error('network'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrgRolesPage />
      </QueryClientProvider>
    )

    expect(
      await screen.findByText('Could not load your organization.')
    ).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()

    api.fetchOrgPermissions.mockResolvedValue(catalog)
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('heading', { name: 'Preset roles' })
    ).toBeVisible()
  })
})
