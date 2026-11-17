import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { members } from '../../test/membersApi'
import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

describe('Settings › Organizations', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('lists the organizations and adds one', async () => {
    const user = userEvent.setup()
    renderApp('/settings/organizations')
    expect(await screen.findByRole('rowheader', { name: /Demo Vendor/ })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Add organization' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Add an organization' }))
    await user.type(dialog.getByRole('textbox', { name: 'Organisation name' }), 'PT Mitra Data')
    await user.type(dialog.getByRole('textbox', { name: 'Ticket ID prefix' }), 'hd')
    await user.click(dialog.getByRole('button', { name: 'Add organization' }))
    expect(await dialog.findByText('Another organization uses this ticket prefix.')).toBeInTheDocument()

    const prefix = dialog.getByRole('textbox', { name: 'Ticket ID prefix' })
    await user.clear(prefix)
    await user.type(prefix, 'md')
    await user.click(dialog.getByRole('button', { name: 'Add organization' }))
    expect(await screen.findByRole('status')).toHaveTextContent('PT Mitra Data added.')
    expect(await screen.findByRole('rowheader', { name: /PT Mitra Data/ })).toBeInTheDocument()
    expect(members.organizations.at(-1)).toMatchObject({ ticketPrefix: 'MD', timezone: 'Asia/Jakarta' })
  })
})
