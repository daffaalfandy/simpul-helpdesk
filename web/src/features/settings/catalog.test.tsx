import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'
import { TICKET_IDS, tickets } from '../../test/ticketsApi'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null
const AGENT = USERS['rina@demo-vendor.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

describe('systems & catalogue', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('lists both lists under Service desk', async () => {
    renderApp('/settings/catalog')
    const nav = await screen.findByRole('navigation', { name: 'Settings sections' })
    expect(within(nav).getByRole('link', { name: 'Systems & catalogue' })).toHaveAttribute('aria-current', 'page')
    const systems = screen.getByRole('table', { name: 'Systems' })
    expect(within(systems).getByText('Old app').closest('tr')).toHaveTextContent('Hidden')
    const items = screen.getByRole('table', { name: 'Catalogue items' })
    expect(within(items).getByText('VPN / remote access').closest('tr')).toHaveTextContent('2 working days')
  })

  it('adds a system and edits a delivery time', async () => {
    const user = userEvent.setup()
    renderApp('/settings/catalog')
    await user.click(await screen.findByRole('button', { name: 'Add system' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add a system' })
    await user.click(within(dialog).getByRole('button', { name: 'Add system' }))
    expect(await within(dialog).findByText('Enter a name.')).toBeInTheDocument()
    await user.type(within(dialog).getByRole('textbox', { name: 'Name' }), 'Website')
    await user.click(within(dialog).getByRole('button', { name: 'Add system' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Website added.')
    expect(lastRequest('POST', '/systems')?.body).toEqual({ name: 'Website', description: null, sortOrder: 40, active: true })

    await user.click(screen.getByRole('button', { name: 'Edit New email account' }))
    const edit = await screen.findByRole('dialog', { name: 'Edit New email account' })
    const amount = within(edit).getByRole('textbox', { name: 'Delivery time' })
    await user.clear(amount)
    await user.type(amount, '3')
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(lastRequest('PUT', `/catalog-items/${TICKET_IDS.account}`)?.body).toMatchObject({ deliveryMinutes: 3 * 540 }),
    )
  })

  it("won't offer delete for an entry tickets use", async () => {
    const user = userEvent.setup()
    renderApp('/settings/catalog')
    await user.click(await screen.findByRole('button', { name: 'Edit e-Office' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit e-Office' })
    expect(within(dialog).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    expect(within(dialog).getByText(/Hide it instead/)).toBeInTheDocument()
  })

  it('keeps agents out', async () => {
    state.me = AGENT
    const { router } = renderApp('/settings/catalog')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })
})
