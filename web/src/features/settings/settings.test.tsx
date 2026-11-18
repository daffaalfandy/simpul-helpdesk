import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { state, USERS } from '../../test/server'
import { IDS, settings, THIS_YEAR } from '../../test/settingsApi'
import { renderApp } from '../../test/renderApp'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null
const AGENT = USERS['rina@demo-vendor.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...settings.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

describe('settings', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('opens General with the business hours and holidays', async () => {
    const { router } = renderApp('/settings')
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/general'))
    expect(await screen.findByRole('heading', { name: 'General', level: 2 })).toBeInTheDocument()
    const hours = screen.getByRole('heading', { name: 'Business hours' }).closest('section') as HTMLElement
    expect(within(hours).getByText('Mon – Fri')).toBeInTheDocument()
    expect(within(hours).getByText('08:00 – 17:00')).toBeInTheDocument()
    expect(within(hours).getByText('Sat – Sun')).toBeInTheDocument()
    expect(within(hours).getByText('Closed')).toBeInTheDocument()
    expect(await within(hours).findByText(`1 holiday in ${THIS_YEAR}`)).toBeInTheDocument()
    expect(screen.getByText('Hari Lahir Pancasila')).toBeInTheDocument()
  })

  it('keeps agents out', async () => {
    state.me = AGENT
    const { router } = renderApp('/settings/clients')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })

  it('collapses groups, collapses all, and folds into an icon rail', async () => {
    const user = userEvent.setup()
    renderApp('/settings/priorities')
    const nav = await screen.findByRole('navigation', { name: 'Settings sections' })
    expect(within(nav).getByRole('button', { name: 'Collapse Service desk' })).toHaveAttribute('aria-expanded', 'true')
    expect(within(nav).getByRole('button', { name: 'Expand Organisation' })).toHaveAttribute('aria-expanded', 'false')
    expect(within(nav).getByRole('link', { name: 'Priorities' })).toHaveAttribute('aria-current', 'page')

    await user.click(within(nav).getByRole('button', { name: 'Expand Organisation' }))
    expect(within(nav).getByRole('link', { name: 'General' })).toBeVisible()

    await user.click(within(nav).getByRole('button', { name: 'Collapse all' }))
    expect(within(nav).getByRole('button', { name: 'Expand all' })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'Priorities' })).not.toBeInTheDocument()

    await user.click(within(nav).getByRole('button', { name: 'Collapse settings menu' }))
    const rail = screen.getByRole('navigation', { name: 'Settings sections' })
    expect(within(rail).getByRole('button', { name: 'Expand settings menu' })).toBeInTheDocument()
    expect(within(rail).getByRole('link', { name: 'Clients' })).toBeInTheDocument()
  })

  describe('clients', () => {
    it('lists clients with contract period and projects', async () => {
      renderApp('/settings/clients')
      const table = await screen.findByRole('table', { name: 'Clients under contract' })
      const jambi = within(table).getByRole('row', { name: /Dinas Kominfo Jambi/ })
      expect(jambi).toHaveTextContent('Jan–Dec 2026')
      expect(jambi).toHaveTextContent('e-Office, Email')
      expect(jambi).toHaveTextContent('Hendra Gunawan')
      const batam = within(table).getByRole('row', { name: /BPKAD Kota Batam/ })
      expect(batam).toHaveTextContent('Mar 2026–Feb 2027')
      expect(batam).toHaveTextContent('Network & infra support')
    })

    it('adds a client with an agent', async () => {
      const user = userEvent.setup()
      renderApp('/settings/clients')
      await user.click(await screen.findByRole('button', { name: 'Add client' }))
      const dialog = await screen.findByRole('dialog', { name: 'Add client' })
      expect(within(dialog).getByRole('textbox', { name: /Contract number/ })).toHaveAccessibleDescription(/Optional/)
      await user.type(within(dialog).getByRole('textbox', { name: /Client name/ }), 'Bappeda Prov. Riau')
      await user.click(within(dialog).getByRole('checkbox', { name: /Adi Saputra/ }))
      await user.click(within(dialog).getByRole('button', { name: 'Add client' }))

      expect(await screen.findByRole('status')).toHaveTextContent('Bappeda Prov. Riau added.')
      expect(lastRequest('POST', '/clients')?.body).toMatchObject({ name: 'Bappeda Prov. Riau', contractNo: null })
      expect(lastRequest('PUT', '/agents')?.body).toEqual({ userIds: [IDS.adi] })
      expect(await screen.findByRole('row', { name: /Bappeda Prov. Riau/ })).toHaveTextContent('No projects yet')
    })

    it('sets the contract period by typing the start and picking the end from its calendar', async () => {
      const user = userEvent.setup()
      renderApp('/settings/clients')
      await user.click(await screen.findByRole('button', { name: 'Add client' }))
      const dialog = await screen.findByRole('dialog', { name: 'Add client' })
      const start = within(dialog).getByRole('group', { name: 'Contract start' })
      expect(start).toHaveAccessibleDescription(/Shown in the clients list/)
      expect(within(dialog).getByRole('group', { name: 'Contract end' })).toHaveAccessibleDescription(/Optional\./)

      await user.type(within(dialog).getByRole('textbox', { name: /Client name/ }), 'Bappeda Prov. Riau')
      await user.click(within(start).getByRole('spinbutton', { name: /day/ }))
      await user.keyboard('01032026')
      // The end's calendar opens on the start's month.
      await user.click(within(dialog).getByRole('button', { name: 'Open calendar Contract end' }))
      const calendar = await screen.findByRole('grid', { name: /March 2026/ })
      await user.click(within(calendar).getByRole('button', { name: /31 March 2026/ }))
      await waitFor(() => expect(screen.queryByRole('grid')).not.toBeInTheDocument())

      await user.click(within(dialog).getByRole('button', { name: 'Add client' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Bappeda Prov. Riau added.')
      expect(lastRequest('POST', '/clients')?.body).toMatchObject({ contractStart: '2026-03-01', contractEnd: '2026-03-31' })
    })

    it('shows the API message when the name is taken', async () => {
      const user = userEvent.setup()
      renderApp('/settings/clients')
      await user.click(await screen.findByRole('button', { name: 'Add client' }))
      const dialog = await screen.findByRole('dialog', { name: 'Add client' })
      await user.type(within(dialog).getByRole('textbox', { name: /Client name/ }), 'BPKAD Kota Batam')
      await user.click(within(dialog).getByRole('button', { name: 'Add client' }))
      expect(await within(dialog).findByRole('alert')).toHaveTextContent('A client with this name already exists.')
      expect(lastRequest('PUT', '/agents')).toBeUndefined()
    })

    it("won't delete a client that still has requesters", async () => {
      const user = userEvent.setup()
      renderApp('/settings/clients')
      await user.click(await screen.findByRole('button', { name: 'Edit Dinas Kominfo Jambi' }))
      await user.click(await screen.findByRole('button', { name: 'Delete client' }))
      const confirm = await screen.findByRole('alertdialog', { name: 'Delete Dinas Kominfo Jambi?' })
      await user.click(within(confirm).getByRole('button', { name: 'Delete client' }))
      expect(await within(confirm).findByRole('alert')).toHaveTextContent('This client still has requesters or tickets')
    })
  })


  describe('General', () => {
    it('saves business hours', async () => {
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Edit hours' }))
      const dialog = await screen.findByRole('dialog', { name: 'Edit business hours' })
      await user.click(within(dialog).getByRole('checkbox', { name: 'Friday' }))
      await user.click(within(dialog).getByRole('button', { name: /Opens/ }))
      await user.click(await screen.findByRole('option', { name: '07:30' }))
      await user.click(within(dialog).getByRole('button', { name: /Closes/ }))
      await user.click(await screen.findByRole('option', { name: '24:00 (end of day)' }))
      expect(within(dialog).getByText(/Now: 16 h 30 min\./)).toBeInTheDocument()
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Business hours saved.')
      expect(lastRequest('PUT', '/business-hours')?.body).toEqual({
        weekdays: ['mon', 'tue', 'wed', 'thu'],
        opens: '07:30',
        closes: '24:00',
      })
      expect(await screen.findByText('Mon – Thu')).toBeInTheDocument()
      expect(screen.getByText('07:30 – 24:00')).toBeInTheDocument()
    })

    it('keeps a saved time that is off the half hour', async () => {
      settings.hours = { ...settings.hours, opens: '08:15' }
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Edit hours' }))
      const dialog = await screen.findByRole('dialog', { name: 'Edit business hours' })
      const opens = within(dialog).getByRole('button', { name: /Opens/ })
      expect(opens).toHaveTextContent('08:15')
      await user.click(opens)
      const times = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
      expect(times.slice(times.indexOf('08:00'), times.indexOf('08:00') + 3)).toEqual(['08:00', '08:15', '08:30'])
      await user.keyboard('{Escape}')
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Business hours saved.')
      expect(lastRequest('PUT', '/business-hours')?.body).toMatchObject({ opens: '08:15', closes: '17:00' })
    })

    it("refuses a closing time that isn't after the opening time", async () => {
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Edit hours' }))
      const dialog = await screen.findByRole('dialog', { name: 'Edit business hours' })
      await user.click(within(dialog).getByRole('button', { name: /Closes/ }))
      await user.click(await screen.findByRole('option', { name: '08:00' }))
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
      expect(within(dialog).getByText('Closing time must be after opening time.')).toBeInTheDocument()
      expect(lastRequest('PUT', '/business-hours')).toBeUndefined()
    })

    it('adds a holiday picked from the calendar', async () => {
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Add holiday' }))
      const dialog = await screen.findByRole('dialog', { name: 'Add a holiday' })
      await user.click(within(dialog).getByRole('button', { name: 'Open calendar Date' }))
      const calendar = await screen.findByRole('dialog', { name: /Open calendar Date/ })
      await user.click(within(calendar).getByRole('button', { name: /Month/ }))
      await user.click(await screen.findByRole('option', { name: 'Aug' }))
      await user.click(within(calendar).getByRole('button', { name: new RegExp(`17 August ${THIS_YEAR}`) }))
      await waitFor(() => expect(screen.queryByRole('grid')).not.toBeInTheDocument())

      await user.type(within(dialog).getByRole('textbox', { name: /Name/ }), 'Hari Kemerdekaan')
      await user.click(within(dialog).getByRole('button', { name: 'Add holiday' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Hari Kemerdekaan added.')
      expect(lastRequest('POST', '/holidays')?.body).toEqual({ date: `${THIS_YEAR}-08-17`, name: 'Hari Kemerdekaan' })
    })

    it('loads the official holiday list and removes a holiday', async () => {
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Add 3 dates' }))
      expect(await screen.findByRole('status')).toHaveTextContent(`Added 3 holidays for ${THIS_YEAR}.`)
      expect(screen.queryByRole('button', { name: 'Add 3 dates' })).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'Remove Hari Lahir Pancasila' }))
      const confirm = await screen.findByRole('alertdialog', { name: 'Remove this holiday?' })
      await user.click(within(confirm).getByRole('button', { name: 'Remove' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Hari Lahir Pancasila removed.')
      expect(lastRequest('DELETE', `/holidays/${IDS.holiday}`)).toBeDefined()
      expect(await screen.findByText(`No holidays for ${THIS_YEAR} yet.`)).toBeInTheDocument()
    })
  })
})
