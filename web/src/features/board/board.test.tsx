import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { PROJECT_IDS } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'
import { BOARD_COLUMN_LIMIT, TICKET_IDS, tickets } from '../../test/ticketsApi'

const AGENT = USERS['rina@demo-vendor.test'] ?? null

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

async function openDialog() {
  const user = userEvent.setup()
  renderApp('/board')
  await user.click(await screen.findByRole('button', { name: 'Log a ticket' }))
  const dialog = await screen.findByRole('dialog', { name: 'Log a ticket for a client' })
  return { user, dialog }
}

/** A column's card list ("New, 2 tickets"). */
function column(title: string) {
  return screen.getByRole('grid', { name: new RegExp(`^${title}, \\d+ tickets?$`) })
}

function keysIn(title: string) {
  return within(column(title))
    .queryAllByRole('row')
    .map((row) => /HD-\d+/.exec(row.textContent ?? '')?.[0])
    .filter(Boolean) // the empty state is a row too
}


/** Starts a keyboard drag as React Aria offers it: focus the card, arrow to its grip, Enter. */
async function startDrag(user: UserEvent, key: string) {
  const handle = screen.getByRole('button', { name: `Move ${key}` })
  act(() => (handle.closest('[role="row"]') as HTMLElement).focus())
  await user.keyboard('{ArrowRight}')
  expect(handle).toHaveFocus()
  await user.keyboard('{Enter}')
}

/** Drags a card with the keyboard: start on the grip, Tab to a column, Enter. */
async function keyboardMove(user: UserEvent, key: string, to: string) {
  const target = column(to) // before the drag, which hides everything but the drop targets
  await startDrag(user, key)
  for (let i = 0; i < 30 && !target.contains(document.activeElement); i++) await user.keyboard('{Tab}')
  expect(target).toContainElement(document.activeElement as HTMLElement)
  await user.keyboard('{Enter}')
}

