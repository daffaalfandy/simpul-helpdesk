import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { LOGO_URL, state, USERS } from '../../test/server'
import { MAX_LOGO_BYTES } from './OrganizationCard'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

async function organisationCard() {
  const heading = await screen.findByRole('heading', { name: 'Organisation', level: 3 })
  return within(heading.closest('section') as HTMLElement)
}

describe('Settings › General › Organisation', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('shows the organisation, with the App URL set on the server', async () => {
    renderApp('/settings/general')
    const card = await organisationCard()
    expect(card.getByRole('textbox', { name: 'Organisation name' })).toHaveValue('Demo Vendor')
    expect(card.getByRole('textbox', { name: 'Ticket ID prefix' })).toHaveValue('HD')
    const appUrl = card.getByRole('textbox', { name: 'App URL' })
    expect(appUrl).toHaveValue('https://helpdesk.demo-vendor.test')
    expect(appUrl).toHaveAttribute('readonly')
    expect(card.getByRole('combobox', { name: 'Time zone' })).toHaveValue('Asia/Jakarta (WIB, UTC+7)')
    expect(card.getByText('Your logo')).toBeInTheDocument()
  })

  it('saves the name, prefix and time zone from the unsaved bar', async () => {
    const user = userEvent.setup()
    renderApp('/settings/general')
    const card = await organisationCard()
    const name = card.getByRole('textbox', { name: 'Organisation name' })
    await user.clear(name)
    await user.type(name, 'PT Vendor Teknologi')
    const prefix = card.getByRole('textbox', { name: 'Ticket ID prefix' })
    await user.clear(prefix)
    await user.type(prefix, 'ops')
    expect(prefix).toHaveValue('OPS')
    await user.click(card.getByRole('combobox', { name: 'Time zone' }))
    await user.click(await screen.findByRole('option', { name: 'Asia/Makassar (WITA, UTC+8)' }))

    const bar = screen.getByRole('region', { name: 'Unsaved changes' })
    expect(bar).toHaveTextContent('Unsaved changes to the organisation')
    await user.click(within(bar).getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Organisation saved.')
    expect(state.orgBodies).toEqual([{ name: 'PT Vendor Teknologi', ticketPrefix: 'OPS', timezone: 'Asia/Makassar' }])
    // The header reads the saved organisation at once.
    expect(screen.getByRole('link', { name: /PT Vendor Teknologi/ })).toBeInTheDocument()
  })

  it('refuses a prefix that is not a letter then letters or digits', async () => {
    const user = userEvent.setup()
    renderApp('/settings/general')
    const card = await organisationCard()
    const prefix = card.getByRole('textbox', { name: 'Ticket ID prefix' })
    await user.clear(prefix)
    await user.type(prefix, '1X')
    await user.click(within(screen.getByRole('region', { name: /Unsaved changes/ })).getByRole('button', { name: 'Save changes' }))
    expect(await card.findByText('Use a letter, then up to 9 letters or digits.')).toBeInTheDocument()
    expect(state.orgBodies).toEqual([])
  })

  it('uploads and removes the logo, and refuses one that is too large', async () => {
    const user = userEvent.setup()
    renderApp('/settings/general')
    const card = await organisationCard()
    const input = card.getByRole('button', { name: 'Upload logo' }).parentElement?.querySelector('input[type=file]') as HTMLInputElement

    const big = new File([new Uint8Array(MAX_LOGO_BYTES + 1)], 'big.png', { type: 'image/png' })
    await user.upload(input, big)
    expect(await card.findByText('The logo is too large. Choose an image up to 512 KB.')).toBeInTheDocument()

    await user.upload(input, new File([new Uint8Array(8)], 'logo.png', { type: 'image/png' }))
    expect(await card.findByRole('img', { name: 'Demo Vendor logo' })).toHaveAttribute('src', LOGO_URL)
    expect(screen.getByRole('status')).toHaveTextContent('Logo saved.')

    await user.click(card.getByRole('button', { name: 'Remove' }))
    expect(await card.findByText('Your logo')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent("Logo removed. Your organisation's initials show instead.")
  })
})
