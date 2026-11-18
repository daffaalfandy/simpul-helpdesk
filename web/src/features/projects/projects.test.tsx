import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { ProjectDodInput, ProjectSla } from '../../api/client'
import { PROJECT_IDS, projectsApi } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null
const AGENT = USERS['rina@demo-vendor.test'] ?? null
const REQUESTER = USERS['hendra@kominfo-jambi.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...projectsApi.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

async function openProjects(url = '/projects') {
  const user = userEvent.setup()
  const view = renderApp(url)
  await screen.findByRole('heading', { name: 'Service level (SLA) · e-Office' })
  return { user, ...view }
}

const slaCard = () => screen.getByRole('region', { name: /^Service level \(SLA\)/ })
const dodCard = () => screen.getByRole('region', { name: 'Definition of Done' })

describe('projects', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('opens the first project and lists every project with its client', async () => {
    const { router } = await openProjects()
    expect(router.state.location.pathname).toBe(`/projects/${PROJECT_IDS.eOffice}`)
    const list = screen.getByRole('navigation', { name: 'Projects' })
    const eOffice = within(list).getByRole('link', { name: /e-Office/ })
    expect(eOffice).toHaveAttribute('aria-current', 'page')
    expect(eOffice).toHaveTextContent('Dinas Kominfo Jambi')
    expect(eOffice).toHaveTextContent('Default')
    expect(within(list).getByRole('link', { name: /Email/ })).not.toHaveTextContent('Default')
    expect(within(list).getAllByRole('link')).toHaveLength(3)
    expect(eOffice).toHaveTextContent('No repo')
    expect(screen.getByText('No repository')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Manage connection' })).toHaveAttribute('href', '/settings/integrations')

    const card = slaCard()
    const p1 = within(card).getByRole('row', { name: /Critical/ })
    expect(within(p1).getAllByRole('cell').map((c) => c.textContent)).toEqual(['15 min', '2 hours', '2 hours', '24/7'])
    const p3 = within(card).getByRole('row', { name: /Standard/ })
    expect(p3).toHaveTextContent('2 working days')
    expect(card).toHaveTextContent('Waiting on client, In UAT')
    expect(card).toHaveTextContent('50% of the time used')
    expect(card).toHaveTextContent('Supervisor, Client PIC (P1)')
  })

  it('edits the SLA, blocks a first response longer than the fix time, and saves', async () => {
    const { user } = await openProjects()
    await user.click(within(slaCard()).getByRole('button', { name: 'Edit SLA' }))
    const p1 = within(slaCard()).getByRole('group', { name: /Critical/ })
    const firstResponse = within(p1).getByRole('group', { name: 'First response within' })
    const amount = within(firstResponse).getByRole('textbox', { name: 'Amount' })
    expect(amount).toHaveFocus()

    await user.clear(amount)
    await user.type(amount, '3')
    await user.click(within(firstResponse).getByRole('button', { name: /Unit/ }))
    await user.click(await screen.findByRole('option', { name: 'h' }))
    expect(within(p1).getByRole('alert')).toHaveTextContent('First response must be shorter than the resolve time.')
    expect(within(slaCard()).getByRole('button', { name: 'Save SLA' })).toBeDisabled()

    // The number commits when the field loses focus.
    await user.clear(amount)
    await user.type(amount, '1')
    await user.tab()
    expect(within(p1).queryByRole('alert')).not.toBeInTheDocument()
    await user.click(within(slaCard()).getByRole('checkbox', { name: /In UAT/ }))
    await user.click(within(slaCard()).getByRole('button', { name: 'Save SLA' }))

    expect(await screen.findByRole('status')).toHaveTextContent(
      'SLA saved. New targets apply to tickets created from now on; open tickets keep the targets they started with.',
    )
    const body = lastRequest('PUT', `/projects/${PROJECT_IDS.eOffice}/sla`)?.body as ProjectSla
    expect(body.targets[0]?.firstResponse).toEqual({ value: 1, unit: 'h' })
    expect(body.pauseOnUat).toBe(false)
    expect(within(slaCard()).getByRole('button', { name: 'Edit SLA' })).toBeInTheDocument()
    expect(slaCard()).toHaveTextContent('Waiting on client')
  })

  it('throws edits away on Cancel and when switching projects', async () => {
    const { user, router } = await openProjects()
    await user.click(within(slaCard()).getByRole('button', { name: 'Edit SLA' }))
    await user.click(within(slaCard()).getByRole('button', { name: 'Cancel' }))
    expect(within(slaCard()).getByRole('button', { name: 'Edit SLA' })).toHaveFocus()

    await user.click(within(slaCard()).getByRole('button', { name: 'Edit SLA' }))
    await user.click(within(screen.getByRole('navigation', { name: 'Projects' })).getByRole('link', { name: /Email/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/projects/${PROJECT_IDS.email}`))
    expect(await screen.findByRole('heading', { name: 'Service level (SLA) · Email' })).toBeInTheDocument()
    expect(within(slaCard()).getByRole('button', { name: 'Edit SLA' })).toBeInTheDocument()
    expect(lastRequest('PUT', '/sla')).toBeUndefined()
  })

  it("shows each project's Definition of Done in the list and on its page", async () => {
    await openProjects()
    const list = screen.getByRole('navigation', { name: 'Projects' })
    expect(within(list).getByRole('link', { name: /e-Office/ })).toHaveTextContent('UAT')
    expect(within(list).getByRole('link', { name: /Email/ })).toHaveTextContent('Resolved by hand')
    expect(within(list).getByRole('link', { name: /Network/ })).toHaveTextContent('Requester confirms')

    const card = dodCard()
    expect(card).toHaveTextContent('Done means: the client approves UAT (auto-approved after 3 working days).')
    const flow = within(card).getByRole('list', { name: 'Status flow' })
    expect(within(flow).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['In progress', 'In UAT', 'Resolved'])
    expect(card).toHaveTextContent('Agents may set a different Definition of Done on a single ticket, with a reason.')
  })

  it('edits the Definition of Done with a live preview and saves it', async () => {
    const { user } = await openProjects()
    expect(dodCard()).toHaveTextContent('Required before resolving: skipping it needs a reason, and the supervisors are told.')
    await user.click(within(dodCard()).getByRole('button', { name: 'Edit DoD' }))
    // Code on a branch, the client's two checks, then the two switches. Without a repository the
    // code condition can't be turned on, and the row says why.
    expect(within(dodCard()).getAllByRole('checkbox')).toHaveLength(5)
    expect(within(dodCard()).queryByRole('checkbox', { name: /merged/i })).not.toBeInTheDocument()
    expect(within(dodCard()).getByRole('checkbox', { name: /Code reached a branch/ })).toBeDisabled()
    expect(within(dodCard()).getByRole('textbox', { name: 'Branch' })).toBeDisabled()
    expect(dodCard()).toHaveTextContent('Link a repository to this project first, in Settings › GitHub / GitLab.')
    expect(within(dodCard()).getByRole('checkbox', { name: /Required before resolving/ })).toBeChecked()

    await user.click(within(dodCard()).getByRole('checkbox', { name: /Requester confirms it's fixed/ }))
    await user.click(within(dodCard()).getByRole('button', { name: /Auto-approve if no answer/ }))
    await user.click(await screen.findByRole('option', { name: /Never/ }))
    expect(dodCard()).toHaveTextContent('Done means: the client approves UAT and the requester confirms it is fixed.')
    await user.click(within(dodCard()).getByRole('checkbox', { name: /Let agents set a different DoD/ }))
    await user.click(within(dodCard()).getByRole('button', { name: 'Save DoD' }))

    expect(await screen.findByRole('status')).toHaveTextContent(
      "Definition of Done saved. It applies to this project's open tickets now, except tickets with their own.",
    )
    const body = lastRequest('PUT', `/projects/${PROJECT_IDS.eOffice}/dod`)?.body as ProjectDodInput
    expect(body.dod.requesterConfirms).toBe(true)
    expect(body.dod.uat).toEqual({ enabled: true, autoApproveWorkingDays: 0 })
    expect(body.allowTicketOverride).toBe(false)
    expect(body.mandatory).toBe(true)
    expect(body.dod.codeOnBranch.enabled).toBe(false)
    expect(dodCard()).toHaveTextContent('Only supervisors and admins may set a different Definition of Done on a ticket.')
  })

  it('asks for code on a branch when the project has a repository, and makes the DoD guidance only', async () => {
    const eOffice = projectsApi.projects.find((p) => p.id === PROJECT_IDS.eOffice)
    if (eOffice) {
      eOffice.repository = { id: 'repo-1', provider: 'github', fullName: 'vendor-id/eoffice-jambi', webUrl: 'https://github.com/vendor-id/eoffice-jambi' }
    }
    const { user } = await openProjects()
    await user.click(within(dodCard()).getByRole('button', { name: 'Edit DoD' }))
    const code = within(dodCard()).getByRole('checkbox', { name: /Code reached a branch/ })
    expect(code).toBeEnabled()
    await user.click(code)
    const branch = within(dodCard()).getByRole('textbox', { name: 'Branch' })

    // A name Git refuses: the API says so and the field is marked.
    await user.type(branch, 'fix me')
    await user.click(within(dodCard()).getByRole('button', { name: 'Save DoD' }))
    expect(await within(dodCard()).findByText(/Enter a valid branch name, such as main/)).toBeInTheDocument()
    expect(branch).toHaveAttribute('aria-invalid', 'true')

    await user.clear(branch)
    await user.type(branch, 'main')
    expect(dodCard()).toHaveTextContent('Done means: a commit naming the ticket reaches main and the client approves UAT')
    await user.click(within(dodCard()).getByRole('checkbox', { name: /Required before resolving/ }))
    await user.click(within(dodCard()).getByRole('button', { name: 'Save DoD' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Definition of Done saved.')
    const body = lastRequest('PUT', `/projects/${PROJECT_IDS.eOffice}/dod`)?.body as ProjectDodInput
    expect(body.dod.codeOnBranch).toEqual({ enabled: true, branch: 'main' })
    expect(body.mandatory).toBe(false)
    expect(dodCard()).toHaveTextContent('Guidance only: anyone can resolve a ticket without meeting it.')
    const list = screen.getByRole('navigation', { name: 'Projects' })
    expect(within(list).getByRole('link', { name: /e-Office/ })).toHaveTextContent('Code on main + UAT')
  })

  it('says when code on a branch is paused because the project lost its repository', async () => {
    const eOffice = projectsApi.projects.find((p) => p.id === PROJECT_IDS.eOffice)
    if (eOffice) eOffice.dod = { ...eOffice.dod, codeOnBranch: { enabled: true, branch: 'main' } }
    const { user } = await openProjects()
    expect(dodCard()).toHaveTextContent("Code on main doesn't apply until a repository is linked to this project.")
    expect(dodCard()).not.toHaveTextContent('reaches main')
    await user.click(within(dodCard()).getByRole('button', { name: 'Edit DoD' }))
    // The form starts with it off, so saving other changes still works.
    expect(within(dodCard()).getByRole('checkbox', { name: /Code reached a branch/ })).not.toBeChecked()
    expect(within(dodCard()).getByRole('textbox', { name: 'Branch' })).toHaveValue('main')
  })

  it('adds a project and opens it', async () => {
    const { user, router } = await openProjects()
    await user.click(screen.getByRole('button', { name: 'New project' }))
    const dialog = await screen.findByRole('dialog', { name: 'New project' })
    await user.click(within(dialog).getByRole('button', { name: 'Add project' }))
    expect(within(dialog).getByText('Choose a client.')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /Client/ }))
    await user.click(await screen.findByRole('option', { name: 'BPKAD Kota Batam' }))
    await user.type(within(dialog).getByLabelText('Name'), 'Finance apps')
    await user.type(within(dialog).getByLabelText('Description'), 'Budgeting and finance systems')
    await user.click(within(dialog).getByRole('button', { name: 'Add project' }))

    expect(await screen.findByRole('heading', { name: 'Service level (SLA) · Finance apps' })).toBeInTheDocument()
    expect(lastRequest('POST', '/projects')?.body).toEqual({
      clientId: IDS.batam,
      name: 'Finance apps',
      description: 'Budgeting and finance systems',
      isDefault: false,
      active: true,
    })
    expect(router.state.location.pathname).not.toBe(`/projects/${PROJECT_IDS.eOffice}`)
  })

  it('shows agents the projects read-only', async () => {
    state.me = AGENT
    await openProjects()
    expect(screen.getByRole('navigation', { name: 'Main' })).toHaveTextContent('Projects')
    expect(within(slaCard()).queryByRole('button', { name: 'Edit SLA' })).not.toBeInTheDocument()
    expect(within(dodCard()).queryByRole('button', { name: 'Edit DoD' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'New project' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit project' })).not.toBeInTheDocument()
  })

  it('keeps requesters out', async () => {
    state.me = REQUESTER
    const { router } = renderApp('/projects')
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal'))
  })
})