describe('board', () => {
  beforeEach(() => {
    state.me = AGENT
  })

  it('shows each column with its cards, owners and counts', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    // Owned tickets nobody started wait in New with the unassigned ones, P1 first.
    expect(keysIn('New')).toEqual(['HD-24910', 'HD-24900'])
    expect(keysIn('In progress')).toEqual(['HD-24871'])
    expect(keysIn('Waiting on client')).toEqual(['HD-24859'])
    expect(keysIn('In UAT')).toEqual(['HD-24915'])
    // Resolved today only; closed tickets are off the board.
    expect(keysIn('Resolved today')).toEqual(['HD-24920'])
    expect(screen.queryByText('Install a PDF reader')).not.toBeInTheDocument()

    const owned = within(column('New')).getAllByRole('row')[0] as HTMLElement
    expect(within(owned).getByText('HD-24910 · Incident')).toBeInTheDocument()
    expect(within(owned).getByRole('img', { name: 'Assignee: Adi Saputra' })).toBeInTheDocument()
    expect(within(owned).getByText('BPKAD Kota Batam')).toBeInTheDocument()
    const openTile = screen.getByText('Open tickets').parentElement as HTMLElement
    expect(within(openTile).getByText('5')).toBeInTheDocument()

    await user.click(within(column('In progress')).getByRole('row', { name: /HD-24871/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/tickets/HD-24871'))
  })

  it('filters by owner and search through the URL', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })

    await user.click(screen.getByRole('button', { name: 'My queue' }))
    await waitFor(() => expect(router.state.location.search).toEqual({ view: 'mine' }))
    expect(screen.getByRole('button', { name: 'My queue' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(lastRequest('GET', '/board')?.search).toContain('assignee=me'))
    expect(await within(column('New')).findByText('Nothing here for this filter.')).toBeInTheDocument()

    await user.type(screen.getByRole('searchbox', { name: 'Search tickets' }), '  e-office ')
    await waitFor(() => expect(router.state.location.search).toEqual({ view: 'mine', q: 'e-office' }))
    expect(lastRequest('GET', '/board')?.search).toContain('q=e-office')
    await waitFor(() => expect(keysIn('Waiting on client')).toEqual([]))
    expect(keysIn('In progress')).toEqual(['HD-24871'])
  })

  it('says when a column is empty or holds more than it shows', async () => {
    const base = tickets.tickets[3]
    if (!base) throw new Error('fixture missing')
    tickets.tickets = tickets.tickets.filter((t) => t.status !== 'in_uat')
    for (let i = 0; i <= BOARD_COLUMN_LIMIT; i++) {
      tickets.tickets.push({ ...base, id: `bulk-${i}`, key: `HD-${30000 + i}`, status: 'new', assignee: null })
    }
    renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    expect(within(column('In UAT')).getByText('No tickets')).toBeInTheDocument()
    expect(screen.getByText(`Showing ${BOARD_COLUMN_LIMIT} of ${BOARD_COLUMN_LIMIT + 3}. Search or filter to narrow it down.`)).toBeInTheDocument()
    expect(within(column('New')).getAllByRole('row')).toHaveLength(BOARD_COLUMN_LIMIT)
  })

  it('moves a card with the keyboard and says so', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    await keyboardMove(user, 'HD-24900', 'In progress')

    expect(await screen.findByText('HD-24900 moved to In progress.')).toBeInTheDocument()
    expect(lastRequest('PUT', '/tickets/HD-24900/status')?.body).toEqual({ status: 'in_progress', from: 'new' })
    expect(keysIn('In progress')).toEqual(['HD-24871', 'HD-24900'])
    expect(keysIn('New')).toEqual(['HD-24910'])
  })

  it('keeps an owned ticket assigned when it goes back to New', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    await keyboardMove(user, 'HD-24871', 'New')
    expect(await screen.findByText('HD-24871 moved to New.')).toBeInTheDocument()
    expect(lastRequest('PUT', '/tickets/HD-24871/status')?.body).toEqual({ status: 'assigned', from: 'in_progress' })
  })

  it('marks the columns a card cannot go to while it is dragged', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    const section = (title: string) => column(title).closest('section') as HTMLElement
    const [inUat, inProgress, fresh] = [section('In UAT'), section('In progress'), section('New')]
    await startDrag(user, 'HD-24900')
    expect(await within(inUat).findByText("Can't move here")).toBeInTheDocument()
    expect(within(inProgress).queryByText("Can't move here")).not.toBeInTheDocument()
    expect(within(fresh).queryByText("Can't move here")).not.toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByText("Can't move here")).not.toBeInTheDocument())
    expect(lastRequest('PUT', '/tickets/HD-24900/status')).toBeUndefined()
  })

  it('puts the card back when someone else moved the ticket first', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    await screen.findByRole('region', { name: 'Ticket board' })
    const ticket = tickets.tickets.find((t) => t.key === 'HD-24900')
    if (!ticket) throw new Error('fixture missing')
    ticket.status = 'waiting_on_client' // another agent, before this board refreshed

    await keyboardMove(user, 'HD-24900', 'In progress')
    expect(await screen.findByText("Someone else changed this ticket's status just now. Showing the latest.")).toBeInTheDocument()
    await waitFor(() => expect(keysIn('Waiting on client')).toEqual(['HD-24859', 'HD-24900']))
    expect(keysIn('In progress')).toEqual(['HD-24871'])
  })

  it('logs a ticket for an existing requester', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: 'Create ticket' }))
    expect(await within(dialog).findByText('Choose a client.')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    const requester = within(dialog).getByRole('combobox', { name: 'Requester' })
    await user.type(requester, 'hen')
    await user.click(await screen.findByRole('option', { name: /Hendra Gunawan/ }))
    await user.click(within(dialog).getByRole('button', { name: /Which system\?/ }))
    await user.click(await screen.findByRole('option', { name: /e-Office/ }))
    await user.type(within(dialog).getByRole('textbox', { name: 'Subject' }), 'Called about e-Office')
    await user.type(within(dialog).getByRole('textbox', { name: 'Details' }), 'Login fails.')
    await user.click(within(dialog).getByRole('button', { name: 'Create ticket' }))

    expect(await screen.findByText('HD-24892 created and assigned to you.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets')?.body).toEqual({
      clientId: IDS.jambi,
      requesterId: TICKET_IDS.hendra,
      channel: 'phone',
      type: 'incident',
      priority: 'P3',
      subject: 'Called about e-Office',
      description: 'Login fails.',
      projectId: PROJECT_IDS.eOffice,
      attachmentIds: [],
      assignToMe: true,
    })
    expect(screen.getByRole('link', { name: 'Open HD-24892' })).toHaveAttribute('href', '/tickets/HD-24892')
  })

  it('adds a new requester inline', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    await user.type(within(dialog).getByRole('combobox', { name: 'Requester' }), 'Siti Rahma')
    await user.click(await screen.findByRole('option', { name: 'Add “Siti Rahma” as a new requester' }))

    const group = within(dialog).getByRole('group', { name: 'New requester' })
    expect(within(group).getByRole('textbox', { name: 'Name' })).toHaveValue('Siti Rahma')
    const email = within(group).getByRole('textbox', { name: 'Email' })
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    expect(await within(group).findByText('Enter a valid email address.')).toBeInTheDocument()

    await user.type(email, 'hendra@kominfo-jambi.test')
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    expect(await within(group).findByText('Someone already uses this email address.')).toBeInTheDocument()

    await user.clear(email)
    await user.type(email, 'siti@kominfo-jambi.test')
    await user.click(within(group).getByRole('button', { name: 'Add requester' }))
    await waitFor(() => expect(within(dialog).getByRole('combobox', { name: 'Requester' })).toHaveValue('Siti Rahma'))
    expect(lastRequest('POST', `/clients/${IDS.jambi}/requesters`)?.body).toEqual({ name: 'Siti Rahma', email: 'siti@kominfo-jambi.test' })
  })

  it('clears the requester when the client changes', async () => {
    const { user, dialog } = await openDialog()
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /Dinas Kominfo Jambi/ }))
    await user.type(within(dialog).getByRole('combobox', { name: 'Requester' }), 'dian')
    await user.click(await screen.findByRole('option', { name: /Dian Puspita/ }))
    await user.click(within(dialog).getByRole('button', { name: /Client$/ }))
    await user.click(await screen.findByRole('option', { name: /BPKAD Kota Batam/ }))
    expect(within(dialog).getByRole('combobox', { name: 'Requester' })).toHaveValue('')
  })
})
