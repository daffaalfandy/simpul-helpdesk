import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { REPORT_PROJECT_IDS, reportsApi } from '../../test/reportsApi'
import { state, USERS } from '../../test/server'

const SUPERVISOR = USERS['dewi@demo-vendor.test'] ?? null

/** The table row whose header starts with the project's name. */
function projectRow(project: string) {
  const row = screen.getByRole('rowheader', { name: new RegExp(`^${project}`) }).closest('tr')
  if (!row) throw new Error(`no row ${project}`)
  return within(row)
}

function rowHeaders() {
  return screen.getAllByRole('rowheader').map((h) => h.textContent)
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

  it('opens on the current month with its figures and every project', async () => {
    const { router, container } = renderApp('/reports')
    expect(await screen.findByRole('heading', { name: 'SLA report', level: 1 })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/reports/sla')
    expect(screen.getByRole('button', { name: /Month/ })).toHaveTextContent('November 2026 (to date)')

    const figures = container.querySelector('dl')
    expect(figures).toHaveTextContent('Tickets38from 3 projects')
    expect(figures).toHaveTextContent('Within SLA89.5%target 90%')
    expect(figures).toHaveTextContent('SLA breaches4tickets that missed a target')
    expect(figures).toHaveTextContent('Projects below target1review with the account manager')

    expect(screen.getByRole('heading', { name: 'By project · November 2026 (to date)' })).toBeInTheDocument()
    expect(screen.getByRole('table', { name: 'SLA by project' })).toBeInTheDocument()
    expect(screen.getByText('Contract target: 90% of tickets within SLA')).toBeInTheDocument()
    // Busiest first, each project with its client underneath.
    expect(rowHeaders()).toEqual([
      'e-OfficeDinas Kominfo Jambi',
      'WebsiteBappeda Prov. Riau',
      'Legacy portalHiddenBappeda Prov. Riau',
      'Network & infra supportSetda Kab. Muaro Jambi',
    ])
    const eOffice = projectRow('e-Office')
    expect(eOffice.getByText('21 / 9')).toBeInTheDocument()
    expect(eOffice.getByText('14m')).toBeInTheDocument()
    expect(eOffice.getByRole('meter', { name: 'Within SLA' })).toHaveAttribute('aria-valuetext', '93.3% within SLA, target 90%')
    expect(eOffice.queryByText('Hidden')).not.toBeInTheDocument()
    const website = projectRow('Website')
    expect(website.getByText('1h 23m')).toBeInTheDocument()
    expect(website.getByText('Below target')).toBeInTheDocument()
    expect(projectRow('Legacy portal').getByText('Hidden')).toBeInTheDocument()
    const network = projectRow('Network')
    expect(network.getByText('No tickets')).toBeInTheDocument()
    expect(network.getByText('Not set')).toBeInTheDocument()
    expect(network.getByRole('button', { name: 'Download PDF for Network & infra support, Setda Kab. Muaro Jambi' })).toBeInTheDocument()
    expect(screen.getByText('Time spent waiting on the client is excluded from SLA.')).toBeInTheDocument()
  })

  it('picks the month from a list, newest first, back to the first ticket', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/reports/sla')
    const picker = await screen.findByRole('button', { name: /Month/ })
    await user.click(picker)
    const months = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(months).toEqual(['November 2026 (to date)', 'October 2026', 'September 2026', 'August 2026'])

    await user.click(screen.getByRole('option', { name: 'October 2026' }))
    expect(await screen.findByRole('heading', { name: 'By project · October 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ month: '2026-10' })
    expect(screen.getByRole('button', { name: /Month/ })).toHaveTextContent('October 2026')
    // A hidden project without tickets this month is left out.
    expect(screen.queryByRole('rowheader', { name: /^Legacy portal/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Month/ }))
    await user.click(await screen.findByRole('option', { name: 'August 2026' }))
    expect(await screen.findByRole('heading', { name: 'By project · August 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ month: '2026-08' })
    expect(screen.getAllByText('no tickets this month')).toHaveLength(2)

    // The current month leaves the URL clean.
    await user.click(screen.getByRole('button', { name: /Month/ }))
    await user.click(await screen.findByRole('option', { name: 'November 2026 (to date)' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  it.each([
    { name: 'more than 12 months back', month: '2025-10' },
    { name: 'not started yet', month: '2026-12' },
  ])('opens the current month for a month it does not serve ($name)', async ({ month }) => {
    const { router } = renderApp(`/reports/sla?month=${month}`)
    expect(await screen.findByRole('heading', { name: 'By project · November 2026 (to date)' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({})
  })

  it('downloads the Excel file and a project PDF for the month shown', async () => {
    const user = userEvent.setup()
    renderApp('/reports/sla?month=2026-10')
    await user.click(await screen.findByRole('button', { name: 'Download Excel' }))
    await waitFor(() => expect(saved).toEqual(['sla-report-2026-10.xlsx']))
    await user.click(projectRow('e-Office').getByRole('button', { name: 'Download PDF for e-Office, Dinas Kominfo Jambi' }))
    await waitFor(() => expect(saved).toHaveLength(2))
    expect(saved[1]).toBe(`sla-report-2026-10-${REPORT_PROJECT_IDS.eOffice}.pdf`)
    expect(reportsApi.requests).toEqual([
      '/api/v1/reports/sla/excel?month=2026-10',
      `/api/v1/reports/sla/projects/${REPORT_PROJECT_IDS.eOffice}/pdf?month=2026-10`,
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

  it('explains an organization without projects and points to Projects', async () => {
    reportsApi.noProjects = true
    renderApp('/reports/sla')
    expect(await screen.findByRole('heading', { name: 'No projects yet' })).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Add a project' })).toHaveAttribute('href', '/projects')
  })

  it('keeps agents out', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/reports/sla')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    expect(nav).not.toHaveTextContent('Reports')
  })
})
