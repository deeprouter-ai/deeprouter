// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { OrgKeysPage } from '../keys'
import type { OrgKey, OrgKeyHolder, OrgMembership } from '../types'
import { keyHolders, keyOf, keyTemplates, membershipOf } from './fixtures'

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3): the
// "Organization keys" page. The backend decides what is allowed and never
// sends a key's value in a list; these tests pin what the page offers each
// kind of member, what it asks the backend for, and the one moment a value is
// on the screen — a service account's, right after it was made.

const api = vi.hoisted(() => ({
  fetchOrgMembership: vi.fn(),
  fetchOrgKeys: vi.fn(),
  fetchOrgKeyHolders: vi.fn(),
  fetchOrgKeyTemplates: vi.fn(),
  fetchOrgKeyModels: vi.fn(),
  createOrgKey: vi.fn(),
  updateOrgKey: vi.fn(),
  rotateOrgKey: vi.fn(),
  freezeOrgKey: vi.fn(),
  unfreezeOrgKey: vi.fn(),
  deleteOrgKey: vi.fn(),
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
    i18n: { language: 'en' },
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

/** A person's key: sally of Sales, no template, 10 dollars left. */
const designKey = keyOf()
/** A service account's key with a template, no quota limit and a rate limit. */
const buildKey = keyOf({
  id: 101,
  name: 'Nightly build',
  key: 'ci01**********ci99',
  holder_id: 4,
  holder: 'CI Pipeline',
  holder_is_service: true,
  department_id: 10,
  department: 'General',
  policy_template: 'coding',
  model_limits: ['model-a', 'model-b'],
  unlimited_quota: true,
  remain_quota: 0,
  rpm_limit: 60,
})
/** A key someone froze, parked under the owner. */
const frozenKey = keyOf({
  id: 102,
  name: 'Old laptop',
  key: 'froz**********en00',
  status: 2,
  holder_id: 1,
  holder: 'Fiona Founder',
  department_id: 10,
  department: 'General',
})
const keys = [designKey, buildKey, frozenKey]

/** A key limited to two models someone picked by hand. */
const pickedKey = keyOf({
  id: 103,
  name: 'Writers',
  key: 'pick**********ed00',
  model_limits: ['claude-sonnet-5', 'gpt-4o'],
})

/** What `GET /api/org/key-models` offers to pick from. */
const OFFERED = ['claude-sonnet-5', 'gpt-4o', 'gpt-image-1']

/** A value the backend answers once; nothing may keep it on the page. */
const SECRET = 'SVCkeyVALUEshownONCE0123456789'

/** A custom role that holds exactly the given primitives across the organization. */
function customRole(role: string, permissions: string[]): OrgMembership {
  return membershipOf('staff', { role, role_scope: 'org', permissions })
}

/**
 * Renders the page as the given member and waits until it has loaded. A test
 * that looks at it as several members unmounts the answer in between.
 */
async function renderPage(membership: OrgMembership = membershipOf('admin')) {
  api.fetchOrgMembership.mockResolvedValue(membership)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={queryClient}>
      <OrgKeysPage />
    </QueryClientProvider>
  )
  await screen.findByText(/^A key’s value never appears on this page\./)
  return view
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

/** What one row of a list says: its pieces of text, in order, with bars between. */
function rowText(row: HTMLElement): string {
  const pieces: string[] = []
  const walker = document.createTreeWalker(row, NodeFilter.SHOW_TEXT)
  while (walker.nextNode()) {
    const piece = walker.currentNode.textContent?.trim()
    if (piece) pieces.push(piece)
  }
  return pieces.join(' | ')
}

/** The rows of the open "who is it for" list: the people, not a filter's choices. */
function personRows(): HTMLElement[] {
  return screen
    .queryAllByRole('option')
    .filter((option) => option.dataset.slot === 'command-item')
}

/** The people the "who is it for" list shows right now, as each row reads. */
function people(): string[] {
  return personRows().map(rowText)
}

/** The row of the open list for the person of that name. */
function person(name: string): HTMLElement {
  const row = personRows().find(
    (each) => rowText(each).split(' | ')[0] === name
  )
  if (!row) throw new Error(`nobody called ${name} in the list`)
  return row
}

/** The name on the row Enter would pick. */
function highlighted(): string | undefined {
  const row = personRows().find((each) => each.ariaSelected === 'true')
  return row && rowText(row).split(' | ')[0]
}

/** The field a name is typed into to find a person. */
function search(): HTMLElement {
  return screen.getByPlaceholderText('Search by name')
}

/** Whether the "who is it for" list is open. */
function peopleOpen(): boolean {
  return screen.queryByPlaceholderText('Search by name') !== null
}

/** Opens the "who is it for" list and waits for it. */
async function openPeople() {
  await userEvent.click(dropdown('Who it is for'))
  await screen.findByPlaceholderText('Search by name')
}

/** Picks who the key is for by pressing their row, the way a person would. */
async function chooseHolder(name: string) {
  await openPeople()
  await userEvent.click(person(name))
  await waitFor(() => expect(peopleOpen()).toBe(false))
}

/** The choices one of the list's two filters shows while it is open. */
function filterChoices(): HTMLElement[] {
  return screen
    .queryAllByRole('option')
    .filter((option) => option.dataset.slot === 'select-item')
}

/** Opens a filter of the list, reads what it offers, and closes it again. */
async function choicesOf(label: 'Department' | 'Role'): Promise<string[]> {
  await userEvent.click(dropdown(label))
  await waitFor(() => expect(filterChoices()).not.toEqual([]))
  const labels = filterChoices().map((choice) => choice.textContent ?? '')
  await userEvent.keyboard('{Escape}')
  await waitFor(() => expect(filterChoices()).toEqual([]))
  return labels
}

/** Narrows the open list down with one of its two filters. */
async function narrow(label: 'Department' | 'Role', choice: string) {
  await userEvent.click(dropdown(label))
  await waitFor(() => expect(filterChoices()).not.toEqual([]))
  const wanted = filterChoices().find((each) => each.textContent === choice)
  if (!wanted) throw new Error(`no ${choice} to narrow down to`)
  await userEvent.click(wanted)
  // The filter's own list goes; the people stay.
  await waitFor(() => expect(filterChoices()).toEqual([]))
}

/** The hand-picked list of the key form. */
function picker(): HTMLElement {
  return screen.getByRole('group', { name: 'Specific models' })
}

/** The models the picker offers right now, with its list open. */
async function offeredModels(): Promise<string[]> {
  await userEvent.click(within(picker()).getByRole('combobox'))
  const options = await within(picker()).findAllByRole('option')
  return options.map((option) => option.textContent ?? '')
}

/** Picks one model from the list the picker offers. */
async function pick(model: string) {
  await userEvent.click(within(picker()).getByRole('combobox'))
  await userEvent.click(
    await within(picker()).findByRole('option', { name: model })
  )
}

/** Puts a number into a number field. */
function enter(label: string | RegExp, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
}

/** The names of the action buttons a key's row offers, in order. */
function actionsOf(key: OrgKey): string[] {
  return within(rowOf(key.name))
    .queryAllByRole('button')
    .map((button) => button.textContent ?? '')
}

/** What a new key is created with when the form is left as it opens. */
const UNTOUCHED = {
  holder_id: 1,
  policy_template: '',
  model_limits: [],
  remain_quota: 5000000,
  unlimited_quota: false,
  expired_time: -1,
  rpm_limit: 0,
  tpm_limit: 0,
  monthly_limit: 0,
}

beforeEach(() => {
  vi.clearAllMocks()
  api.fetchOrgKeys.mockResolvedValue(keys)
  api.fetchOrgKeyHolders.mockResolvedValue(keyHolders)
  api.fetchOrgKeyTemplates.mockResolvedValue(keyTemplates)
  api.fetchOrgKeyModels.mockResolvedValue(OFFERED)
  api.createOrgKey.mockImplementation((input: { name: string }) =>
    Promise.resolve({
      success: true,
      data: keyOf({ id: 200, name: input.name }),
    })
  )
  api.rotateOrgKey.mockResolvedValue({ success: true, data: designKey })
  for (const call of [
    api.updateOrgKey,
    api.freezeOrgKey,
    api.unfreezeOrgKey,
    api.deleteOrgKey,
  ]) {
    call.mockResolvedValue({ success: true })
  }
})

describe('the list', () => {
  it('shows each key with who holds it, what it may call and its state', async () => {
    await renderPage()

    const design = rowOf('Design tools')
    expect(design).toHaveTextContent('sally')
    expect(design).toHaveTextContent('Sales')
    expect(design).toHaveTextContent('sk-abcd**********wxyz')
    expect(design).toHaveTextContent('Every model')
    expect(design).toHaveTextContent('$10')
    expect(design).toHaveTextContent('Used $0')
    expect(design).toHaveTextContent('Working')
    expect(design).not.toHaveTextContent('Service account')

    const build = rowOf('Nightly build')
    expect(build).toHaveTextContent('CI Pipeline')
    expect(build).toHaveTextContent('Service account')
    expect(build).toHaveTextContent('Coding pack')
    expect(build).toHaveTextContent('No limit')
    expect(build).toHaveTextContent('60 requests/min')

    expect(rowOf('Old laptop')).toHaveTextContent('Frozen')
  })

  it('shows a key that ran out or ran past its date as what it is', async () => {
    api.fetchOrgKeys.mockResolvedValue([
      keyOf({ id: 1, name: 'Spent', remain_quota: 0 }),
      keyOf({ id: 2, name: 'Lapsed', expired_time: 1000 }),
      keyOf({ id: 3, name: 'Orphan', holder: '', department: '' }),
    ])
    await renderPage()

    expect(rowOf('Spent')).toHaveTextContent('Out of quota')
    expect(rowOf('Lapsed')).toHaveTextContent('Expired')
    expect(rowOf('Orphan')).toHaveTextContent('Someone who has left')
  })

  it('has nothing that shows or copies a value', async () => {
    // Acceptance: 组织 key 的明文不在任何界面显示. The list carries masked
    // values only, and the page has no way to ask for more.
    await renderPage(membershipOf('owner'))

    expect(screen.queryByRole('textbox')).toBeNull()
    for (const name of [/copy/i, /reveal/i, /show/i, /view/i]) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
    expect(
      screen.getByText(/^A key’s value never appears on this page\./)
    ).toBeInTheDocument()
  })

  it('tells a department manager that the list is their departments', async () => {
    const hint =
      'You see the keys held by members of the departments you manage.'
    await renderPage(membershipOf('manager'))
    expect(screen.getByText(hint)).toBeInTheDocument()
  })

  it('does not say that to someone who sees the whole organization', async () => {
    await renderPage(membershipOf('readonly'))
    expect(
      screen.queryByText(
        'You see the keys held by members of the departments you manage.'
      )
    ).toBeNull()
  })

  it('explains itself when there are no keys', async () => {
    api.fetchOrgKeys.mockResolvedValue([])
    await renderPage()

    expect(screen.getByText('No keys yet')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Create one for a member or a service account, with the models it may call and what it may spend.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
    // The header's button and the empty state's own.
    expect(screen.getAllByRole('button', { name: 'New key' })).toHaveLength(2)
  })

  it('does not invite someone who cannot create a key to create one', async () => {
    api.fetchOrgKeys.mockResolvedValue([])
    await renderPage(membershipOf('readonly'))

    expect(
      screen.getByText(
        'Keys appear here once someone who may create them has made one.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'New key' })).toBeNull()
  })
})

describe('what each role is offered', () => {
  it('gives the owner and an admin every action', async () => {
    for (const viewer of [membershipOf('owner'), membershipOf('admin')]) {
      const { unmount } = await renderPage(viewer)

      expect(screen.getByRole('button', { name: 'New key' })).toBeEnabled()
      expect(actionsOf(designKey)).toEqual([
        'Edit',
        'New value',
        'Freeze',
        'Delete',
      ])
      // A frozen key is offered the way back instead.
      expect(actionsOf(frozenKey)).toEqual([
        'Edit',
        'New value',
        'Unfreeze',
        'Delete',
      ])
      unmount()
    }
  })

  it('shows a read-only member the keys and offers them nothing', async () => {
    await renderPage(membershipOf('readonly'))

    expect(rowOf('Design tools')).toHaveTextContent('sk-abcd**********wxyz')
    expect(screen.queryByRole('button')).toBeNull()
    expect(screen.queryByRole('columnheader', { name: 'Actions' })).toBeNull()
    // Whom a key can be made out to is nobody's business who cannot make one.
    expect(api.fetchOrgKeyHolders).not.toHaveBeenCalled()
  })

  it('offers a manager nothing here: handing keys out is not on this page', async () => {
    // The preset manager holds key.read and key.assign (PRD §2).
    await renderPage(membershipOf('manager'))

    expect(rowOf('Design tools')).toBeInTheDocument()
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('offers a custom role exactly the actions it grants', async () => {
    const { unmount } = await renderPage(
      customRole('Key Desk', ['key.read', 'key.freeze'])
    )
    expect(actionsOf(designKey)).toEqual(['Freeze'])
    expect(actionsOf(frozenKey)).toEqual(['Unfreeze'])
    expect(screen.queryByRole('button', { name: 'New key' })).toBeNull()
    unmount()

    // The HR pack: stop a leaver's keys, nothing else (PRD §2).
    const hr = await renderPage(
      customRole('HR Ops', ['key.read', 'key.freeze', 'key.delete'])
    )
    expect(actionsOf(designKey)).toEqual(['Freeze', 'Delete'])
    hr.unmount()

    const editor = await renderPage(
      customRole('Key Editor', ['key.read', 'key.update', 'key.rotate'])
    )
    expect(actionsOf(designKey)).toEqual(['Edit', 'New value'])
    editor.unmount()

    await renderPage(customRole('Key Maker', ['key.read', 'key.create']))
    expect(screen.getByRole('button', { name: 'New key' })).toBeEnabled()
    expect(actionsOf(designKey)).toEqual([])
  })
})

describe('creating a key', () => {
  it('needs a name, and parks the key under the owner unless told otherwise', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))

    const create = screen.getByRole('button', { name: 'Create' })
    expect(create).toBeDisabled()
    expect(dropdown('Who it is for')).toHaveTextContent(
      'Fiona Founder (owner — not handed out yet)'
    )
    expect(
      screen.getByText(
        'The key waits under the owner. Its value is not shown to anyone here.'
      )
    ).toBeInTheDocument()

    await userEvent.type(screen.getByLabelText('Name'), '  Spare  ')
    await userEvent.click(create)

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Spare',
      })
    )
  })

  it('offers the members a key can be made out to, each as what they are', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))

    // The owner first — that is where a key waits — then everyone by name.
    await openPeople()
    expect(people()).toEqual([
      'Fiona Founder | Not handed out yet | General · Owner',
      'CI Pipeline | Service account | General · Staff',
      'sally | Sales · Staff',
    ])
    // The one the form is on is ticked.
    expect(personRows().map((row) => row.dataset.checked)).toEqual([
      'true',
      'false',
      'false',
    ])
  })

  it('makes a key for a person without ever showing its value', async () => {
    // Acceptance: 组织 key 的明文不在任何界面显示：成员的 key 只能由持有人
    // 本人经一键配置下发到自己的工具.
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Sales deck')
    await chooseHolder('sally')
    expect(dropdown('Who it is for')).toHaveTextContent('sally · Sales')
    expect(
      screen.getByText(
        'The key’s value is not shown to you. sally installs it into their own tools with one-click setup, from their API keys page.'
      )
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Sales deck',
        holder_id: 3,
      })
    )
    const done = await screen.findByRole('alertdialog')
    expect(done).toHaveTextContent('Key created')
    expect(done).toHaveTextContent(
      '“Sales deck” is held by sally. Its value is not shown here: sally signs in, opens their API keys page and installs it into their tools with one-click setup.'
    )
    expect(within(done).queryByRole('textbox')).toBeNull()
    expect(within(done).queryByRole('button', { name: 'Copy key' })).toBeNull()
    // The list is fetched again, so the page shows what the backend now has.
    await waitFor(() => expect(api.fetchOrgKeys).toHaveBeenCalledTimes(2))

    await userEvent.click(within(done).getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
  })

  it('shows a service account’s key once, and keeps nothing of it afterwards', async () => {
    // Acceptance: 服务账号的 key 在归到服务账号名下时向操作者展示一次.
    api.createOrgKey.mockResolvedValue({
      success: true,
      data: { ...buildKey, id: 201, name: 'Deploy bot', value: SECRET },
    })
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Deploy bot')
    await chooseHolder('CI Pipeline')
    expect(dropdown('Who it is for')).toHaveTextContent(
      'CI Pipeline (service account) · General'
    )
    // Said before the key exists: this is the one chance.
    expect(
      screen.getByText(
        'A service account cannot sign in, so the full key is shown to you once, right after it is created. Save it then.'
      )
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    const shown = await screen.findByRole('alertdialog')
    expect(shown).toHaveTextContent('Key created')
    expect(shown).toHaveTextContent(
      'This is the full key of “CI Pipeline”. It is shown this once: after you close this window nobody can see it again, and the only way to get one is to replace it.'
    )
    expect(
      within(shown).getByRole('textbox', { name: 'Full key' })
    ).toHaveValue(`sk-${SECRET}`)
    expect(
      within(shown).getByRole('button', { name: 'Copy key' })
    ).toBeEnabled()

    // This is the one look at the key: a stray Escape must not take it away.
    await userEvent.keyboard('{Escape}')
    expect(
      within(shown).getByRole('textbox', { name: 'Full key' })
    ).toHaveValue(`sk-${SECRET}`)

    await userEvent.click(
      within(shown).getByRole('button', { name: 'I have saved it' })
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(document.body.innerHTML).not.toContain(SECRET)
  })

  it('applies a policy template, saying what it allows', async () => {
    // Acceptance: 创建 key 时可：选策略模板（一键填充模型白名单）.
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Studio')

    expect(await optionsOf('Policy template')).toEqual([
      'Every model',
      'Creative pack',
      'Coding pack',
      'Specific models',
    ])
    expect(
      screen.getByText('The key may call every model the organization can use.')
    ).toBeInTheDocument()

    await choose('Policy template', 'Creative pack')
    expect(
      screen.getByText(
        'The key may only call models for: image generation, video generation, chat and writing.'
      )
    ).toBeInTheDocument()
    // Applying a template again is for a key that already has one.
    expect(screen.queryByRole('checkbox')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))
    // Which models there are to pick from is nobody's question here.
    expect(api.fetchOrgKeyModels).not.toHaveBeenCalled()

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Studio',
        policy_template: 'creative',
      })
    )
  })

  it('sends the quota, the rate limits and the monthly cap that were typed', async () => {
    // Acceptance: 创建 key 时可：… 设额度/RPM/TPM/月限额（复用现有 token
    // 配置能力）.
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Capped')
    enter(/^Quota \(/, '25.5')
    enter('Requests per minute', '60')
    enter('Tokens per minute', '90000')
    enter('Requests per month', '1000')
    // The expiry is a date picker that starts on "never".
    expect(
      screen.getByRole('button', { name: 'Never expires' })
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Capped',
        remain_quota: 12750000,
        rpm_limit: 60,
        tpm_limit: 90000,
        monthly_limit: 1000,
      })
    )
  })

  it('takes a key without a quota limit', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Open')
    await userEvent.click(
      screen.getByRole('switch', { name: 'No quota limit' })
    )
    expect(screen.queryByLabelText(/^Quota \(/)).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Open',
        unlimited_quota: true,
        remain_quota: 0,
      })
    )
  })

  it('keeps the form open when the backend refuses', async () => {
    api.createOrgKey.mockResolvedValue({ success: false, message: 'refused' })
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Nope')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(api.createOrgKey).toHaveBeenCalled())
    expect(screen.getByLabelText('Name')).toHaveValue('Nope')
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(api.fetchOrgKeys).toHaveBeenCalledTimes(1)
  })

  it('says so when there is nobody to create a key for', async () => {
    api.fetchOrgKeyHolders.mockResolvedValue([])
    await renderPage(customRole('Key Maker', ['key.read', 'key.create']))
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Stuck')

    expect(
      screen.getByText('There is nobody you can create a key for yet.')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()
  })
})

