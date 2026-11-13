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

describe('catalogue', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('lists the catalogue under Service desk', async () => {
    renderApp('/settings/catalog')
    const nav = await screen.findByRole('navigation', { name: 'Settings sections' })
    expect(within(nav).getByRole('link', { name: 'Catalogue' })).toHaveAttribute('aria-current', 'page')
    const items = screen.getByRole('table', { name: 'Catalogue items' })
    expect(within(items).getByText('VPN / remote access').closest('tr')).toHaveTextContent('Shown')
    expect(screen.queryByRole('table', { name: 'Systems' })).not.toBeInTheDocument()
  })

  it('adds an item and edits one', async () => {
    const user = userEvent.setup()
    renderApp('/settings/catalog')
    await user.click(await screen.findByRole('button', { name: 'Add item' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add a catalogue item' })
    await user.click(within(dialog).getByRole('button', { name: 'Add item' }))
    expect(await within(dialog).findByText('Enter a name.')).toBeInTheDocument()
    await user.type(within(dialog).getByRole('textbox', { name: 'Name' }), 'Install licensed software')
    await user.click(within(dialog).getByRole('button', { name: 'Add item' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Install licensed software added.')
    expect(lastRequest('POST', '/catalog-items')?.body).toEqual({
      name: 'Install licensed software',
      description: null,
      sortOrder: 30,
      active: true,
    })

    await user.click(screen.getByRole('button', { name: 'Edit New email account' }))
    const edit = await screen.findByRole('dialog', { name: 'Edit New email account' })
    await user.click(within(edit).getByRole('checkbox', { name: 'Show on the report form' }))
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(lastRequest('PUT', `/catalog-items/${TICKET_IDS.account}`)?.body).toMatchObject({ active: false }),
    )
  })

  it("won't offer delete for an item tickets use", async () => {
    const user = userEvent.setup()
    renderApp('/settings/catalog')
    await user.click(await screen.findByRole('button', { name: 'Edit VPN / remote access' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit VPN / remote access' })
    expect(within(dialog).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    expect(within(dialog).getByText(/Hide it instead/)).toBeInTheDocument()
  })

  it('keeps agents out', async () => {
    state.me = AGENT
    const { router } = renderApp('/settings/catalog')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })
})
