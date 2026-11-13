import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'
import { TICKET_IDS, tickets } from '../../test/ticketsApi'

const AGENT = USERS['rina@demo-vendor.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

async function openDialog() {
  const user = userEvent.setup()
  renderApp('/board')
  await user.click(await screen.findByRole('button', { name: 'Log a ticket' }))
  const dialog = await screen.findByRole('dialog', { name: 'Log a ticket for a client' })
  return { user, dialog }
}

describe('board', () => {
  beforeEach(() => {
    state.me = AGENT
  })

  it('lists open tickets with links and filters them', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    const table = await screen.findByRole('table', { name: 'Tickets' })
    expect(within(table).getByRole('link', { name: 'HD-24871' })).toHaveAttribute('href', '/tickets/HD-24871')
    expect(within(table).queryByText('Install a PDF reader')).not.toBeInTheDocument()
    const sipd = within(table).getByText('SIPD export times out').closest('tr') as HTMLElement
    expect(within(sipd).getByText('Unassigned')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'My queue' }))
    await waitFor(() => expect(router.state.location.search).toEqual({ view: 'mine' }))
    expect(screen.getByRole('button', { name: 'My queue' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(lastRequest('GET', '/tickets')?.search).toContain('assignee=me'))
    await waitFor(() => expect(screen.queryByText('SIPD export times out')).not.toBeInTheDocument())
  })

  it('logs a ticket for an existing requester', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: 'Create ticket' }))
    expect(await within(dialog).findByText('Choose a client.')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    const requester = within(dialog).getByRole('combobox', { name: 'Requester' })
    await user.type(requester, 'hen')
    await user.click(await screen.findByRole('option', { name: /Hendra Gunawan/ }))
    await user.click(within(dialog).getByRole('button', { name: /Which system\?/ }))
    await user.click(await screen.findByRole('option', { name: /e-Office/ }))
    await user.type(within(dialog).getByRole('textbox', { name: 'Subject' }), 'Called about e-Office')
    await user.type(within(dialog).getByRole('textbox', { name: 'Details' }), 'Login fails.')
    await user.click(within(dialog).getByRole('button', { name: 'Create ticket' }))

    expect(await screen.findByText('HD-24892 created and assigned to you.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets')?.body).toEqual({
      clientId: IDS.jambi,
      requesterId: TICKET_IDS.hendra,
      channel: 'phone',
      type: 'incident',
      priority: 'P3',
      subject: 'Called about e-Office',
      description: 'Login fails.',
      systemId: TICKET_IDS.eoffice,
      attachmentIds: [],
      assignToMe: true,
    })
    expect(screen.getByRole('link', { name: 'Open HD-24892' })).toHaveAttribute('href', '/tickets/HD-24892')
  })

  it('adds a new requester inline', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    await user.type(within(dialog).getByRole('combobox', { name: 'Requester' }), 'Siti Rahma')
    await user.click(await screen.findByRole('option', { name: 'Add “Siti Rahma” as a new requester' }))

    const group = within(dialog).getByRole('group', { name: 'New requester' })
    expect(within(group).getByRole('textbox', { name: 'Name' })).toHaveValue('Siti Rahma')
    const email = within(group).getByRole('textbox', { name: 'Email' })
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    expect(await within(group).findByText('Enter a valid email address.')).toBeInTheDocument()

    await user.type(email, 'hendra@kominfo-jambi.test')
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    expect(await within(group).findByText('Someone already uses this email address.')).toBeInTheDocument()

    await user.clear(email)
    await user.type(email, 'siti@kominfo-jambi.test')
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    await waitFor(() => expect(within(dialog).getByRole('combobox', { name: 'Requester' })).toHaveValue('Siti Rahma'))
    expect(lastRequest('POST', `/clients/${IDS.jambi}/requesters`)?.body).toEqual({ name: 'Siti Rahma', email: 'siti@kominfo-jambi.test' })
  })

  it('clears the requester when the client changes', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    await user.type(within(dialog).getByRole('combobox', { name: 'Requester' }), 'dian')
    await user.click(await screen.findByRole('option', { name: /Dian Puspita/ }))
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /BPKAD Kota Batam/ }))
    expect(within(dialog).getByRole('combobox', { name: 'Requester' })).toHaveValue('')
  })
})