// PRD D34: a company can be too large to scroll through for the person a key
// is for. The list is searched by name and narrowed down by department and by
// role; all of it happens in the page, on the list the backend already cut down
// to whom the creator may make a key out to.
describe('finding who a key is for', () => {
  /** A member: staff of Sales unless said otherwise. */
  function holderOf(
    id: number,
    name: string,
    over: Partial<OrgKeyHolder> = {}
  ): OrgKeyHolder {
    return {
      id,
      name,
      department_id: 11,
      department: 'Sales',
      role_id: 4,
      role: 'staff',
      is_service: false,
      is_owner: false,
      ...over,
    }
  }

  const product = { department_id: 12, department: 'Product' }
  /** Six people in three departments, holding four roles. */
  const company = [
    ...keyHolders,
    holderOf(5, 'Mark', { role_id: 3, role: 'manager' }),
    holderOf(6, 'paula', product),
    holderOf(7, 'Samir', { ...product, role_id: 31, role: 'IT Ops' }),
  ]

  /** Opens the form of a new key on the given people, with their list open. */
  async function openListOf(holders: OrgKeyHolder[]) {
    api.fetchOrgKeyHolders.mockResolvedValue(holders)
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await openPeople()
  }

  it('finds a person by part of their name, and takes them on Enter', async () => {
    await openListOf(company)
    expect(search()).toHaveFocus()

    await userEvent.type(search(), 'SA')
    expect(people()).toEqual([
      'sally | Sales · Staff',
      'Samir | Product · IT Ops',
    ])
    await userEvent.type(search(), 'm')
    expect(people()).toEqual(['Samir | Product · IT Ops'])

    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('Samir · Product')
    await userEvent.type(screen.getByLabelText('Name'), 'Laptop')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Laptop',
        holder_id: 7,
      })
    )
  })

  it('narrows the list down to a department, to a role, and to both', async () => {
    await openListOf(company)
    // What there is to narrow down to is read off the people themselves.
    expect(await choicesOf('Department')).toEqual([
      'All departments',
      'General',
      'Sales',
      'Product',
    ])
    expect(await choicesOf('Role')).toEqual([
      'All roles',
      'Owner',
      'Manager',
      'Staff',
      'IT Ops',
    ])

    await narrow('Department', 'Sales')
    expect(people()).toEqual([
      'Mark | Sales · Manager',
      'sally | Sales · Staff',
    ])
    await narrow('Role', 'Staff')
    expect(people()).toEqual(['sally | Sales · Staff'])

    await narrow('Department', 'All departments')
    expect(people()).toEqual([
      'CI Pipeline | Service account | General · Staff',
      'paula | Product · Staff',
      'sally | Sales · Staff',
    ])
    // A name on top of a filter.
    await userEvent.type(search(), 'p')
    expect(people()).toEqual([
      'CI Pipeline | Service account | General · Staff',
      'paula | Product · Staff',
    ])
    expect(peopleOpen()).toBe(true)
  })

  it('says so when nobody is left', async () => {
    await openListOf(company)
    await narrow('Department', 'Product')
    await narrow('Role', 'Manager')

    expect(people()).toEqual([])
    expect(screen.getByText('Nobody matches.')).toBeInTheDocument()
    // Enter has nobody to pick.
    await userEvent.click(search())
    await userEvent.keyboard('{Enter}')
    expect(peopleOpen()).toBe(true)
    expect(dropdown('Who it is for')).toHaveTextContent(
      'Fiona Founder (owner — not handed out yet)'
    )
  })

  it('starts the search afresh after a pick, and keeps the filters', async () => {
    await openListOf(company)
    await narrow('Department', 'Sales')
    await userEvent.type(search(), 'sal')
    await userEvent.click(person('sally'))
    await waitFor(() => expect(peopleOpen()).toBe(false))

    await openPeople()
    expect(search()).toHaveValue('')
    expect(dropdown('Department')).toHaveTextContent('Sales')
    expect(people()).toEqual([
      'Mark | Sales · Manager',
      'sally | Sales · Staff',
    ])
  })

  it('opens with the highlight on whoever the form is on', async () => {
    // Someone who cannot park keys under the owner starts on the first member
    // the backend lists — who is not the first by name.
    await openListOf([holderOf(3, 'sally'), holderOf(5, 'Mark')])

    expect(people()).toEqual(['Mark | Sales · Staff', 'sally | Sales · Staff'])
    // One department and one role between them: nothing to narrow down by.
    expect(screen.queryByRole('combobox', { name: 'Department' })).toBeNull()
    expect(screen.queryByRole('combobox', { name: 'Role' })).toBeNull()
    expect(highlighted()).toBe('sally')
    expect(person('sally').dataset.checked).toBe('true')
    // Enter straight away changes nothing.
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('sally · Sales')
  })

  it('opens again on whoever was picked, wherever the search left the highlight', async () => {
    await openListOf(company)
    await userEvent.type(search(), 'sal')
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))

    // The search is empty again and the list whole; the highlight is not on
    // its first row but on her.
    await openPeople()
    expect(people()).toHaveLength(6)
    expect(highlighted()).toBe('sally')
    expect(person('sally').dataset.checked).toBe('true')
  })

  it('moves through the list with the arrow keys', async () => {
    await openListOf(company)
    expect(highlighted()).toBe('Fiona Founder')

    await userEvent.keyboard('{ArrowDown}{ArrowDown}')
    expect(highlighted()).toBe('Mark')
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('Mark · Sales')
  })

  it('keeps Enter on a row once a filter took the highlighted one off the list', async () => {
    await openListOf(company)
    await userEvent.keyboard('{ArrowDown}')
    expect(highlighted()).toBe('CI Pipeline')

    await narrow('Department', 'Sales')
    expect(highlighted()).toBe('Mark')
    await userEvent.click(search())
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('Mark · Sales')
  })

  it('leaves the keys pressed on a filter to the filter', async () => {
    await openListOf(company)
    await userEvent.keyboard('{ArrowDown}')
    expect(highlighted()).toBe('CI Pipeline')

    // Enter on a filter opens the filter. It does not pick the highlighted
    // person, and the arrow keys move through the filter, not through them.
    dropdown('Department').focus()
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(filterChoices()).not.toEqual([]))
    expect(peopleOpen()).toBe(true)
    await userEvent.keyboard('{ArrowDown}')
    expect(highlighted()).toBe('CI Pipeline')

    // Escape closes the filter, then the list, then the form: one at a time.
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(filterChoices()).toEqual([]))
    expect(peopleOpen()).toBe(true)
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(screen.getByLabelText('Name')).toBeInTheDocument()
    expect(dropdown('Who it is for')).toHaveTextContent(
      'Fiona Founder (owner — not handed out yet)'
    )
  })

  it('offers no filter that has nothing to choose between', async () => {
    // One department, two roles: only the role narrows anything down.
    await openListOf([
      holderOf(3, 'sally'),
      holderOf(5, 'Mark', { role_id: 3, role: 'manager' }),
    ])

    expect(screen.queryByRole('combobox', { name: 'Department' })).toBeNull()
    expect(await choicesOf('Role')).toEqual(['All roles', 'Manager', 'Staff'])
    await narrow('Role', 'Manager')
    expect(people()).toEqual(['Mark | Sales · Manager'])
  })

  it('shows the first hundred of a large company and says how many there are', async () => {
    const crowd = Array.from({ length: 250 }, (_, i) =>
      holderOf(100 + i, `Employee ${String(i + 1).padStart(3, '0')}`)
    )
    await openListOf([...keyHolders, ...crowd])

    expect(people()).toHaveLength(100)
    expect(people()[0]).toBe(
      'Fiona Founder | Not handed out yet | General · Owner'
    )
    expect(
      screen.getByText(
        'Showing the first 100 of 253. Search or filter to narrow it down.'
      )
    ).toBeInTheDocument()

    // Still too many after a filter: it counts the people left, not everyone.
    await narrow('Department', 'Sales')
    expect(people()).toHaveLength(100)
    expect(
      screen.getByText(
        'Showing the first 100 of 251. Search or filter to narrow it down.'
      )
    ).toBeInTheDocument()

    // Narrowed down to what fits, the list is whole and says nothing.
    await userEvent.type(search(), 'employee 25')
    expect(people()).toEqual(['Employee 250 | Sales · Staff'])
    expect(screen.queryByText(/^Showing the first/)).toBeNull()
    // Someone past the first hundred is found all the same.
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('Employee 250 · Sales')
  })

  it('keeps whoever the form is on in the list of a large company', async () => {
    const crowd = Array.from({ length: 250 }, (_, i) =>
      holderOf(100 + i, `Employee ${String(i + 1).padStart(3, '0')}`)
    )
    await openListOf([...keyHolders, ...crowd])
    await userEvent.type(search(), 'employee 250')
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))

    // Far past the first hundred by name, and still there: one more row at
    // the end, ticked, and where the list opens.
    await openPeople()
    expect(people()).toHaveLength(101)
    expect(people()[100]).toBe('Employee 250 | Sales · Staff')
    expect(person('Employee 250').dataset.checked).toBe('true')
    expect(highlighted()).toBe('Employee 250')
    expect(
      screen.getByText(
        'Showing the first 100 of 253. Search or filter to narrow it down.'
      )
    ).toBeInTheDocument()

    // A filter that leaves them out leaves them out.
    await narrow('Department', 'General')
    expect(people()).toEqual([
      'Fiona Founder | Not handed out yet | General · Owner',
      'CI Pipeline | Service account | General · Staff',
    ])
    await narrow('Department', 'All departments')

    // Enter straight away changes nothing.
    await userEvent.click(search())
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(peopleOpen()).toBe(false))
    expect(dropdown('Who it is for')).toHaveTextContent('Employee 250 · Sales')
  })

  it('says exactly a hundred people need no narrowing down', async () => {
    const crowd = Array.from({ length: 99 }, (_, i) =>
      holderOf(100 + i, `Employee ${String(i + 1).padStart(3, '0')}`)
    )
    await openListOf([keyHolders[0], ...crowd])

    expect(people()).toHaveLength(100)
    expect(screen.queryByText(/^Showing the first/)).toBeNull()
  })

  it('shows no role, and offers none to narrow down by, to someone who is told nobody’s', async () => {
    // What the backend answers a creator who may not read the member list.
    await openListOf(company.map((each) => ({ ...each, role_id: 0, role: '' })))

    expect(people()).toEqual([
      // Who the owner is, the form knows anyway.
      'Fiona Founder | Not handed out yet | General · Owner',
      'CI Pipeline | Service account | General',
      'Mark | Sales',
      'paula | Product',
      'sally | Sales',
      'Samir | Product',
    ])
    expect(screen.queryByRole('combobox', { name: 'Role' })).toBeNull()
    await narrow('Department', 'Product')
    expect(people()).toEqual(['paula | Product', 'Samir | Product'])
  })
})

