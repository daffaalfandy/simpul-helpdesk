import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { NEW_SECRET, tokens } from '../../test/accountApi'
import { renderApp } from '../../test/renderApp'
import { ORG, state, USERS } from '../../test/server'

const MCP_URL = `${ORG.appUrl}/api/mcp`

async function tokenRow(name: string) {
  const cell = await screen.findByRole('rowheader', { name: new RegExp(`^${name}`) })
  return within(cell.closest('tr') as HTMLElement)
}

describe('MCP access', () => {
  it('opens from the account menu for staff, and lists their tokens with the server address', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    const { router } = renderApp('/board')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Account menu/ }))
    await user.click(await screen.findByRole('menuitem', { name: 'MCP access' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/account/mcp'))

    expect(await screen.findByRole('heading', { level: 1, name: 'MCP access' })).toBeInTheDocument()
    const laptop = await tokenRow('Claude Code on my laptop')
    expect(laptop.getByText('Ends in …q9Zx')).toBeInTheDocument()
    expect(laptop.getByText('Read and write')).toBeInTheDocument()
    const old = await tokenRow('Old desktop')
    expect(old.getByText('Read only')).toBeInTheDocument()
    expect(old.getByText('Never')).toBeInTheDocument()
    expect(old.getByText('Expired')).toBeInTheDocument()

    const connect = within(screen.getByRole('region', { name: 'Connect your AI client' }))
    expect(connect.getByText(MCP_URL)).toBeInTheDocument()
    expect(connect.getByText(`claude mcp add --transport http simdesk ${MCP_URL} --header "Authorization: Bearer YOUR_TOKEN"`)).toBeInTheDocument()
  })

  it('makes a token and shows its secret once, with the client settings filled in', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    renderApp('/account/mcp')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Make a token' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.click(dialog.getByRole('button', { name: 'Make token' }))
    expect(dialog.getByText('Give the token a name.')).toBeInTheDocument()
    expect(tokens.created).toEqual([])

    await user.type(dialog.getByRole('textbox', { name: 'Name' }), 'Cursor at the office')
    await user.click(dialog.getByRole('button', { name: /Access/ }))
    await user.click(await screen.findByRole('option', { name: /^Read and write/ }))
    await user.click(dialog.getByRole('button', { name: 'Make token' }))

    const created = within(await screen.findByRole('dialog', { name: /Copy your token/ }))
    expect(tokens.created).toEqual([{ name: 'Cursor at the office', access: 'write', expiresInDays: 90 }])
    expect(created.getByText(NEW_SECRET)).toBeInTheDocument()
    expect(created.getByText(new RegExp(`--header "Authorization: Bearer ${NEW_SECRET}"`))).toBeInTheDocument()
    await user.click(created.getByRole('button', { name: 'Done' }))
    expect(await tokenRow('Cursor at the office')).toBeTruthy()
    expect(screen.queryByText(NEW_SECRET)).not.toBeInTheDocument()
  })

  it('revokes a token after confirming', async () => {
    state.me = USERS['rina@demo-vendor.test'] ?? null
    renderApp('/account/mcp')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Revoke Old desktop' }))
    const dialog = within(await screen.findByRole('alertdialog'))
    expect(dialog.getByText('Revoke “Old desktop”?')).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: 'Revoke token' }))
    expect(await screen.findByText('“Old desktop” is revoked.')).toBeInTheDocument()
    expect(tokens.revoked).toEqual(['01a1cedf-0000-7000-8000-0000000000c2'])
    expect(screen.queryByRole('rowheader', { name: /^Old desktop/ })).not.toBeInTheDocument()
  })

  it('is not in a requester’s menu', async () => {
    state.me = USERS['hendra@kominfo-jambi.test'] ?? null
    renderApp('/portal')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Account menu/ }))
    expect(await screen.findByRole('menuitem', { name: 'Sign out' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'MCP access' })).not.toBeInTheDocument()
  })
})
