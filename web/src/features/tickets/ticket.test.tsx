import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { PROJECT_IDS } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { writeRichText } from '../../test/richText'
import { state, USERS } from '../../test/server'
import { tickets } from '../../test/ticketsApi'

const AGENT = USERS['rina@demo-vendor.test'] ?? null
const REQUESTER = USERS['hendra@kominfo-jambi.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

async function open() {
  const user = userEvent.setup()
  renderApp('/tickets/HD-24871')
  await screen.findByRole('heading', { level: 1, name: 'e-Office login fails for all staff' })
  return user
}

describe('agent ticket detail', () => {
  beforeEach(() => {
    state.me = AGENT
  })

  it('shows the header, the whole conversation and the targets', async () => {
    await open()
    const meta = screen.getByText(/^HD-24871 · Incident · Opened 12 Nov( 2026)?, 08:14 via client portal$/).parentElement as HTMLElement
    expect(within(meta).getByText('P1 · Critical')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Status/ })).toHaveTextContent('In progress')
    const headers = screen.getAllByText('Rina Pratiwi', { selector: 'strong' }).map((el) => el.parentElement?.textContent)
    expect(headers).toContain('Internal note · Rina Pratiwi (not visible to client)')
    expect(screen.getByText("Matches last night's SSO patch.")).toBeInTheDocument()
    const sla = screen.getByRole('region', { name: 'SLA · P1 · e-Office project' })
    expect(within(sla).getByText('First response · 15m')).toBeInTheDocument()
    expect(within(sla).getByText(/^Sent /)).toBeInTheDocument()
    expect(within(sla).getByText('Resolution · 4h')).toBeInTheDocument()
    expect(within(sla).getByText('Runs 24/7')).toBeInTheDocument()
    expect(within(sla).getByText("Time waiting for the client's reply or test doesn't count.")).toBeInTheDocument()
    const properties = screen.getByRole('heading', { name: 'Properties' }).closest('section') as HTMLElement
    expect(within(properties).getByRole('link', { name: 'e-Office' })).toHaveAttribute('href', `/projects/${PROJECT_IDS.eOffice}`)
    const log = screen.getByRole('heading', { name: 'Activity log' }).closest('section') as HTMLElement
    expect(await within(log).findByText('Created by Hendra Gunawan via client portal')).toBeInTheDocument()
    expect(within(log).getByText('Assigned to Rina Pratiwi by Rina Pratiwi')).toBeInTheDocument()
  })

  it('sends replies and internal notes from the tabs', async () => {
    const user = await open()
    const tabs = screen.getByRole('tablist', { name: 'Message type' })
    expect(within(tabs).getByRole('tab', { name: 'Reply to client' })).toHaveAttribute('aria-selected', 'true')
    await user.click(screen.getByRole('button', { name: 'Send reply' }))
    expect(await screen.findByText('Write a message first.')).toBeInTheDocument()

    await user.click(within(tabs).getByRole('tab', { name: 'Internal note' }))
    await writeRichText(user, screen.getByRole('textbox', { name: 'Note for the support team only' }), 'Ask Adi to revert')
    await user.click(screen.getByRole('button', { name: 'Save note' }))
    await waitFor(() =>
      expect(lastRequest('POST', '/tickets/HD-24871/messages')?.body).toEqual({
        visibility: 'internal',
        body: 'Ask Adi to revert',
        attachmentIds: [],
      }),
    )
    expect(await screen.findByText('Ask Adi to revert')).toBeInTheDocument()
  })

  it('changes status with the hint shown', async () => {
    const user = await open()
    await user.click(screen.getByRole('button', { name: /Status$/ }))
    const option = await screen.findByRole('option', { name: /Waiting on client/ })
    expect(option).toHaveTextContent('Pauses the SLA clock')
    expect(screen.queryByRole('option', { name: /Closed/ })).not.toBeInTheDocument()
    await user.click(option)
    await waitFor(() => expect(lastRequest('PUT', '/tickets/HD-24871/status')?.body).toEqual({ status: 'waiting_on_client', from: 'in_progress' }))
    expect(
      await screen.findByText('Status changed from In progress to Waiting on client by Rina Pratiwi'),
    ).toBeInTheDocument()
  })

  it('unassigns', async () => {
    const user = await open()
    await user.click(screen.getByRole('button', { name: /Assignee$/ }))
    await user.click(await screen.findByRole('option', { name: 'Unassigned' }))
    await waitFor(() => expect(lastRequest('PUT', '/tickets/HD-24871/assignee')?.body).toEqual({ assigneeId: null }))
  })

  it('asks for a reason before changing priority', async () => {
    const user = await open()
    await user.click(screen.getByRole('button', { name: /Priority$/ }))
    await user.click(await screen.findByRole('option', { name: 'P2 · High' }))
    const dialog = await screen.findByRole('alertdialog', { name: 'Change priority to P2 · High?' })
    const confirm = within(dialog).getByRole('button', { name: 'Change priority' })
    expect(confirm).toBeDisabled()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(lastRequest('PUT', '/tickets/HD-24871/priority')).toBeUndefined()

    await user.click(screen.getByRole('button', { name: /Priority$/ }))
    await user.click(await screen.findByRole('option', { name: 'P2 · High' }))
    const again = await screen.findByRole('alertdialog')
    await user.type(within(again).getByRole('textbox'), 'Only one department affected')
    await user.click(within(again).getByRole('button', { name: 'Change priority' }))
    await waitFor(() =>
      expect(lastRequest('PUT', '/tickets/HD-24871/priority')?.body).toEqual({
        priority: 'P2',
        reason: 'Only one department affected',
      }),
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it("shows the ticket's Definition of Done with what is still pending", async () => {
    await open()
    const card = screen.getByRole('region', { name: 'Definition of Done' })
    expect(card).toHaveTextContent('0 of 1 done')
    const which = within(card).getByRole('radiogroup', { name: 'Which Definition of Done applies' })
    expect(within(which).getByRole('radio', { name: /Project default/ })).toBeChecked()
    const item = within(within(card).getByRole('list', { name: 'Checklist' })).getByRole('listitem')
    expect(item).toHaveTextContent('UAT approved by client')
    expect(item).toHaveTextContent('Asked of Hendra Gunawan when the ticket moves to In UAT')
    expect(within(item).getByRole('img', { name: 'Not done yet' })).toBeInTheDocument()
    expect(card).toHaveTextContent('Move the ticket to In UAT when the fix is ready; the client is asked to test.')
  })

  it('guards resolving with the Definition of Done unmet', async () => {
    const user = await open()
    await user.click(screen.getByRole('button', { name: /Status$/ }))
    await user.click(await screen.findByRole('option', { name: /^Resolved/ }))
    const guard = await screen.findByRole('alertdialog', { name: 'Resolve HD-24871 without meeting its Definition of Done?' })
    expect(guard).toHaveTextContent("This ticket's project needs the client's UAT approval before it resolves.")
    expect(within(guard).getByRole('button', { name: 'Resolve anyway' })).toBeDisabled()
    await user.click(within(guard).getByRole('button', { name: 'Keep where it is' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(lastRequest('PUT', '/tickets/HD-24871/status')).toBeUndefined()

    // The header button opens the same guard.
    await user.click(screen.getByRole('button', { name: 'Resolve without DoD…' }))
    const again = await screen.findByRole('alertdialog')
    await user.type(within(again).getByRole('textbox', { name: /Reason/ }), 'Client confirmed by phone')
    await user.click(within(again).getByRole('button', { name: 'Resolve anyway' }))
    await waitFor(() =>
      expect(lastRequest('PUT', '/tickets/HD-24871/status')?.body).toEqual({
        status: 'resolved',
        from: 'in_progress',
        reason: 'Client confirmed by phone',
      }),
    )
    expect(
      await screen.findByText('Resolved by Rina Pratiwi without meeting the Definition of Done: Client confirmed by phone'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Resolve without DoD…' })).not.toBeInTheDocument()
  })

  it('gives the ticket its own Definition of Done with a reason, and returns to the project one', async () => {
    const user = await open()
    const card = screen.getByRole('region', { name: 'Definition of Done' })
    await user.click(within(card).getByRole('radio', { name: /Custom for this ticket/ }))
    const dialog = await screen.findByRole('dialog', { name: 'A Definition of Done for HD-24871' })
    await user.click(within(dialog).getByRole('checkbox', { name: /UAT approved by the client/ }))
    expect(dialog).toHaveTextContent('Done means: nothing automatic. An agent resolves each ticket by hand.')
    await user.click(within(dialog).getByRole('button', { name: 'Use for this ticket' }))
    expect(within(dialog).getByText('Give a reason.')).toBeInTheDocument()
    await user.type(within(dialog).getByRole('textbox', { name: /Reason/ }), 'P1 hotfix, no UAT')
    await user.click(within(dialog).getByRole('button', { name: 'Use for this ticket' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    const body = lastRequest('PUT', '/tickets/HD-24871/dod')?.body as { dod: { uat: { enabled: boolean } }; reason: string }
    expect(body.dod.uat.enabled).toBe(false)
    expect(body.reason).toBe('P1 hotfix, no UAT')
    const updated = screen.getByRole('region', { name: 'Definition of Done' })
    expect(within(updated).getByRole('radio', { name: /Custom for this ticket/ })).toBeChecked()
    expect(updated).toHaveTextContent('Reason: P1 hotfix, no UAT')
    expect(updated).toHaveTextContent('Nothing automatic: an agent resolves this ticket by hand.')
    expect(screen.queryByRole('button', { name: 'Resolve without DoD…' })).not.toBeInTheDocument()

    await user.click(within(updated).getByRole('radio', { name: /Project default/ }))
    const reset = await screen.findByRole('alertdialog', { name: "Use the project's Definition of Done again?" })
    await user.click(within(reset).getByRole('button', { name: "Use the project's" }))
    await waitFor(() => expect(lastRequest('PUT', '/tickets/HD-24871/dod')?.body).toEqual({ dod: null }))
    expect(await screen.findByRole('button', { name: 'Resolve without DoD…' })).toBeInTheDocument()
  })

  it('shows the code that named the ticket', async () => {
    await open()
    const code = await screen.findByRole('region', { name: 'Code activity' })
    expect(within(code).getByRole('link', { name: /GitHub · vendor-id\/eoffice-jambi/ })).toHaveAttribute(
      'href',
      'https://github.com/vendor-id/eoffice-jambi',
    )
    const commits = within(code).getByRole('list', { name: 'Commits' })
    expect(within(commits).getAllByRole('listitem')[0]).toHaveTextContent('a1f9c2e')
    expect(within(commits).getByText('Fix HD-24871: revert SSO token TTL to 8h')).toBeInTheDocument()
    // Where the fix has landed so far: its branch, then main.
    const [fixBranches] = within(commits).getAllByRole('list', { name: 'Branches it reached' })
    expect(within(fixBranches!).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['fix/HD-24871-sso', 'main'])
  })

  it('leaves code activity out for a project without a repository and no code', async () => {
    renderApp('/tickets/HD-24859')
    await screen.findByRole('heading', { level: 1 })
    await screen.findByRole('heading', { name: 'Activity log' })
    expect(screen.queryByRole('region', { name: 'Code activity' })).not.toBeInTheDocument()
  })

  it('shows not found for unknown keys', async () => {
    renderApp('/tickets/HD-1')
    expect(await screen.findByRole('heading', { name: 'Ticket not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to board' })).toBeInTheDocument()
  })

  it('sends requesters to the portal', async () => {
    state.me = REQUESTER
    const { router } = renderApp('/tickets/HD-24871')
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal'))
  })
})

describe('live SLA on the ticket', () => {
  const MINUTE = 60_000
  const DAY = 24 * 60 * MINUTE

  function setClocks() {
    const t = tickets.tickets.find((x) => x.key === 'HD-24871')
    if (!t) throw new Error('no fixture')
    t.sla.firstResponseClock = { phase: 'met', usedMs: 8 * MINUTE, budgetMs: 15 * MINUTE, remainingMs: 7 * MINUTE, dayMs: DAY, dueAt: null, metAt: '2026-11-12T01:22:00Z' }
    t.sla.resolutionClock = { phase: 'breached', usedMs: 252 * MINUTE, budgetMs: 240 * MINUTE, remainingMs: -12 * MINUTE, dayMs: DAY, dueAt: '2026-11-12T05:14:00Z', metAt: null }
  }

  it('shows each clock with a bar to agents', async () => {
    state.me = AGENT
    setClocks()
    await open()
    const sla = screen.getByRole('region', { name: 'SLA · P1 · e-Office project' })
    expect(within(sla).getByText('Met in 8m')).toBeInTheDocument()
    expect(within(sla).getByText('Breached 12m')).toBeInTheDocument()
    const bars = within(sla).getAllByRole('meter')
    expect(bars.map((b) => b.getAttribute('aria-valuenow'))).toEqual(['53', '100'])
    expect(bars[1]).toHaveAttribute('aria-valuetext', 'Breached 12m')
  })

  it('shows requesters where things stand, not deadlines', async () => {
    state.me = REQUESTER
    setClocks()
    renderApp('/portal/tickets/HD-24871')
    const card = (await screen.findByRole('heading', { name: 'Service level (P1)' })).closest('section') as HTMLElement
    expect(within(card).getByText('Met · 8m')).toBeInTheDocument()
    expect(within(card).getByText('In progress')).toBeInTheDocument()
    expect(within(card).queryByText(/Breached/)).not.toBeInTheDocument()
  })

  it('says a move pauses the clock only when the project pauses it', async () => {
    state.me = AGENT
    const t = tickets.tickets.find((x) => x.key === 'HD-24871')
    if (t) t.sla.pauseOnWaitingClient = false
    const user = await open()
    await user.click(screen.getByRole('button', { name: /Status$/ }))
    const option = await screen.findByRole('option', { name: /Waiting on client/ })
    expect(option).not.toHaveTextContent('Pauses the SLA clock')
  })
})

describe('a refused status change', () => {
  it('shows the status someone else set, so the next try starts from it', async () => {
    state.me = AGENT
    const user = await open()
    const t = tickets.tickets.find((x) => x.key === 'HD-24871')
    if (t) t.status = 'waiting_on_client' // a teammate moved it meanwhile
    await user.click(screen.getByRole('button', { name: /Status$/ }))
    await user.click(await screen.findByRole('option', { name: /In UAT/ }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: /Status$/ })).toHaveTextContent('Waiting on client'))
  })

  it('reads an overdue clock as breached before the scan flags it', async () => {
    state.me = AGENT
    const t = tickets.tickets.find((x) => x.key === 'HD-24871')
    if (t) {
      t.sla.resolutionClock = {
        phase: 'running', usedMs: 243 * 60_000, budgetMs: 240 * 60_000, remainingMs: -3 * 60_000,
        dayMs: 24 * 3_600_000, dueAt: null, metAt: null,
      }
    }
    await open()
    const sla = screen.getByRole('region', { name: 'SLA · P1 · e-Office project' })
    expect(within(sla).getByText('Breached 3m')).toBeInTheDocument()
  })
})
