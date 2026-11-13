import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'
import { TICKET_IDS, tickets } from '../../test/ticketsApi'

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
    expect(screen.getByText(/P1 issues are handled 24\/7\./)).toBeInTheDocument()

    const p1 = screen.getByRole('radio', { name: /Critical/ }).closest('label') as HTMLElement
    expect(within(p1).getByText('First reply: 15 min')).toBeInTheDocument()
    expect(within(p1).getByText('Fixed within: 4 hours')).toBeInTheDocument()
    expect(within(p1).getByText('Handled 24/7')).toBeInTheDocument()

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
    await user.type(screen.getByRole('textbox', { name: 'Details' }), 'Since this morning.')
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
      systemId: TICKET_IDS.eoffice,
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
    await user.type(screen.getByRole('textbox', { name: 'Add a reply' }), 'Still failing on my PC')
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
