import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { PROJECT_IDS } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { members } from '../../test/membersApi'
import { state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'
import { TICKET_IDS, tickets } from '../../test/ticketsApi'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null
const JAMBI = 'Dinas Kominfo Jambi'

async function openRequesters(client = JAMBI) {
  const user = userEvent.setup()
  renderApp('/settings/clients')
  await user.click(await screen.findByRole('button', { name: `Requesters of ${client}` }))
  const dialog = within(await screen.findByRole('dialog', { name: `Requesters at ${client}` }))
  return { user, dialog }
}

function requesterRow(dialog: ReturnType<typeof within>, name: string, client = JAMBI) {
  const list = dialog.getByRole('list', { name: `Requesters at ${client}` })
  return within(within(list).getByText(name).closest('li') as HTMLElement)
}

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

describe('Settings › Clients › Requesters', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('lists the client’s requesters with their status and projects', async () => {
    const { dialog } = await openRequesters()
    expect(await dialog.findByText('Hendra Gunawan')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Hendra Gunawan').getByText('Active')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Hendra Gunawan').getByText('hendra@kominfo-jambi.test · IT Officer')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Hendra Gunawan').getByText('Reports on e-Office and Email')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Dian Puspita').getByText('Invited')).toBeInTheDocument()
  })

  it('invites a requester by email to the projects they report on', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Hendra Gunawan')
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))
    expect(dialog.getByText('Enter their name.')).toBeInTheDocument()
    expect(dialog.getByText('Choose at least one project.')).toBeInTheDocument()

    const form = within(dialog.getByRole('group', { name: 'Invite a requester' }))
    await user.type(form.getByRole('textbox', { name: 'Name' }), 'Siti Rahma')
    await user.type(form.getByRole('textbox', { name: 'Work email' }), 'Siti@Kominfo-Jambi.test')
    await user.type(form.getByRole('textbox', { name: 'Job title' }), 'Staf TU')
    await user.click(form.getByRole('checkbox', { name: 'e-Office' }))
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))

    expect(await dialog.findByRole('status')).toHaveTextContent('Invite sent to Siti@Kominfo-Jambi.test.')
    expect(await dialog.findByText('Siti Rahma')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Siti Rahma').getByText('Invited')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Siti Rahma').getByText('Reports on e-Office')).toBeInTheDocument()
    expect(form.getByRole('textbox', { name: 'Name' })).toHaveValue('')
    expect(form.getByRole('checkbox', { name: 'e-Office' })).not.toBeChecked()
    expect(tickets.requesters[IDS.jambi]?.at(-1)).toMatchObject({
      email: 'siti@kominfo-jambi.test',
      jobTitle: 'Staf TU',
      projectIds: [PROJECT_IDS.eOffice],
    })
  })

  it('picks the only project of a client with one', async () => {
    const { user, dialog } = await openRequesters('BPKAD Kota Batam')
    const form = within(dialog.getByRole('group', { name: 'Invite a requester' }))
    expect(form.getByRole('checkbox', { name: 'Network & infra support' })).toBeChecked()
    await user.type(form.getByRole('textbox', { name: 'Name' }), 'Sri Lestari')
    await user.type(form.getByRole('textbox', { name: 'Work email' }), 'sri@batam.test')
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('Invite sent to sri@batam.test.')
    expect(lastRequest('POST', `/clients/${IDS.batam}/requesters`)?.body).toMatchObject({ projectIds: [PROJECT_IDS.network] })
  })

  it('changes which projects a requester reports on', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Hendra Gunawan')
    await user.click(requesterRow(dialog, 'Hendra Gunawan').getByRole('button', { name: 'Projects of Hendra Gunawan' }))
    const projects = within(await screen.findByRole('dialog', { name: 'Projects for Hendra Gunawan' }))
    expect(projects.getByRole('checkbox', { name: 'e-Office' })).toBeChecked()
    await user.click(projects.getByRole('checkbox', { name: 'Email' }))
    await user.click(projects.getByRole('button', { name: 'Save projects' }))

    expect(await dialog.findByRole('status')).toHaveTextContent('Projects for Hendra Gunawan saved.')
    expect(screen.queryByRole('dialog', { name: 'Projects for Hendra Gunawan' })).not.toBeInTheDocument()
    expect(await requesterRow(dialog, 'Hendra Gunawan').findByText('Reports on e-Office')).toBeInTheDocument()
    expect(lastRequest('PUT', `/requesters/${TICKET_IDS.hendra}/projects`)?.body).toEqual({ projectIds: [PROJECT_IDS.eOffice] })
  })

  it('asks before taking a requester off every project', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Dian Puspita')
    await user.click(requesterRow(dialog, 'Dian Puspita').getByRole('button', { name: 'Projects of Dian Puspita' }))
    const projects = within(await screen.findByRole('dialog', { name: 'Projects for Dian Puspita' }))
    await user.click(projects.getByRole('checkbox', { name: 'e-Office' }))
    await user.click(projects.getByRole('checkbox', { name: 'Email' }))
    await user.click(projects.getByRole('button', { name: 'Save projects' }))

    const confirm = within(await screen.findByRole('alertdialog', { name: 'Take Dian Puspita off every project?' }))
    expect(lastRequest('PUT', `/requesters/${TICKET_IDS.dian}/projects`)).toBeUndefined()
    await user.click(confirm.getByRole('button', { name: 'Remove all projects' }))

    expect(await dialog.findByRole('status')).toHaveTextContent('Projects for Dian Puspita saved.')
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    const row = requesterRow(dialog, 'Dian Puspita')
    expect(await row.findByText('No projects')).toBeInTheDocument()
    expect(row.getByText('Sees only the tickets they reported')).toBeInTheDocument()
    expect(lastRequest('PUT', `/requesters/${TICKET_IDS.dian}/projects`)?.body).toEqual({ projectIds: [] })
  })

  it('resends an invite, suspends after confirming and lets them back in', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Dian Puspita')

    await user.click(requesterRow(dialog, 'Dian Puspita').getByRole('button', { name: 'Actions for Dian Puspita' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Resend invite' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('A new invite is on its way to Dian Puspita.')

    await user.click(requesterRow(dialog, 'Hendra Gunawan').getByRole('button', { name: 'Actions for Hendra Gunawan' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Suspend account' }))
    const confirm = within(await screen.findByRole('alertdialog', { name: 'Suspend Hendra Gunawan?' }))
    await user.click(confirm.getByRole('button', { name: 'Suspend account' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('Hendra Gunawan is suspended and signed out of every organization.')
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    // Still listed for the administrator, so they can be let back in.
    expect(await requesterRow(dialog, 'Hendra Gunawan').findByText('Suspended')).toBeInTheDocument()

    await user.click(requesterRow(dialog, 'Hendra Gunawan').getByRole('button', { name: 'Actions for Hendra Gunawan' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Reactivate' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('Hendra Gunawan can sign in again.')

    expect(members.actions).toEqual(['invite:Dian Puspita', 'suspend:Hendra Gunawan', 'reactivate:Hendra Gunawan'])
  })
})
