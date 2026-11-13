import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { ProjectSla } from '../../api/client'
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

describe('projects', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('opens the first project and lists every project with its P1 fix time', async () => {
    const { router } = await openProjects()
    expect(router.state.location.pathname).toBe(`/projects/${PROJECT_IDS.eOffice}`)
    const list = screen.getByRole('navigation', { name: 'Projects' })
    const eOffice = within(list).getByRole('link', { name: /e-Office/ })
    expect(eOffice).toHaveAttribute('aria-current', 'page')
    expect(eOffice).toHaveTextContent('P1 fix 2 h')
    expect(eOffice).toHaveTextContent('Default')
    expect(within(list).getAllByRole('link')).toHaveLength(3)

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
    expect(screen.queryByRole('button', { name: 'New project' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit project' })).not.toBeInTheDocument()
  })

  it('keeps requesters out', async () => {
    state.me = REQUESTER
    const { router } = renderApp('/projects')
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal'))
  })
})