describe('changing a key', () => {
  it('opens on what the key has and sends only what was changed', async () => {
    // PRD D31: with the personal endpoints closed to it, this is where a
    // key's settings are changed. The page's part is to send the change and
    // nothing it did not mean to change.
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Design tools' })
    )

    expect(screen.getByLabelText('Name')).toHaveValue('Design tools')
    expect(screen.getByText('Held by sally · Sales')).toBeInTheDocument()
    // Who holds a key is not changed here.
    expect(screen.queryByRole('combobox', { name: 'Who it is for' })).toBeNull()
    expect(dropdown('Policy template')).toHaveTextContent('Every model')
    expect(screen.getByLabelText(/^Quota left \(/)).toHaveValue(10)
    expect(
      screen.getByText(
        'Changes apply to the next request the key makes. Its value stays the same.'
      )
    ).toBeInTheDocument()

    await userEvent.clear(screen.getByLabelText('Name'))
    await userEvent.type(screen.getByLabelText('Name'), 'Design suite')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    // The quota is not sent back: the key may have spent some since.
    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(100, {
        name: 'Design suite',
      })
    )
    expect(mockToast.success).toHaveBeenCalledWith('Key updated')
    await waitFor(() => expect(api.fetchOrgKeys).toHaveBeenCalledTimes(2))
  })

  it('asks the backend for nothing when nothing was changed', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Design tools' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.queryByLabelText('Name')).toBeNull())
    expect(api.updateOrgKey).not.toHaveBeenCalled()
  })

  it('sends new limits and a new quota', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Design tools' })
    )
    enter(/^Quota left \(/, '2')
    enter('Requests per minute', '30')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(100, {
        remain_quota: 1000000,
        rpm_limit: 30,
      })
    )
  })

  it('applies a key’s template again only when asked', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Nightly build' })
    )

    expect(dropdown('Policy template')).toHaveTextContent('Coding pack')
    const again = screen.getByRole('checkbox', {
      name: /^Apply the template again/,
    })
    expect(again).not.toBeChecked()
    await userEvent.click(again)
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(101, {
        policy_template: 'coding',
      })
    )
  })

  it('has no "apply again" for a key without a template, or once another is picked', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Design tools' })
    )
    expect(screen.queryByRole('checkbox')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByLabelText('Name')).toBeNull())

    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Nightly build' })
    )
    expect(screen.getByRole('checkbox')).toBeInTheDocument()
    await choose('Policy template', 'Creative pack')
    expect(screen.queryByRole('checkbox')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(101, {
        policy_template: 'creative',
      })
    )
  })
})

