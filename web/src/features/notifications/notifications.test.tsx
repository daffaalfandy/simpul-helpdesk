import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { NOTICE_IDS, notifications } from '../../test/notificationsApi'
import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'

const AGENT = USERS['rina@demo-vendor.test'] ?? null
const REQUESTER = USERS['hendra@kominfo-jambi.test'] ?? null
const SUPERVISOR = USERS['dewi@demo-vendor.test'] ?? null

function writes(path: string) {
  return notifications.requests.filter((r) => r.path.endsWith(path))
}

describe('notification bell', () => {
  beforeEach(() => {
    state.me = AGENT
  })

  it('shows the unread count, opens the newest items and closes with Esc', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    const bell = await screen.findByRole('button', { name: 'Notifications, 2 unread' })
    await user.click(bell)
    const panel = await screen.findByRole('dialog', { name: 'Notifications' })
    expect(await within(panel).findByRole('link', { name: /Unread: HD-24871 · e-Office login fails: Hendra Gunawan replied/ })).toBeInTheDocument()
    expect(within(panel).getByText('“Two of three staff can sign in now, one still sees the error.”')).toBeInTheDocument()
    expect(within(panel).getByRole('link', { name: 'Notification settings' })).toHaveAttribute('href', '/notifications#settings')

    await user.click(within(panel).getByRole('button', { name: 'Unread (2)' }))
    expect(within(panel).queryByText(/Adi Saputra changed priority/)).not.toBeInTheDocument()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(bell).toHaveFocus())
  })

  it('marks everything read', async () => {
    const user = userEvent.setup()
    renderApp('/board')
    await user.click(await screen.findByRole('button', { name: 'Notifications, 2 unread' }))
    const panel = await screen.findByRole('dialog', { name: 'Notifications' })
    await user.click(within(panel).getByRole('button', { name: 'Mark all as read' }))
    await waitFor(() => expect(within(panel).getByRole('button', { name: 'Mark all as read' })).toBeDisabled())
    expect(within(panel).getByRole('button', { name: 'Unread (0)' })).toBeInTheDocument()
    expect(writes('/read-all')).toHaveLength(1)
    // The page behind the open panel is hidden from assistive tech, hence hidden: true.
    expect(await screen.findByRole('button', { name: 'Notifications', hidden: true })).toBeInTheDocument()
  })

  it('opens the ticket and marks the item read', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    await user.click(await screen.findByRole('button', { name: 'Notifications, 2 unread' }))
    const panel = await screen.findByRole('dialog', { name: 'Notifications' })
    await user.click(await within(panel).findByRole('link', { name: /Hendra Gunawan replied/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/tickets/HD-24871'))
    expect(writes(`/${NOTICE_IDS.reply}/read`)[0]?.body).toEqual({ read: true })
  })

  it('is in the portal header too', async () => {
    state.me = REQUESTER
    renderApp('/portal')
    expect(await screen.findByRole('button', { name: 'Notifications, 2 unread' })).toBeInTheDocument()
  })
})

