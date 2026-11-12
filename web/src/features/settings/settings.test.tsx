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
    renderApp('/settings/sla')
    const nav = await screen.findByRole('navigation', { name: 'Settings sections' })
    expect(within(nav).getByRole('button', { name: 'Collapse Service desk' })).toHaveAttribute('aria-expanded', 'true')
    expect(within(nav).getByRole('button', { name: 'Expand Organisation' })).toHaveAttribute('aria-expanded', 'false')
    expect(within(nav).getByRole('link', { name: 'SLA & priority' })).toHaveAttribute('aria-current', 'page')

    await user.click(within(nav).getByRole('button', { name: 'Expand Organisation' }))
    expect(within(nav).getByRole('link', { name: 'General' })).toBeVisible()

    await user.click(within(nav).getByRole('button', { name: 'Collapse all' }))
    expect(within(nav).getByRole('button', { name: 'Expand all' })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'SLA & priority' })).not.toBeInTheDocument()

    await user.click(within(nav).getByRole('button', { name: 'Collapse settings menu' }))
    const rail = screen.getByRole('navigation', { name: 'Settings sections' })
    expect(within(rail).getByRole('button', { name: 'Expand settings menu' })).toBeInTheDocument()
    expect(within(rail).getByRole('link', { name: 'Clients' })).toBeInTheDocument()
  })

  describe('clients', () => {
    it('lists clients with contract period, plan and agents', async () => {
      renderApp('/settings/clients')
      const table = await screen.findByRole('table', { name: 'Clients under contract' })
      const jambi = within(table).getByRole('row', { name: /Dinas Kominfo Jambi/ })
      expect(jambi).toHaveTextContent('Jan–Dec 2026')
      expect(jambi).toHaveTextContent('Gold')
      expect(jambi).toHaveTextContent('Rina Pratiwi')
      expect(within(table).getByRole('row', { name: /BPKAD Kota Batam/ })).toHaveTextContent('Mar 2026–Feb 2027')
    })

    it('adds a client with a plan and an agent', async () => {
      const user = userEvent.setup()
      renderApp('/settings/clients')
      await user.click(await screen.findByRole('button', { name: 'Add client' }))
      const dialog = await screen.findByRole('dialog', { name: 'Add client' })
      await user.type(within(dialog).getByRole('textbox', { name: /Client name/ }), 'Bappeda Prov. Riau')
      await user.click(within(dialog).getByRole('button', { name: /SLA plan/ }))
      await user.click(await screen.findByRole('option', { name: /Gold/ }))
      await user.click(within(dialog).getByRole('checkbox', { name: /Adi Saputra/ }))
      await user.click(within(dialog).getByRole('button', { name: 'Add client' }))

      expect(await screen.findByRole('status')).toHaveTextContent('Bappeda Prov. Riau added.')
      expect(lastRequest('POST', '/clients')?.body).toMatchObject({ name: 'Bappeda Prov. Riau', slaPlanId: IDS.gold })
      expect(lastRequest('PUT', '/agents')?.body).toEqual({ userIds: [IDS.adi] })
      expect(await screen.findByRole('row', { name: /Bappeda Prov. Riau/ })).toHaveTextContent('Adi Saputra')
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
      expect(await within(confirm).findByRole('alert')).toHaveTextContent('This client still has requesters.')
    })
  })

  describe('SLA & priority', () => {
    it('shows each plan with readable targets', async () => {
      renderApp('/settings/sla')
      const standard = await screen.findByRole('region', { name: /Standard/ })
      expect(within(standard).getByText('Default')).toBeInTheDocument()
      const p3 = within(standard).getByRole('row', { name: /P3 · Standard/ })
      expect(p3).toHaveTextContent('2 hours')
      expect(p3).toHaveTextContent('2 working days')
      expect(p3).toHaveTextContent('Catalogue time, else 2 working days')
      expect(within(standard).getByRole('row', { name: /P1 · Critical/ })).toHaveTextContent('24/7')
      expect(within(standard).getByRole('button', { name: 'Delete Standard' })).toBeDisabled()
    })

    it('saves edited targets as minutes', async () => {
      const user = userEvent.setup()
      renderApp('/settings/sla')
      await user.click(await screen.findByRole('button', { name: 'Edit Standard' }))
      const dialog = await screen.findByRole('dialog', { name: 'Edit Standard' })
      const p2 = within(dialog).getByRole('group', { name: 'P2 · High' })
      const amount = within(p2).getByRole('textbox', { name: 'Resolve (incidents)' })
      await user.clear(amount)
      await user.type(amount, '1')
      await user.click(within(p2).getByRole('button', { name: /Resolve \(incidents\) unit for P2/ }))
      await user.click(await screen.findByRole('option', { name: 'working days' }))
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))

      expect(await screen.findByRole('status')).toHaveTextContent('Standard saved.')
      const body = lastRequest('PUT', `/sla-plans/${IDS.standard}`)?.body as { targets: Array<{ resolveMinutes: number }> }
      expect(body.targets.map((t) => t.resolveMinutes)).toEqual([240, 540, 1080, 2700])
    })

    it('makes another plan the default', async () => {
      const user = userEvent.setup()
      renderApp('/settings/sla')
      await user.click(await screen.findByRole('button', { name: 'Make Gold the default plan' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Gold is now the default plan for new clients.')
      expect(lastRequest('POST', `/sla-plans/${IDS.gold}/default`)).toBeDefined()
    })
  })

  describe('General', () => {
    it('saves business hours', async () => {
      const user = userEvent.setup()
      renderApp('/settings/general')
      await user.click(await screen.findByRole('button', { name: 'Edit hours' }))
      const dialog = await screen.findByRole('dialog', { name: 'Edit business hours' })
      await user.click(within(dialog).getByRole('checkbox', { name: 'Friday' }))
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
      expect(await screen.findByRole('status')).toHaveTextContent('Business hours saved.')
      expect(lastRequest('PUT', '/business-hours')?.body).toEqual({
        weekdays: ['mon', 'tue', 'wed', 'thu'],
        opens: '08:00',
        closes: '17:00',
      })
      expect(await screen.findByText('Mon – Thu')).toBeInTheDocument()
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