describe('choosing the models by hand', () => {
  // PRD §5 (D33): 不套模板 = 不限制模型，或手动指定. The fourth choice of the
  // template list limits a key to models picked from what its holder can be
  // served.

  it('limits a new key to the models picked, and wants at least one', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Writers')
    // Nothing is asked about models until someone wants to pick.
    expect(screen.queryByRole('group', { name: 'Specific models' })).toBeNull()
    expect(api.fetchOrgKeyModels).not.toHaveBeenCalled()

    await choose('Policy template', 'Specific models')
    expect(
      screen.getByText('The key may only call the models you choose here.')
    ).toBeInTheDocument()
    // The key is parked under the owner, so the list is the owner's.
    await waitFor(() => expect(api.fetchOrgKeyModels).toHaveBeenCalledWith(1))
    expect(
      await screen.findByText('Choose at least one model.')
    ).toBeInTheDocument()
    // An empty hand-picked list would mean "every model" to the backend.
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()

    expect(await offeredModels()).toEqual(OFFERED)
    await pick('gpt-4o')
    await pick('claude-sonnet-5')
    expect(screen.getByText('2 model(s) selected')).toBeInTheDocument()
    // What was picked is no longer on offer.
    expect(await offeredModels()).toEqual(['gpt-image-1'])
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Writers',
        model_limits: ['gpt-4o', 'claude-sonnet-5'],
      })
    )
  })

  it('offers what the member the key is for can be served', async () => {
    api.fetchOrgKeyModels.mockImplementation((holderId: number) =>
      Promise.resolve(holderId === 3 ? ['sales-model'] : OFFERED)
    )
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await choose('Policy template', 'Specific models')
    expect(await offeredModels()).toEqual(OFFERED)

    await chooseHolder('sally')
    await waitFor(() => expect(api.fetchOrgKeyModels).toHaveBeenCalledWith(3))
    await waitFor(async () =>
      expect(await offeredModels()).toEqual(['sales-model'])
    )
  })

  it('keeps the picks while a template is tried, without asking for them', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Studio')
    await choose('Policy template', 'Specific models')
    await pick('gpt-image-1')

    await choose('Policy template', 'Creative pack')
    expect(screen.queryByRole('group', { name: 'Specific models' })).toBeNull()
    await choose('Policy template', 'Specific models')
    expect(await screen.findByText('1 model(s) selected')).toBeInTheDocument()
    expect(picker()).toHaveTextContent('gpt-image-1')

    // Back on the template, the picks stay behind: a key takes one or the other.
    await choose('Policy template', 'Creative pack')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() =>
      expect(api.createOrgKey).toHaveBeenCalledWith({
        ...UNTOUCHED,
        name: 'Studio',
        policy_template: 'creative',
      })
    )
  })

  it('spells a hand-picked list out in the table, for those who cannot open the form too', async () => {
    api.fetchOrgKeys.mockResolvedValue([pickedKey, buildKey])
    await renderPage(membershipOf('readonly'))

    const row = rowOf('Writers')
    expect(row).toHaveTextContent('2 model(s)')
    expect(row).toHaveTextContent('claude-sonnet-5, gpt-4o')
    // A template's name says what it allows; its list is not spelled out.
    expect(rowOf('Nightly build')).toHaveTextContent('Coding pack')
    expect(rowOf('Nightly build')).not.toHaveTextContent('model-a')
  })

  it('opens a hand-picked key on its list, and sends the list once it changes', async () => {
    api.fetchOrgKeys.mockResolvedValue([pickedKey])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))

    expect(dropdown('Policy template')).toHaveTextContent('Specific models')
    // Asked for the key's holder, not for whoever edits it.
    await waitFor(() => expect(api.fetchOrgKeyModels).toHaveBeenCalledWith(3))
    expect(await screen.findByText('2 model(s) selected')).toBeInTheDocument()
    expect(picker()).toHaveTextContent('claude-sonnet-5')
    expect(picker()).toHaveTextContent('gpt-4o')
    expect(screen.queryByRole('checkbox')).toBeNull()

    await pick('gpt-image-1')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(103, {
        model_limits: ['claude-sonnet-5', 'gpt-4o', 'gpt-image-1'],
      })
    )
  })

  it('takes a model out of the list, and does not let the last one go', async () => {
    api.fetchOrgKeys.mockResolvedValue([pickedKey])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))
    await screen.findByText('2 model(s) selected')

    const remove = () => within(picker()).getAllByRole('button')
    await userEvent.click(remove()[0])
    expect(screen.getByText('1 model(s) selected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    await userEvent.click(remove()[0])
    expect(screen.getByText('Choose at least one model.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()

    await pick('gpt-4o')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(103, {
        model_limits: ['gpt-4o'],
      })
    )
  })

  it('lifts a hand-picked list by going back to every model', async () => {
    api.fetchOrgKeys.mockResolvedValue([pickedKey])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))
    await choose('Policy template', 'Every model')
    expect(screen.queryByRole('group', { name: 'Specific models' })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(103, { model_limits: [] })
    )
  })

  it('swaps a hand-picked list for a template', async () => {
    api.fetchOrgKeys.mockResolvedValue([pickedKey])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))
    await choose('Policy template', 'Creative pack')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(103, {
        policy_template: 'creative',
        model_limits: [],
      })
    )
  })

  it('swaps a template for a hand-picked list', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Edit Nightly build' })
    )
    await choose('Policy template', 'Specific models')
    // There is no template left to apply again, and the list starts empty:
    // what the template had put on the key is rules, not picks.
    expect(screen.queryByRole('checkbox')).toBeNull()
    await waitFor(() => expect(api.fetchOrgKeyModels).toHaveBeenCalledWith(4))
    expect(
      await screen.findByText('Choose at least one model.')
    ).toBeInTheDocument()

    await pick('gpt-4o')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(101, {
        policy_template: '',
        model_limits: ['gpt-4o'],
      })
    )
  })

  it('sends nothing about models when only something else was changed', async () => {
    // A model the key is limited to already stays on show when its holder
    // can no longer be served it, and is not sent back unasked.
    api.fetchOrgKeys.mockResolvedValue([
      keyOf({ ...pickedKey, model_limits: ['retired-model', 'gpt-4o'] }),
    ])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))
    await screen.findByText('2 model(s) selected')
    expect(picker()).toHaveTextContent('retired-model')
    // It cannot be picked again once it is gone: it is not on offer.
    expect(await offeredModels()).toEqual(['claude-sonnet-5', 'gpt-image-1'])

    enter('Requests per minute', '30')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgKey).toHaveBeenCalledWith(103, { rpm_limit: 30 })
    )
  })

  it('closes the list of models on Escape, and the form only on the next', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Half done')
    await choose('Policy template', 'Specific models')
    expect(await offeredModels()).toEqual(OFFERED)

    await userEvent.keyboard('{Escape}')
    await waitFor(() =>
      expect(within(picker()).queryByRole('option')).toBeNull()
    )
    // A form half filled in is not thrown away with the list.
    expect(screen.getByLabelText('Name')).toHaveValue('Half done')

    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByLabelText('Name')).toBeNull())
  })

  it('lets Escape close the form from a picked model, where no list is open', async () => {
    // Only the open list takes an Escape for itself. Anywhere else in the
    // picker the key means what it means in the rest of the form.
    api.fetchOrgKeys.mockResolvedValue([pickedKey])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Edit Writers' }))
    await screen.findByText('2 model(s) selected')

    within(picker()).getAllByRole('button')[0].focus()
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByLabelText('Name')).toBeNull())
    expect(api.updateOrgKey).not.toHaveBeenCalled()
  })

  it('says so when the models cannot be loaded', async () => {
    api.fetchOrgKeyModels.mockRejectedValue(new Error('network'))
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Stuck')
    await choose('Policy template', 'Specific models')

    expect(
      await screen.findByText('Could not load the models to choose from.')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()
    // Every other way to make the key is still there.
    await choose('Policy template', 'Every model')
    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()
  })

  it('says so when the holder can be served no model at all', async () => {
    api.fetchOrgKeyModels.mockResolvedValue([])
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'New key' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Stuck')
    await choose('Policy template', 'Specific models')

    expect(
      await screen.findByText(
        'There is no model the holder of this key can use right now.'
      )
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()
  })
})

