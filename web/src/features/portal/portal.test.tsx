import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { PROJECT_IDS, projectsApi } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { writeRichText } from '../../test/richText'
import { PASSWORD, state, USERS } from '../../test/server'
import { IDS } from '../../test/settingsApi'
import { ticket, tickets } from '../../test/ticketsApi'

const REQUESTER = USERS['hendra@kominfo-jambi.test'] ?? null
const AGENT = USERS['rina@demo-vendor.test'] ?? null
const MB = 1024 * 1024

function lastRequest(method: string, path: string) {
  return [...tickets.requests].reverse().find((r) => r.method === method && r.path.endsWith(path))
}

const png = (name = 'screen.png', size = 2048) => new File([new Uint8Array(size)], name, { type: 'image/png' })

/** React Aria's FileTrigger renders a hidden file input next to its button. */
function fileInput(): HTMLInputElement {
  const input = document.body.querySelector<HTMLInputElement>('input[type="file"]')
  if (!input) throw new Error('no file input')
  return input
}

describe('portal home', () => {
  beforeEach(() => {
    state.me = REQUESTER
  })

  it("lists the client's open tickets and filters by status", async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/portal')
    const list = await screen.findByRole('heading', { name: 'My tickets' })
    const card = list.closest('section') as HTMLElement
    expect(within(card).getByRole('button', { name: 'Open' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(card).getByText('e-Office login fails for all staff')).toBeInTheDocument()
    expect(within(card).getByText('Waiting on you')).toBeInTheDocument()
    expect(within(card).queryByText('Install a PDF reader')).not.toBeInTheDocument()
    expect(within(card).queryByText('SIPD export times out')).not.toBeInTheDocument()

    await user.click(within(card).getByRole('button', { name: 'Resolved' }))
    await waitFor(() => expect(router.state.location.search).toEqual({ filter: 'resolved' }))
    expect(await within(card).findByText('Install a PDF reader')).toBeInTheDocument()
    expect(lastRequest('GET', '/tickets')?.search).toBe('?status=resolved&status=closed&limit=50')

    await user.click(within(card).getByRole('link', { name: 'View details of HD-24790' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal/tickets/HD-24790'))
  })

  it('keeps agents out of the portal', async () => {
    state.me = AGENT
    const { router } = renderApp('/portal/new')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })
})

describe('report an issue', () => {
  beforeEach(() => {
    state.me = REQUESTER
  })

  it("shows the client's hours and the priority cards for each type", async () => {
    const user = userEvent.setup()
    renderApp('/portal/new')
    expect(await screen.findByRole('heading', { name: 'How can we help?' })).toBeInTheDocument()
    expect(screen.getByText(/Our team works 08:00 – 17:00 WIB, Monday – Friday\./)).toBeInTheDocument()
    // Times depend on the system's project, so they wait for the choice.
    expect(screen.getByText('Choose the system first to see how fast each priority is handled.')).toBeInTheDocument()
    expect(screen.queryByText(/First reply:/)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Which system\?/ }))
    await user.click(await screen.findByRole('option', { name: /e-Office/ }))
    expect(screen.getByText(/P1 issues are handled 24\/7\./)).toBeInTheDocument()
    const p1 = screen.getByRole('radio', { name: /Critical/ }).closest('label') as HTMLElement
    expect(within(p1).getByText('First reply: 15 min')).toBeInTheDocument()
    expect(within(p1).getByText('Fixed within: 2 hours')).toBeInTheDocument()
    expect(within(p1).getByText('Handled 24/7')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Which system\?/ }))
    await user.click(await screen.findByRole('option', { name: /Email/ }))
    expect(within(p1).getByText('Fixed within: 4 hours')).toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: /I need something new/ }))
    expect(screen.getByRole('radio', { name: /Urgent/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /What do you need\?/ }))
    await user.click(await screen.findByRole('option', { name: /VPN \/ remote access/ }))
    const p3 = screen.getByRole('radio', { name: /Standard/ }).closest('label') as HTMLElement
    expect(within(p3).getByText('Delivered within: 2 working days')).toBeInTheDocument()
  })

  it('needs every field before sending anything', async () => {
    const user = userEvent.setup()
    renderApp('/portal/new')
    await user.click(await screen.findByRole('button', { name: 'Submit ticket' }))
    expect(await screen.findByText('Enter a short subject.')).toBeInTheDocument()
    expect(screen.getByText('Choose a system.')).toBeInTheDocument()
    expect(screen.getByText('Choose a priority.')).toBeInTheDocument()
    expect(screen.getByText('Tell us what happened.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets')).toBeUndefined()
  })

  it('submits the report with its file and shows the new key', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/portal/new')
    await user.type(await screen.findByRole('textbox', { name: 'Subject' }), 'Printer offline')
    await user.click(screen.getByRole('button', { name: /Which system\?/ }))
    await user.click(await screen.findByRole('option', { name: /e-Office/ }))
    await user.click(screen.getByRole('radio', { name: /High/ }))
    expect(screen.getByText(/First reply within 30 min · fix target 8 hours/)).toBeInTheDocument()
    await writeRichText(user, screen.getByRole('textbox', { name: 'Details' }), 'Since this morning.')
    await screen.findByRole('button', { name: 'Attach screenshot or file' })
    await user.upload(fileInput(), png())
    await waitFor(() => expect(tickets.uploads).toHaveLength(1))
    await waitFor(() => expect(screen.queryByText('Uploading…')).not.toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Submit ticket' }))
    const heading = await screen.findByRole('heading', { name: 'Ticket HD-24892 received' })
    expect(heading).toHaveFocus()
    const body = lastRequest('POST', '/tickets')?.body as Record<string, unknown>
    expect(body).toMatchObject({
      type: 'incident',
      priority: 'P2',
      subject: 'Printer offline',
      description: 'Since this morning.',
      projectId: PROJECT_IDS.eOffice,
    })
    expect(body.attachmentIds).toHaveLength(1)

    await user.click(screen.getByRole('link', { name: 'View ticket' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal/tickets/HD-24892'))
  })

  it('refuses files over the limit, of the wrong type, or too many, without uploading them', async () => {
    const user = userEvent.setup({ applyAccept: false })
    renderApp('/portal/new')
    await screen.findByRole('button', { name: 'Attach screenshot or file' })
    const input = fileInput()
    await user.upload(input, png('huge.png', 11 * MB))
    expect(await screen.findByText('Larger than 10 MB')).toBeInTheDocument()
    await user.upload(input, new File(['MZ'], 'setup.exe', { type: 'application/octet-stream' }))
    expect(await screen.findByText("This file type isn't allowed")).toBeInTheDocument()
    expect(tickets.uploads).toHaveLength(0)

    await user.upload(input, [png('a.png'), png('b.png'), png('c.png')])
    expect(await screen.findByText('Up to 2 files per message')).toBeInTheDocument()
    await waitFor(() => expect(tickets.uploads).toHaveLength(2))
    await user.click(screen.getByRole('button', { name: 'Remove huge.png' }))
    expect(screen.queryByText('huge.png')).not.toBeInTheDocument()
  })
})

describe('report an issue without a project', () => {
  beforeEach(() => {
    state.me = REQUESTER
    projectsApi.projects = projectsApi.projects.filter((p) => p.client.id !== IDS.jambi)
  })

  it('explains that reporting is not set up yet', async () => {
    renderApp('/portal/new')
    expect(await screen.findByRole('heading', { name: "Reporting isn't set up yet" })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit ticket' })).not.toBeInTheDocument()
  })
})

describe('requester ticket detail', () => {
  beforeEach(() => {
    state.me = REQUESTER
  })

  it('shows progress and the public conversation only', async () => {
    renderApp('/portal/tickets/HD-24871')
    expect(await screen.findByRole('heading', { level: 1, name: 'e-Office login fails for all staff' })).toBeInTheDocument()
    expect(screen.getByText('In progress', { selector: 'span.rounded-full' })).toBeInTheDocument()
    const tracker = screen.getByRole('list', { name: 'Progress' })
    const current = within(tracker).getAllByRole('listitem').filter((li) => li.getAttribute('aria-current') === 'step')
    expect(current).toHaveLength(1)
    expect(current[0]).toHaveTextContent('In progress')
    expect(screen.getByText('Thanks Pak Hendra, we are on it.')).toBeInTheDocument()
    expect(screen.queryByText(/SSO patch/)).not.toBeInTheDocument()
    expect(screen.queryByText('rollback-plan.txt')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /screenshot-login-error\.png/ })).toHaveAttribute('href', '/api/v1/attachments/a-shot')
    expect(tickets.requests.some((r) => r.path.endsWith('/activity'))).toBe(false)
  })

  it('posts a reply', async () => {
    const user = userEvent.setup()
    renderApp('/portal/tickets/HD-24871')
    const send = await screen.findByRole('button', { name: 'Send reply' })
    await user.click(send)
    expect(await screen.findByText('Write a message first.')).toBeInTheDocument()
    await writeRichText(user, screen.getByRole('textbox', { name: 'Add a reply' }), 'Still failing on my PC')
    await user.click(send)
    expect(await screen.findByText('Still failing on my PC')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets/HD-24871/messages')?.body).toEqual({
      visibility: 'public',
      body: 'Still failing on my PC',
      attachmentIds: [],
    })
  })

  it("can't open another client's ticket", async () => {
    renderApp('/portal/tickets/HD-24900')
    expect(await screen.findByRole('heading', { name: 'Ticket not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to my tickets' })).toBeInTheDocument()
  })
})

