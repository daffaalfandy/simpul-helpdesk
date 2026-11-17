import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { members } from '../../test/membersApi'
import { state, USERS } from '../../test/server'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

async function teamRow(name: string) {
  const cell = await screen.findByRole('rowheader', { name: new RegExp(`^${name}`) })
  return within(cell.closest('tr') as HTMLElement)
}

describe('Settings › Members & roles', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('counts each role and lists the team, with your own row read-only', async () => {
    renderApp('/settings/members')
    expect(
      await screen.findByText('3 team members and 46 client requesters in this organization. Administrators work in every organization; client requesters are invited from each client.'),
    ).toBeInTheDocument()
    const agent = within(screen.getByRole('region', { name: 'Agent' }))
    expect(agent.getByText('2 users')).toBeInTheDocument()
    expect(agent.getByText('Work tickets for their clients and tickets assigned to them')).toBeInTheDocument()
    // Suspended people don't count.
    expect(within(screen.getByRole('region', { name: 'Supervisor' })).getByText('0 users')).toBeInTheDocument()

    const me = await teamRow('Demo Admin')
    expect(me.getByText('(you)')).toBeInTheDocument()
    expect(me.queryByRole('button', { name: /Role for/ })).not.toBeInTheDocument()
    expect(me.queryByRole('button', { name: /Actions for/ })).not.toBeInTheDocument()

    const yoga = await teamRow('Yoga Hidayat')
    expect(yoga.getByText('Invited')).toBeInTheDocument()
    expect(yoga.getByText('Invite sent 14 Nov 2026')).toBeInTheDocument()
  })

  it('invites a team member by email', async () => {
    const user = userEvent.setup()
    renderApp('/settings/members')
    await user.click(await screen.findByRole('button', { name: 'Invite member' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Invite a team member' }))
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))
    expect(dialog.getByText('Enter their name.')).toBeInTheDocument()

    await user.type(dialog.getByRole('textbox', { name: 'Name' }), 'Putri Ayu')
    await user.type(dialog.getByRole('textbox', { name: 'Work email' }), 'putri@demo-vendor.test')
    await user.click(dialog.getByRole('button', { name: /^Agent Role/ }))
    await user.click(await screen.findByRole('option', { name: /^Supervisor/ }))
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Invite sent to putri@demo-vendor.test.')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const putri = await teamRow('Putri Ayu')
    expect(putri.getByText('Invited')).toBeInTheDocument()
    expect(members.team.at(-1)).toMatchObject({ email: 'putri@demo-vendor.test', role: 'supervisor' })
  })

  it('shows an email that is already taken', async () => {
    const user = userEvent.setup()
    renderApp('/settings/members')
    await user.click(await screen.findByRole('button', { name: 'Invite member' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Invite a team member' }))
    await user.type(dialog.getByRole('textbox', { name: 'Name' }), 'Rina again')
    await user.type(dialog.getByRole('textbox', { name: 'Work email' }), 'rina@demo-vendor.test')
    await user.click(dialog.getByRole('button', { name: 'Send invite' }))
    expect(await dialog.findByText('Someone already uses this email address.')).toBeInTheDocument()
  })

  it('changes a role from the row', async () => {
    const user = userEvent.setup()
    renderApp('/settings/members')
    const rina = await teamRow('Rina Pratiwi')
    await user.click(rina.getByRole('button', { name: /Role for Rina Pratiwi$/ }))
    await user.click(await screen.findByRole('option', { name: 'Supervisor' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Rina Pratiwi’s role changed.')
    expect(members.actions).toEqual(['role:Rina Pratiwi:supervisor'])
  })

  it('suspends after confirming, reactivates and resends invites', async () => {
    const user = userEvent.setup()
    renderApp('/settings/members')

    await user.click((await teamRow('Rina Pratiwi')).getByRole('button', { name: 'Actions for Rina Pratiwi' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Suspend account' }))
    const confirm = within(await screen.findByRole('alertdialog', { name: 'Suspend Rina Pratiwi?' }))
    expect(confirm.getByText(/Tickets assigned to them stay theirs/)).toBeInTheDocument()
    await user.click(confirm.getByRole('button', { name: 'Suspend account' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Rina Pratiwi is suspended and signed out of every organization.')
    expect((await teamRow('Rina Pratiwi')).getByText('Suspended')).toBeInTheDocument()

    await user.click((await teamRow('Bayu Nugroho')).getByRole('button', { name: 'Actions for Bayu Nugroho' }))
    expect(screen.queryByRole('menuitem', { name: 'Suspend account' })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('menuitem', { name: 'Reactivate' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Bayu Nugroho can sign in again.')

    await user.click((await teamRow('Yoga Hidayat')).getByRole('button', { name: 'Actions for Yoga Hidayat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Resend invite' }))
    expect(await screen.findByRole('status')).toHaveTextContent('A new invite is on its way to Yoga Hidayat.')

    expect(members.actions).toEqual(['suspend:Rina Pratiwi', 'reactivate:Bayu Nugroho', 'invite:Yoga Hidayat'])
  })
})

describe('Settings › Members & roles › removing', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('takes a member off the organization after confirming', async () => {
    const user = userEvent.setup()
    renderApp('/settings/members')
    await user.click((await teamRow('Rina Pratiwi')).getByRole('button', { name: 'Actions for Rina Pratiwi' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove from organization' }))
    const confirm = within(await screen.findByRole('alertdialog', { name: 'Remove Rina Pratiwi from this organization?' }))
    await user.click(confirm.getByRole('button', { name: 'Remove from organization' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Rina Pratiwi no longer works in this organization.')
    expect(members.actions).toEqual(['remove:Rina Pratiwi'])
  })

  it('never offers to remove an administrator', async () => {
    const user = userEvent.setup()
    members.team.push({ ...members.team[0]!, id: '01a1cedf-0000-7000-8000-0000000000a9', name: 'Second Admin', email: 'two@demo-vendor.test' })
    renderApp('/settings/members')
    await user.click((await teamRow('Second Admin')).getByRole('button', { name: 'Actions for Second Admin' }))
    expect(await screen.findByRole('menuitem', { name: 'Suspend account' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Remove from organization' })).not.toBeInTheDocument()
  })
})