describe('giving a key a new value', () => {
  it('asks first, and shows nothing for a person’s key', async () => {
    // Acceptance: 一键 rotation：key 值更换、配置/归属/历史用量保留、旧值
    // 立即失效. The page asks first and shows no value for a person's key.
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Replace the value of Design tools' })
    )

    const confirm = screen.getByRole('alertdialog')
    expect(confirm).toHaveTextContent('Give this key a new value?')
    expect(confirm).toHaveTextContent(
      'The current value of “Design tools” stops working at once. The new one is not shown: sally runs one-click setup again to go on using it.'
    )
    expect(api.rotateOrgKey).not.toHaveBeenCalled()

    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Replace the value' })
    )
    await waitFor(() => expect(api.rotateOrgKey).toHaveBeenCalledWith(100))
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith('The key has a new value')
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(screen.queryByRole('textbox')).toBeNull()
    await waitFor(() => expect(api.fetchOrgKeys).toHaveBeenCalledTimes(2))
  })

  it('shows a service account’s new value once', async () => {
    api.rotateOrgKey.mockResolvedValue({
      success: true,
      data: { ...buildKey, value: SECRET },
    })
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Replace the value of Nightly build' })
    )
    expect(screen.getByRole('alertdialog')).toHaveTextContent(
      'The current value of “Nightly build” stops working at once. The new one is shown to you once — have the place that uses it ready to be updated.'
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Replace the value' })
    )

    const field = await screen.findByRole('textbox', { name: 'Full key' })
    expect(field).toHaveValue(`sk-${SECRET}`)
    const shown = field.closest('[role="alertdialog"]') as HTMLElement
    expect(shown).toHaveTextContent('The key has a new value')
    // A line of confirmation would be one more place to miss the value from.
    expect(mockToast.success).not.toHaveBeenCalled()

    await userEvent.click(
      within(shown).getByRole('button', { name: 'I have saved it' })
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(document.body.innerHTML).not.toContain(SECRET)
  })

  it('does nothing when the question is answered with no', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Replace the value of Design tools' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(api.rotateOrgKey).not.toHaveBeenCalled()
  })
})