describe('people notified', () => {
  beforeEach(() => {
    state.me = REQUESTER
  })

  it('adds and removes a colleague', async () => {
    const user = userEvent.setup()
    renderApp('/portal/tickets/HD-24871')
    const heading = await screen.findByRole('heading', { name: 'People notified' })
    const card = heading.closest('section') as HTMLElement
    expect(within(card).getByText('Hendra Gunawan (you)')).toBeInTheDocument()

    await user.click(within(card).getByRole('button', { name: 'Add a colleague' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add a colleague' })
    await user.click(within(dialog).getByRole('button', { name: 'Add' }))
    expect(await within(dialog).findByText('Choose a colleague.')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('combobox', { name: /Colleague/ }))
    await user.click(await screen.findByRole('option', { name: /Dian Puspita/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(await within(card).findByText('Dian Puspita')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets/HD-24871/watchers')?.body).toEqual({ userId: tickets.requesters[IDS.jambi]?.[1]?.id })

    await user.click(within(card).getByRole('button', { name: 'Remove Dian Puspita' }))
    await waitFor(() => expect(within(card).queryByText('Dian Puspita')).not.toBeInTheDocument())
    expect(lastRequest('DELETE', '/watchers/' + tickets.requesters[IDS.jambi]?.[1]?.id)).toBeDefined()
  })
})

const INSTRUCTIONS = 'The fix is on the e-Office test server. Create a disposition letter and check the date on the PDF.'
const IN_A_FEW_DAYS = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString()

/** HD-24830 waits In UAT for Pak Hendra's test (PortalHome mock). */
function addTestRequest() {
  tickets.tickets.push(
    ticket({
      id: 't-24830',
      key: 'HD-24830',
      subject: 'Disposition letter PDF shows wrong date',
      status: 'in_uat',
      assignee: { id: IDS.adi, name: 'Adi Saputra' },
      uat: { autoApproveAt: IN_A_FEW_DAYS, canAnswer: true, instructions: INSTRUCTIONS },
      progress: [
        { step: 'received', state: 'done', at: '2026-11-12T01:14:00Z' },
        { step: 'assigned', state: 'done', at: '2026-11-12T01:16:00Z' },
        { step: 'in_progress', state: 'done', at: '2026-11-12T01:52:00Z' },
        { step: 'ready_for_test', state: 'current', at: '2026-11-12T05:40:00Z' },
        { step: 'resolved', state: 'upcoming', at: null },
      ],
    }),
  )
}

/** HD-24834 was resolved and can still be reopened. */
function addResolved() {
  tickets.tickets.push(
    ticket({
      id: 't-24834',
      key: 'HD-24834',
      subject: "News page still shows last month's banner",
      status: 'resolved',
      assignee: { id: IDS.rina, name: 'Rina Pratiwi' },
      resolvedAt: '2026-11-15T03:00:00Z',
      reopen: { until: IN_A_FEW_DAYS, canReopen: true },
      progress: [
        { step: 'received', state: 'done', at: '2026-11-14T01:14:00Z' },
        { step: 'assigned', state: 'done', at: '2026-11-14T01:16:00Z' },
        { step: 'in_progress', state: 'done', at: '2026-11-14T01:52:00Z' },
        { step: 'ready_for_test', state: 'skipped', at: null },
        { step: 'resolved', state: 'done', at: '2026-11-15T03:00:00Z' },
      ],
    }),
  )
}

describe('client tests (UAT) and reopening', () => {
  beforeEach(() => {
    state.me = REQUESTER
  })

  it('lists the tests waiting on the requester and approves one from the home page', async () => {
    addTestRequest()
    const user = userEvent.setup()
    renderApp('/portal')
    const section = (await screen.findByRole('heading', { name: 'Waiting for your testing (UAT)' })).closest('section') as HTMLElement
    expect(screen.getByText('1 fix is waiting for you to test.')).toBeInTheDocument()
    const card = within(section).getByRole('listitem')
    expect(card).toHaveTextContent('HD-24830 · e-Office')
    expect(card).toHaveTextContent(/Auto-approves \d+ \w+/)
    expect(card).toHaveTextContent(INSTRUCTIONS)
    expect(within(card).getByRole('link', { name: 'Disposition letter PDF shows wrong date' })).toHaveAttribute(
      'href',
      '/portal/tickets/HD-24830',
    )

    await user.click(within(card).getByRole('button', { name: 'Approve, it works' }))
    const dialog = await screen.findByRole('alertdialog', { name: 'Approve the fix for HD-24830?' })
    await user.click(within(dialog).getByRole('button', { name: 'Yes, approve' }))
    expect(await screen.findByText('Thank you. HD-24830 is now resolved.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets/HD-24830/uat')?.body).toEqual({ answer: 'approve' })
    await waitFor(() => expect(within(section).queryByRole('listitem')).not.toBeInTheDocument())
  })

  it('sends a fix back from the ticket page with what went wrong', async () => {
    addTestRequest()
    const user = userEvent.setup()
    renderApp('/portal/tickets/HD-24830')
    const banner = (await screen.findByRole('heading', { name: 'Your test is needed' })).closest('section') as HTMLElement
    expect(banner).toHaveTextContent(INSTRUCTIONS)
    expect(banner).toHaveTextContent(/If we don't hear back, it's approved automatically on \d+ \w+\./)
    const tracker = screen.getByRole('list', { name: 'Progress' })
    expect(within(tracker).getByText('Waiting for you')).toBeInTheDocument()

    await user.click(within(banner).getByRole('button', { name: 'Still a problem' }))
    const dialog = await screen.findByRole('alertdialog', { name: 'Send HD-24830 back to the team?' })
    const send = within(dialog).getByRole('button', { name: 'Send back' })
    expect(send).toBeDisabled()
    await user.type(within(dialog).getByRole('textbox', { name: /What went wrong/ }), 'The date is still one day behind')
    await user.click(send)
    expect(await screen.findByText('Sent back to the team with what you saw.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets/HD-24830/uat')?.body).toEqual({
      answer: 'reject',
      message: 'The date is still one day behind',
    })
    expect(screen.queryByRole('heading', { name: 'Your test is needed' })).not.toBeInTheDocument()
    expect(await screen.findByText('The date is still one day behind')).toBeInTheDocument()
  })

  it("opens the email button's answer after signing in", async () => {
    addTestRequest()
    state.me = null
    const user = userEvent.setup()
    const { router } = renderApp('/portal/tickets/HD-24830?answer=reject')
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    await user.type(await screen.findByLabelText('Work email'), 'hendra@kominfo-jambi.test')
    await user.type(screen.getByLabelText('Password'), PASSWORD)
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByRole('alertdialog', { name: 'Send HD-24830 back to the team?' })).toBeInTheDocument()
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal/tickets/HD-24830'))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  it("says so when the email's ticket no longer waits for a test", async () => {
    renderApp('/portal/tickets/HD-24871?answer=approve')
    expect(
      await screen.findByText("This ticket isn't waiting for your test any more. Someone may have answered already."),
    ).toBeInTheDocument()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('reopens a resolved ticket inside its window', async () => {
    addResolved()
    const user = userEvent.setup()
    renderApp('/portal/tickets/HD-24834')
    expect(await screen.findByText(/^Resolved\. You can reopen this ticket until \d+ \w+ if the problem comes back\.$/)).toBeInTheDocument()
    expect(screen.getByText("Replying doesn't reopen the ticket. Use Reopen ticket if the problem is back.")).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Reopen ticket' }))
    const dialog = await screen.findByRole('alertdialog', { name: 'Reopen HD-24834?' })
    await user.click(within(dialog).getByRole('button', { name: 'Reopen' }))
    expect(await screen.findByText('HD-24834 is back with the support team.')).toBeInTheDocument()
    expect(lastRequest('POST', '/tickets/HD-24834/reopen')).toBeDefined()
    expect(screen.queryByRole('button', { name: 'Reopen ticket' })).not.toBeInTheDocument()
  })
})