describe('notifications page', () => {
  beforeEach(() => {
    state.me = AGENT
  })

  async function open(url = '/notifications') {
    const user = userEvent.setup()
    const view = renderApp(url)
    await screen.findByRole('heading', { name: 'Notifications', level: 1 })
    return { user, ...view }
  }

  it('lists the inbox with filters and toggles read state', async () => {
    const { user } = await open()
    expect(screen.getByText("Updates on tickets you're assigned to, plus anything you need to act on.")).toBeInTheDocument()
    const filters = screen.getByRole('group', { name: 'Filter notifications' })
    await within(filters).findByRole('button', { name: 'Unread (2)' })
    expect(within(filters).getAllByRole('button').map((b) => b.textContent)).toEqual([
      'All',
      'Unread (2)',
      'Comments',
      'Status',
      'Assigned',
      'SLA',
      'Code',
    ])
    const inbox = screen.getByRole('list')
    expect(within(inbox).getByText('Adi Saputra changed priority: P3 → P2')).toBeInTheDocument()
    expect(within(inbox).getByText('“The whole team is blocked.”')).toBeInTheDocument()
    expect(within(inbox).getByText(/· Web$/)).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: 'Mark as read' })[0] as HTMLElement)
    expect(writes(`/${NOTICE_IDS.reply}/read`)[0]?.body).toEqual({ read: true })
    const priority = screen.getByText('Adi Saputra changed priority: P3 → P2').closest('li') as HTMLElement
    await user.click(within(priority).getByRole('button', { name: 'Mark as unread' }))
    expect(writes(`/${NOTICE_IDS.priority}/read`)[0]?.body).toEqual({ read: false })

    await user.click(within(filters).getByRole('button', { name: 'Assigned' }))
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1))
    expect(screen.getByText('Dewi Wulandari assigned this ticket to you')).toBeInTheDocument()
    await user.click(within(filters).getByRole('button', { name: 'SLA' }))
    expect(await screen.findByText('Nothing here.')).toBeInTheDocument()
  })

  it('saves each switch at once and keeps always-on events on', async () => {
    const { user } = await open()
    const settings = screen.getByRole('table', { name: 'Where each notification goes' })
    expect(within(settings).getByRole('switch', { name: 'Web notifications for SLA at risk and breached' })).toBeDisabled()
    expect(within(settings).getByRole('switch', { name: 'Email for Assigned to me or unassigned' })).toBeChecked()

    const noteEmail = within(settings).getByRole('switch', { name: 'Email for Internal notes' })
    expect(noteEmail).not.toBeChecked()
    await user.click(noteEmail)
    expect(await screen.findByText('Saved.')).toBeInTheDocument()
    expect(noteEmail).toBeChecked()
    expect(writes('/events/internal_note')[0]?.body).toEqual({ web: true, email: true })
    expect(screen.getByText('Assignments and SLA alerts are always on, so nothing slips past its deadline.')).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: /Never notify me about my own changes/ })).toBeDisabled()
    expect(within(settings).getByRole('rowheader', { name: /Ticket updates/ })).toHaveTextContent(
      'Status changes, including moves on the board',
    )
    // New tickets nobody is on are a supervisor's row.
    expect(within(settings).queryByText('New tickets')).not.toBeInTheDocument()
  })

  it('tells supervisors about tickets nobody is assigned to', async () => {
    state.me = SUPERVISOR
    const { user } = await open()
    const settings = screen.getByRole('table', { name: 'Where each notification goes' })
    const rows = within(settings).getAllByRole('rowheader').map((r) => r.textContent)
    expect(rows.slice(3, 5)).toEqual(['Assigned to me or unassignedAlways on', 'New ticketsTickets nobody is assigned to yet'])
    const newTicketsEmail = within(settings).getByRole('switch', { name: 'Email for New tickets' })
    expect(newTicketsEmail).toBeEnabled()
    await user.click(newTicketsEmail)
    expect(await screen.findByText('Saved.')).toBeInTheDocument()
    expect(writes('/events/new_ticket')[0]?.body).toEqual({ web: true, email: false })

    const inbox = screen.getByRole('list')
    expect(within(inbox).getByText('Hendra Gunawan opened this ticket; nobody is assigned yet')).toBeInTheDocument()
    const filters = screen.getByRole('group', { name: 'Filter notifications' })
    await user.click(within(filters).getByRole('button', { name: 'Assigned' }))
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2))
    expect(screen.getByRole('link', { name: /HD-24898 · Scanner jams on every page/ })).toHaveAttribute('href', '/tickets/HD-24898')
  })

  it('puts a switch back when the save fails', async () => {
    const { user } = await open()
    notifications.failNextSave = true
    const commentWeb = screen.getByRole('switch', { name: 'Web notifications for Comments and replies' })
    await user.click(commentWeb)
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    await waitFor(() => expect(commentWeb).toBeChecked())
  })

  it('chooses when emails go out', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: /Send emails/ }))
    await user.click(await screen.findByRole('option', { name: /Hourly summary/ }))
    expect(await screen.findByText('Saved.')).toBeInTheDocument()
    expect(writes('/email-frequency')[0]?.body).toEqual({ emailFrequency: 'hourly' })
  })

  it('opens at the settings from the bell link', async () => {
    await open('/notifications#settings')
    await waitFor(() => expect(document.getElementById('settings')).toHaveFocus())
  })

  it('uses the portal wording for requesters', async () => {
    state.me = REQUESTER
    await open('/portal/notifications')
    expect(screen.getByText('Updates on tickets you reported or were added to.')).toBeInTheDocument()
    const filters = screen.getByRole('group', { name: 'Filter notifications' })
    await within(filters).findByRole('button', { name: 'Unread (2)' })
    expect(within(filters).getAllByRole('button').map((b) => b.textContent)).toEqual(['All', 'Unread (2)', 'Replies', 'Status'])
    expect(screen.getByText('Yoga Hidayat is waiting for your reply')).toBeInTheDocument()
    expect(screen.getByText('Status changed: In progress → Ready for your test')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /HD-24812/ })).toHaveAttribute('href', '/portal/tickets/HD-24812')
    expect(screen.queryByText(/always on, so nothing slips/)).not.toBeInTheDocument()
    expect(
      screen.getByText('UAT requests and “Waiting for your reply” are always on, because the ticket waits for your answer.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Includes the 3-working-day reopen window')).toBeInTheDocument()
    expect(screen.queryByText(/notifications\.settings/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('switch')).toHaveLength(12)
    expect(screen.getByRole('switch', { name: 'Web notifications for Ready for your test (UAT)' })).toBeDisabled()
    expect(screen.getByRole('switch', { name: 'Email for Waiting for your reply' })).toBeDisabled()
    expect(screen.getByText('The team needs your reply or confirmation to continue')).toBeInTheDocument()
  })
})