describe('freezing and deleting', () => {
  it('freezes in one click, with no question asked', async () => {
    // Acceptance: 一键吊销（冻结）与解冻. Freezing is what to do first with a
    // key that may have leaked, and it can be undone.
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Freeze Design tools' })
    )

    await waitFor(() => expect(api.freezeOrgKey).toHaveBeenCalledWith(100))
    expect(screen.queryByRole('alertdialog')).toBeNull()
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith('Key frozen')
    )
    await waitFor(() => expect(api.fetchOrgKeys).toHaveBeenCalledTimes(2))
  })

  it('unfreezes a frozen key', async () => {
    await renderPage()
    expect(
      screen.queryByRole('button', { name: 'Freeze Old laptop' })
    ).toBeNull()
    await userEvent.click(
      screen.getByRole('button', { name: 'Unfreeze Old laptop' })
    )

    await waitFor(() => expect(api.unfreezeOrgKey).toHaveBeenCalledWith(102))
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith('Key unfrozen')
    )
    expect(api.freezeOrgKey).not.toHaveBeenCalled()
  })

  it('holds a key’s buttons while an action on it is running', async () => {
    let finish: (answer: { success: boolean }) => void = () => {}
    api.freezeOrgKey.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve
      })
    )
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Freeze Design tools' })
    )

    for (const button of within(rowOf('Design tools')).getAllByRole('button')) {
      expect(button).toBeDisabled()
    }
    // Only that key waits.
    expect(
      screen.getByRole('button', { name: 'Edit Nightly build' })
    ).toBeEnabled()

    finish({ success: true })
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Edit Design tools' })
      ).toBeEnabled()
    )
  })

  it('claims nothing when the backend refuses', async () => {
    api.freezeOrgKey.mockResolvedValue({ success: false, message: 'refused' })
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Freeze Design tools' })
    )

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Freeze Design tools' })
      ).toBeEnabled()
    )
    expect(mockToast.success).not.toHaveBeenCalled()
    expect(api.fetchOrgKeys).toHaveBeenCalledTimes(1)
  })

  it('survives a request that fails outright', async () => {
    // A 403 reaches the page as a rejection; the interceptor has said why.
    api.freezeOrgKey.mockRejectedValue(new Error('403'))
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Freeze Design tools' })
    )

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Freeze Design tools' })
      ).toBeEnabled()
    )
    expect(mockToast.success).not.toHaveBeenCalled()
  })

  it('says what deleting means, and deletes only on confirm', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete Design tools' })
    )

    const confirm = screen.getByRole('alertdialog')
    expect(confirm).toHaveTextContent('Delete this key?')
    expect(confirm).toHaveTextContent(
      '“Design tools” stops working at once and cannot be brought back. What it has spent stays in the usage records.'
    )
    expect(api.deleteOrgKey).not.toHaveBeenCalled()

    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Delete' })
    )
    await waitFor(() => expect(api.deleteOrgKey).toHaveBeenCalledWith(100))
    await waitFor(() =>
      expect(mockToast.success).toHaveBeenCalledWith('Key deleted')
    )
    await waitFor(() => expect(api.fetchOrgKeys).toHaveBeenCalledTimes(2))
  })

  it('keeps the key when the question is answered with no', async () => {
    await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete Design tools' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(api.deleteOrgKey).not.toHaveBeenCalled()
  })
})

describe('loading failures', () => {
  it('shows an error with a retry instead of an empty page', async () => {
    api.fetchOrgMembership.mockResolvedValue(membershipOf('owner'))
    api.fetchOrgKeys.mockRejectedValue(new Error('network'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrgKeysPage />
      </QueryClientProvider>
    )

    expect(
      await screen.findByText('Could not load your organization.')
    ).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
    // Nothing to create a key from while the page could not load.
    expect(screen.getByRole('button', { name: 'New key' })).toBeDisabled()

    api.fetchOrgKeys.mockResolvedValue(keys)
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Design tools')).toBeVisible()
  })
})
