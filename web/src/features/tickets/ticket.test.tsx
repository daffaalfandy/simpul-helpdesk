import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { PROJECT_IDS } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
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
    expect(sla).toHaveTextContent("Runs 24/7 Time waiting for the client's reply or test doesn't count.")
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
    await user.type(screen.getByRole('textbox', { name: 'Note for the support team only' }), 'Ask Adi to revert')
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
    await waitFor(() => expect(lastRequest('PUT', '/tickets/HD-24871/status')?.body).toEqual({ status: 'waiting_on_client' }))
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
