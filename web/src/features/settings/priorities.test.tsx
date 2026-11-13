import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { projectsApi } from '../../test/projectsApi'
import { renderApp } from '../../test/renderApp'
import { state, USERS } from '../../test/server'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

function puts() {
  return projectsApi.requests.filter((r) => r.method === 'PUT' && r.path.endsWith('/priorities'))
}

async function open() {
  const user = userEvent.setup()
  renderApp('/settings/priorities')
  await screen.findByRole('heading', { name: 'Priorities', level: 2 })
  return user
}

describe('settings › priorities', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('shows the names, the guidance matrix and the way to project SLAs', async () => {
    await open()
    expect(screen.getByLabelText('Name for P1')).toHaveValue('Critical')
    expect(screen.getByLabelText('Description for requesters for P4')).toHaveValue('Minor issue or question, no rush')
    expect(screen.getByRole('link', { name: 'Set SLA in Projects' })).toHaveAttribute('href', '/projects')
    const matrix = screen.getByRole('table', { name: 'Priority by impact and urgency' })
    const team = within(matrix).getByRole('row', { name: /Team/ })
    expect(within(team).getAllByRole('cell').map((c) => c.textContent)).toEqual(['P3', 'P2', 'P1'])
    expect(screen.queryByRole('region', { name: 'Unsaved changes' })).not.toBeInTheDocument()
  })

  it('discards edits from the unsaved-changes bar', async () => {
    const user = await open()
    await user.type(screen.getByLabelText('Name for P2'), '!')
    const bar = screen.getByRole('region', { name: 'Unsaved changes' })
    expect(bar).toHaveTextContent('You have unsaved changes to priority names.')
    await user.click(within(bar).getByRole('button', { name: 'Discard' }))
    expect(screen.getByLabelText('Name for P2')).toHaveValue('High')
    expect(screen.queryByRole('region', { name: 'Unsaved changes' })).not.toBeInTheDocument()
    expect(puts()).toHaveLength(0)
  })

  it('saves renamed priorities and blocks blank ones', async () => {
    const user = await open()
    const name = screen.getByLabelText('Name for P1')
    await user.clear(name)
    await user.click(within(screen.getByRole('region', { name: 'Unsaved changes' })).getByRole('button', { name: 'Save changes' }))
    expect(screen.getAllByText('Enter a name and a description.')).not.toHaveLength(0)
    expect(puts()).toHaveLength(0)

    await user.type(name, 'Urgent')
    await user.click(within(screen.getByRole('region', { name: 'Unsaved changes' })).getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Priority names saved.')
    expect(puts()).toHaveLength(1)
    expect((puts()[0]?.body as { name: string }[])[0]?.name).toBe('Urgent')
    expect(screen.queryByRole('region', { name: 'Unsaved changes' })).not.toBeInTheDocument()
  })
})
