import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { http } from 'msw'

import { renderApp } from '../../test/renderApp'
import { PASSWORD, problem, server, state, USERS } from '../../test/server'

async function signIn(email: string, password = PASSWORD) {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('Work email'), email)
  await user.type(screen.getByLabelText('Password'), password)
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
  return user
}

describe('sign in', () => {
  it('shows the organisation name', async () => {
    renderApp('/login')
    expect(await screen.findByRole('heading', { name: 'Sign in to Demo Vendor' })).toBeInTheDocument()
  })

  it('sends agents to the board', async () => {
    const { router } = renderApp('/login')
    await signIn('rina@demo-vendor.test')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
    expect(await screen.findByRole('heading', { level: 1, name: /, Rina$/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Account menu for Rina Pratiwi/ })).toHaveTextContent('Agent · L2')
  })

  it('sends requesters to the portal with their client', async () => {
    const { router } = renderApp('/login')
    await signIn('hendra@kominfo-jambi.test')
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal'))
    expect(await screen.findByRole('heading', { level: 1, name: 'Hello, Hendra' })).toBeInTheDocument()
    expect(screen.getByText('Dinas Kominfo Jambi')).toBeInTheDocument()
  })

  it('shows a clear message for wrong credentials', async () => {
    renderApp('/login')
    await signIn('rina@demo-vendor.test', 'wrong-password')
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong email or password.')
  })

  it('tells the user how long to wait when rate limited', async () => {
    server.use(http.post('*/api/v1/auth/login', () => problem(429, 'rate_limited', { 'Retry-After': '120' })))
    renderApp('/login')
    await signIn('rina@demo-vendor.test')
    expect(await screen.findByRole('alert')).toHaveTextContent('Try again in 2 minutes.')
  })

  it('returns to the page the user asked for', async () => {
    const { router } = renderApp('/login?redirect=%2Fsettings')
    await signIn('admin@demo-vendor.test')
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  })

  it('ignores redirects to other sites', async () => {
    const { router } = renderApp('/login?redirect=%2F%2Fevil.example')
    await signIn('rina@demo-vendor.test')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
  })
})

describe('route guards', () => {
  it('sends anonymous users to sign in, remembering the page', async () => {
    const { router } = renderApp('/board')
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toEqual({ redirect: '/board' })
  })

  it('keeps requesters out of the agent app', async () => {
    state.me = USERS['hendra@kominfo-jambi.test'] ?? null
    const { router } = renderApp('/board')
    await waitFor(() => expect(router.state.location.pathname).toBe('/portal'))
  })

  it('hides admin-only pages and nav from agents', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/settings')
    await waitFor(() => expect(router.state.location.pathname).toBe('/board'))
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    expect(nav).toHaveTextContent('Board')
    expect(nav).not.toHaveTextContent('Settings')
  })

  it('shows every section to admins', async () => {
    state.me = USERS['admin@demo-vendor.test'] ?? null
    renderApp('/board')
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    for (const item of ['Board', 'Projects', 'Reports', 'Settings']) expect(nav).toHaveTextContent(item)
  })

  it('signs out from the account menu', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/board')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Account menu/ }))
    await user.click(await screen.findByRole('menuitem', { name: 'Sign out' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(state.me).toBeNull()
  })
})

describe('password reset', () => {
  it('confirms the same way whether or not the account exists', async () => {
    renderApp('/forgot-password')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Work email'), 'ghost@example.test')
    await user.click(screen.getByRole('button', { name: 'Send reset link' }))
    expect(await screen.findByRole('status')).toHaveTextContent('If an account exists for ghost@example.test')
    expect(state.forgotRequests).toEqual(['ghost@example.test'])
  })

  it('reads the token from the link fragment and removes it from the address bar', async () => {
    window.history.replaceState(null, '', '/reset-password#token=valid-token-0123456789abcdef')
    renderApp('/reset-password')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('New password'), 'brand-new-password')
    await user.type(screen.getByLabelText('Repeat new password'), 'brand-new-password')
    expect(window.location.hash).toBe('')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))
    expect(await screen.findByRole('status')).toHaveTextContent('signed out everywhere')
    expect(state.resetBodies).toEqual([{ token: 'valid-token-0123456789abcdef', password: 'brand-new-password' }])
  })

  it('checks the new password before sending it', async () => {
    window.history.replaceState(null, '', '/reset-password#token=valid-token-0123456789abcdef')
    renderApp('/reset-password')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('New password'), 'short')
    await user.type(screen.getByLabelText('Repeat new password'), 'different')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))
    expect(await screen.findByText('Use at least 10 characters.')).toBeInTheDocument()
    expect(screen.getByText("The passwords don't match.")).toBeInTheDocument()
    expect(state.resetBodies).toEqual([])
  })

  it('explains an expired link and offers a new one', async () => {
    window.history.replaceState(null, '', '/reset-password#token=expired-token-0123456789abc')
    renderApp('/reset-password')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('New password'), 'brand-new-password')
    await user.type(screen.getByLabelText('Repeat new password'), 'brand-new-password')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid or has expired')
    expect(screen.getByRole('link', { name: 'Request a new link' })).toHaveAttribute('href', '/forgot-password')
  })
})
