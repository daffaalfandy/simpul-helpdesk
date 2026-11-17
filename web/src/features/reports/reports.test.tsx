import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { REPORT_CLIENT_IDS, reportsApi } from '../../test/reportsApi'
import { state, USERS } from '../../test/server'

const SUPERVISOR = USERS['dewi@demo-vendor.test'] ?? null

function clientRow(name: string) {
  const row = screen.getByRole('rowheader', { name }).closest('tr')
  if (!row) throw new Error(`no row ${name}`)
  return within(row)
}

describe('SLA report', () => {
  let saved: string[]

  beforeEach(() => {
    state.me = SUPERVISOR
    saved = []
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:report'), revokeObjectURL: vi.fn() }))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      saved.push(this.download)
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('opens on the current month with its figures and every client', async () => {
    const { router, container } = renderApp('/reports')
    expect(await screen.findByRole('heading', { name: 'SLA report', level: 1 })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/reports/sla')

    const months = screen.getByRole('group', { name: 'Month' })
    // en-GB writes September as "Sept".
    expect(within(months).getAllByRole('button').map((b) => b.textContent)).toEqual(['Sept', 'Oct', 'Nov'])
    expect(within(months).getByRole('button', { name: 'Nov' })).toHaveAttribute('aria-pressed', 'true')

    const figures = container.querySelector('dl')
    expect(figures).toHaveTextContent('Tickets38from 2 clients')
    expect(figures).toHaveTextContent('Within SLA89.5%target 90%')
    expect(figures).toHaveTextContent('SLA breaches4tickets that missed a target')
    expect(figures).toHaveTextContent('Clients below target1review with the account manager')

    expect(screen.getByRole('heading', { name: 'By client · November 2026 (to date)' })).toBeInTheDocument()
    expect(screen.getByText('Contract target: 90% of tickets within SLA')).toBeInTheDocument()
    const jambi = clientRow('Dinas Kominfo Jambi')
    expect(jambi.getByText('21 / 9')).toBeInTheDocument()
    expect(jambi.getByText('14m')).toBeInTheDocument()
    expect(jambi.getByRole('meter', { name: 'Within SLA' })).toHaveAttribute('aria-valuetext', '93.3% within SLA, target 90%')
    const riau = clientRow('Bappeda Prov. Riau')
    expect(riau.getByText('1h 23m')).toBeInTheDocument()
    expect(riau.getByText('Below target')).toBeInTheDocument()
    const setda = clientRow('Setda Kab. Muaro Jambi')
    expect(setda.getByText('No tickets')).toBeInTheDocument()
    expect(setda.getByText('Not set')).toBeInTheDocument()
    expect(setda.getByRole('button', { name: 'Download PDF for Setda Kab. Muaro Jambi' })).toBeInTheDocument()
    expect(screen.getByText('Time spent waiting on the client is excluded from SLA.')).toBeInTheDocument()
  })

  it('switches months in the URL, older ones from the list', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/reports/sla')
    await user.click(await screen.findByRole('button', { name: 'Oct' }))
    expect(await screen.findByRole('heading', { name: 'By client · October 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ month: '2026-10' })
    expect(screen.getByRole('button', { name: 'Oct' })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: /Earlier month/ }))
    await user.click(await screen.findByRole('option', { name: 'August 2026' }))
    expect(await screen.findByRole('heading', { name: 'By client · August 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ month: '2026-08' })
    expect(screen.getAllByText('no tickets this month')).toHaveLength(2)
    for (const name of ['Sept', 'Oct', 'Nov']) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('aria-pressed', 'false')
    }

    // The current month leaves the URL clean.
    await user.click(screen.getByRole('button', { name: 'Nov' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  it('downloads the Excel file and a client PDF for the month shown', async () => {
    const user = userEvent.setup()
    renderApp('/reports/sla?month=2026-10')
    await user.click(await screen.findByRole('button', { name: 'Download Excel' }))
    await waitFor(() => expect(saved).toEqual(['sla-report-2026-10.xlsx']))
    await user.click(clientRow('Dinas Kominfo Jambi').getByRole('button', { name: 'Download PDF for Dinas Kominfo Jambi' }))
    await waitFor(() => expect(saved).toHaveLength(2))
    expect(saved[1]).toBe(`sla-report-2026-10-${REPORT_CLIENT_IDS.jambi}.pdf`)
    expect(reportsApi.requests).toEqual([
      '/api/v1/reports/sla/excel?month=2026-10',
      `/api/v1/reports/sla/clients/${REPORT_CLIENT_IDS.jambi}/pdf?month=2026-10`,
    ])
  })

  it('says when a download fails', async () => {
    reportsApi.failDownloads = true
    const user = userEvent.setup()
    renderApp('/reports/sla')
    await user.click(await screen.findByRole('button', { name: 'Download Excel' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong')
    expect(saved).toEqual([])
  })

  it('explains an install without clients', async () => {
    reportsApi.noClients = true
    renderApp('/reports/sla')
    expect(await screen.findByRole('heading', { name: 'No clients yet' })).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    // Supervisors can't add clients; admins get the way there.
    expect(screen.queryByRole('link', { name: 'Add clients in Settings' })).not.toBeInTheDocument()
  })

  it('points admins without clients to Settings', async () => {
    reportsApi.noClients = true
    state.me = USERS['admin@demo-vendor.test'] ?? null
    renderApp('/reports/sla')
    expect(await screen.findByRole('link', { name: 'Add clients in Settings' })).toHaveAttribute('href', '/settings/clients')
  })

  it('keeps agents out', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/reports/sla')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    expect(nav).not.toHaveTextContent('Reports')
  })
})
