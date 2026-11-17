import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { members } from '../../test/membersApi'
import { state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'
import { tickets } from '../../test/ticketsApi'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

async function openRequesters() {
  const user = userEvent.setup()
  renderApp('/settings/clients')
  await user.click(await screen.findByRole('button', { name: 'Requesters of Dinas Kominfo Jambi' }))
  const dialog = within(await screen.findByRole('dialog', { name: 'Requesters at Dinas Kominfo Jambi' }))
  return { user, dialog }
}

function requesterRow(dialog: ReturnType<typeof within>, name: string) {
  const list = dialog.getByRole('list', { name: 'Requesters at Dinas Kominfo Jambi' })
  return within(within(list).getByText(name).closest('li') as HTMLElement)
}

describe('Settings › Clients › Requesters', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('lists the client’s requesters with their status', async () => {
    const { dialog } = await openRequesters()
    expect(await dialog.findByText('Hendra Gunawan')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Hendra Gunawan').getByText('Active')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Hendra Gunawan').getByText('hendra@kominfo-jambi.test · IT Officer')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Dian Puspita').getByText('Invited')).toBeInTheDocument()
  })

  it('invites a requester by email', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Hendra Gunawan')
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))
    expect(dialog.getByText('Enter their name.')).toBeInTheDocument()

    const form = within(dialog.getByRole('group', { name: 'Invite a requester' }))
    await user.type(form.getByRole('textbox', { name: 'Name' }), 'Siti Rahma')
    await user.type(form.getByRole('textbox', { name: 'Work email' }), 'Siti@Kominfo-Jambi.test')
    await user.type(form.getByRole('textbox', { name: 'Job title' }), 'Staf TU')
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))

    expect(await dialog.findByRole('status')).toHaveTextContent('Invite sent to Siti@Kominfo-Jambi.test.')
    expect(await dialog.findByText('Siti Rahma')).toBeInTheDocument()
    expect(requesterRow(dialog, 'Siti Rahma').getByText('Invited')).toBeInTheDocument()
    expect(form.getByRole('textbox', { name: 'Name' })).toHaveValue('')
    expect(tickets.requesters[IDS.jambi]?.at(-1)).toMatchObject({ email: 'siti@kominfo-jambi.test', jobTitle: 'Staf TU' })
  })

  it('resends an invite, suspends after confirming and lets them back in', async () => {
    const { user, dialog } = await openRequesters()
    await dialog.findByText('Dian Puspita')

    await user.click(requesterRow(dialog, 'Dian Puspita').getByRole('button', { name: 'Actions for Dian Puspita' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Resend invite' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('A new invite is on its way to Dian Puspita.')

    await user.click(requesterRow(dialog, 'Hendra Gunawan').getByRole('button', { name: 'Actions for Hendra Gunawan' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Suspend' }))
    const confirm = within(await screen.findByRole('alertdialog', { name: 'Suspend Hendra Gunawan?' }))
    await user.click(confirm.getByRole('button', { name: 'Suspend' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('Hendra Gunawan is suspended and signed out everywhere.')
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    // Still listed for the administrator, so they can be let back in.
    expect(await requesterRow(dialog, 'Hendra Gunawan').findByText('Suspended')).toBeInTheDocument()

    await user.click(requesterRow(dialog, 'Hendra Gunawan').getByRole('button', { name: 'Actions for Hendra Gunawan' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Reactivate' }))
    expect(await dialog.findByRole('status')).toHaveTextContent('Hendra Gunawan can sign in again.')

    expect(members.actions).toEqual(['invite:Dian Puspita', 'suspend:Hendra Gunawan', 'reactivate:Hendra Gunawan'])
  })
})
