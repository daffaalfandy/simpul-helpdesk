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
    const months = screen.getByRole('group', { name: 'Report months' })
    expect(within(months).getByRole('button', { name: /From/ })).toHaveTextContent('November 2026 (to date)')
    expect(within(months).getByRole('button', { name: /To/ })).toHaveTextContent('November 2026 (to date)')

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

  it('starts the range at an earlier month and adds the months up', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/reports/sla')
    await user.click(await screen.findByRole('button', { name: /From/ }))
    const months = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(months).toEqual(['November 2026 (to date)', 'October 2026', 'September 2026', 'August 2026'])

    await user.click(screen.getByRole('option', { name: 'October 2026' }))
    expect(await screen.findByRole('heading', { name: 'By project · October 2026 to November 2026 (to date)' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ from: '2026-10', to: '2026-11' })
    expect(screen.getByRole('button', { name: /From/ })).toHaveTextContent('October 2026')
    expect(screen.getByRole('button', { name: /To/ })).toHaveTextContent('November 2026 (to date)')
    // e-Office's 71 October tickets and 30 November ones.
    expect(document.querySelector('dl')).toHaveTextContent('Tickets109from 3 projects')
    expect(projectRow('e-Office').getByText('66 / 35')).toBeInTheDocument()
    expect(projectRow('Legacy portal').getByText('Hidden')).toBeInTheDocument()
  })

  it('offers only months that keep the range in order, and the current month leaves the URL clean', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/reports/sla?from=2026-10&to=2026-11')
    const to = await screen.findByRole('button', { name: /To/ })
    await user.click(to)
    let options = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(options).toEqual(['November 2026 (to date)', 'October 2026'])
    await user.click(screen.getByRole('option', { name: 'October 2026' }))
    expect(await screen.findByRole('heading', { name: 'By project · October 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ from: '2026-10', to: '2026-10' })
    // A hidden project without tickets in the range is left out.
    expect(screen.queryByRole('rowheader', { name: /^Legacy portal/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /From/ }))
    options = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(options).toEqual(['October 2026', 'September 2026', 'August 2026'])
    await user.click(screen.getByRole('option', { name: 'August 2026' }))
    expect(await screen.findByRole('heading', { name: 'By project · August 2026 to October 2026' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ from: '2026-08', to: '2026-10' })

    // Back to the current month alone, with the keyboard: To first, then From.
    screen.getByRole('button', { name: /To/ }).focus()
    await user.keyboard('{Enter}')
    await screen.findByRole('listbox')
    await user.keyboard('{Home}{Enter}')
    expect(await screen.findByRole('heading', { name: 'By project · August 2026 to November 2026 (to date)' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ from: '2026-08', to: '2026-11' })
    screen.getByRole('button', { name: /From/ }).focus()
    await user.keyboard('{Enter}')
    await screen.findByRole('listbox')
    await user.keyboard('{Home}{Enter}')
    expect(await screen.findByRole('heading', { name: 'By project · November 2026 (to date)' })).toBeInTheDocument()
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  it('says when a range of months has no tickets', async () => {
    renderApp('/reports/sla?from=2026-08&to=2026-09')
    expect(await screen.findByRole('heading', { name: 'By project · August 2026 to September 2026' })).toBeInTheDocument()
    expect(screen.getAllByText('no tickets in these months')).toHaveLength(2)
  })

  it.each([
    { name: 'a start more than 12 months back', search: '?from=2025-10&to=2026-11' },
    { name: 'an end not started yet', search: '?from=2026-11&to=2026-12' },
    { name: 'an end before the start', search: '?from=2026-11&to=2026-10' },
    { name: 'an earlier end alone', search: '?to=2026-10' },
  ])('opens the current month for months it does not serve ($name)', async ({ search }) => {
    const { router } = renderApp(`/reports/sla${search}`)
    expect(await screen.findByRole('heading', { name: 'By project · November 2026 (to date)' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({})
  })

  it.each([
    {
      name: 'one month',
      search: '?from=2026-10&to=2026-10',
      query: '?from=2026-10&to=2026-10',
      excel: 'sla-report-2026-10.xlsx',
      pdf: `sla-report-2026-10-${REPORT_PROJECT_IDS.eOffice}.pdf`,
    },
    {
      name: 'several months',
      search: '?from=2026-09&to=2026-11',
      query: '?from=2026-09&to=2026-11',
      excel: 'sla-report-2026-09-to-2026-11.xlsx',
      pdf: `sla-report-2026-09-to-2026-11-${REPORT_PROJECT_IDS.eOffice}.pdf`,
    },
    {
      name: 'the current month',
      search: '',
      query: '?from=2026-11&to=2026-11',
      excel: 'sla-report-2026-11.xlsx',
      pdf: `sla-report-2026-11-${REPORT_PROJECT_IDS.eOffice}.pdf`,
    },
  ])('downloads the Excel file and a project PDF for the months shown ($name)', async ({ search, query, excel, pdf }) => {
    const user = userEvent.setup()
    renderApp(`/reports/sla${search}`)
    await user.click(await screen.findByRole('button', { name: 'Download Excel' }))
    await waitFor(() => expect(saved).toEqual([excel]))
    await user.click(projectRow('e-Office').getByRole('button', { name: 'Download PDF for e-Office, Dinas Kominfo Jambi' }))
    await waitFor(() => expect(saved).toHaveLength(2))
    expect(saved[1]).toBe(pdf)
    expect(reportsApi.requests).toEqual([
      `/api/v1/reports/sla/excel${query}`,
      `/api/v1/reports/sla/projects/${REPORT_PROJECT_IDS.eOffice}/pdf${query}`,
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
