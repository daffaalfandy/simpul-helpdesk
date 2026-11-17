import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Problem } from '../../api/client'
import { renderApp } from '../../test/renderApp'
import { ORG_ID, ORGANIZATION_HEADER, PASSWORD, SECOND_ORG, server, state, USERS } from '../../test/server'

const SARI = USERS['sari@demo-vendor.test'] ?? null
const NOBODY = USERS['yudi@demo-vendor.test'] ?? null
const TICKET = 'HD-24871'

async function signIn(email: string) {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('Work email'), email)
  await user.type(screen.getByLabelText('Password'), PASSWORD)
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
  return user
}

describe('choosing an organization after sign-in', () => {
  it('asks people who work in several, then opens the one they pick', async () => {
    const { router } = renderApp('/login')
    const user = await signIn('sari@demo-vendor.test')
    expect(await screen.findByRole('heading', { name: 'Choose an organization' })).toBeInTheDocument()
    const list = within(screen.getByRole('list', { name: 'Your organizations' }))
    expect(list.getByRole('button', { name: 'Demo Vendor, Agent' })).toBeInTheDocument()

    await user.click(list.getByRole('button', { name: 'PT Mitra Data, Supervisor' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
    expect(await screen.findByRole('button', { name: 'Switch organization, now PT Mitra Data' })).toBeInTheDocument()
    expect(state.switches).toEqual(['PT Mitra Data'])
    expect(state.orgHeaders.at(-1)).toBe(SECOND_ORG.id)
  })

  it('keeps the page they were going to', async () => {
    const { router } = renderApp('/login?redirect=%2Fnotifications')
    const user = await signIn('sari@demo-vendor.test')
    await user.click(await screen.findByRole('button', { name: 'Demo Vendor, Agent' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/notifications'))
  })

  it('is skipped by people in one organization', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/select-organization')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })
})

describe('switching from the header', () => {
  it('moves the tab to the other organization and drops what it showed', async () => {
    state.me = SARI
    const user = userEvent.setup()
    renderApp('/board')
    await user.click(await screen.findByRole('button', { name: 'Switch organization, now Demo Vendor' }))
    const menu = within(await screen.findByRole('menu', { name: 'Switch organization, now Demo Vendor' }))
    expect(menu.getByRole('menuitem', { name: 'Demo Vendor, Agent' })).toBeInTheDocument()
    expect(menu.queryByRole('menuitem', { name: 'Manage organizations' })).not.toBeInTheDocument()

    await user.click(menu.getByRole('menuitem', { name: 'PT Mitra Data, Supervisor' }))
    expect(await screen.findByRole('button', { name: 'Switch organization, now PT Mitra Data' })).toBeInTheDocument()
    // Supervisors there see the reports.
    expect(screen.getByRole('link', { name: 'Reports' })).toBeInTheDocument()
    expect(state.orgHeaders).toEqual([ORG_ID, SECOND_ORG.id])
  })

  it('shows a plain brand to people in one organization', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    renderApp('/board')
    expect(await screen.findByText('Demo Vendor')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Switch organization/ })).not.toBeInTheDocument()
  })

  it('takes administrators to the organizations they manage', async () => {
    state.me = USERS['admin@demo-vendor.test'] ?? null
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    await user.click(await screen.findByRole('button', { name: 'Switch organization, now Demo Vendor' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Manage organizations' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/organizations'))
  })
})

describe('a ticket in another organization', () => {
  it('offers the switch, then opens the ticket there', async () => {
    state.me = SARI
    const elsewhere: Problem = {
      type: 'about:blank', title: 'other_organization', status: 409, code: 'other_organization', detail: SECOND_ORG.id,
    }
    // The ticket lives in PT Mitra Data: asked from anywhere else, the API names it.
    server.use(
      http.get(`*/api/v1/tickets/${TICKET}`, ({ request }) =>
        request.headers.get(ORGANIZATION_HEADER) === SECOND_ORG.id ? undefined : HttpResponse.json(elsewhere, { status: 409 }),
      ),
    )
    const user = userEvent.setup()
    renderApp(`/tickets/${TICKET}`)
    expect(await screen.findByRole('heading', { name: 'This ticket is in PT Mitra Data' })).toBeInTheDocument()
    expect(screen.getByText('You’re working in Demo Vendor. Switch this tab to PT Mitra Data to open it.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Switch to PT Mitra Data' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'e-Office login fails for all staff' })).toBeInTheDocument()
    expect(state.switches).toEqual(['PT Mitra Data'])
  })

  it('is plainly not found when the user doesn’t work there', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    server.use(
      http.get(`*/api/v1/tickets/${TICKET}`, () =>
        HttpResponse.json({ type: 'about:blank', title: 'x', status: 409, code: 'other_organization', detail: SECOND_ORG.id }, { status: 409 }),
      ),
    )
    renderApp(`/tickets/${TICKET}`)
    expect(await screen.findByRole('heading', { name: 'Ticket not found' })).toBeInTheDocument()
  })
})

describe('staff in no organization', () => {
  it('wait on their own page until an administrator adds them', async () => {
    state.me = NOBODY
    const user = userEvent.setup()
    const { router } = renderApp('/board')
    expect(await screen.findByRole('heading', { name: 'You’re not in any organization yet' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Check again' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Still no organization.')

    const choice = { id: ORG_ID, name: 'Demo Vendor', role: 'agent' as const, logoUrl: null }
    state.me = { ...NOBODY!, role: 'agent', organization: choice, organizations: [choice] }
    await user.click(screen.getByRole('button', { name: 'Check again' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })

  it('can sign out', async () => {
    state.me = NOBODY
    const user = userEvent.setup()
    const { router } = renderApp('/no-organization')
    await user.click(await screen.findByRole('button', { name: 'Sign out' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })
})
