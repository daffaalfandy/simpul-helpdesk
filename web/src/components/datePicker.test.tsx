import { CalendarDate } from '@internationalized/date'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { I18nProvider } from 'react-aria-components'
import { describe, expect, it, vi } from 'vitest'

import { DATE_LOCALE } from '../i18n/locale'
import { DatePicker } from './DatePicker'
import { TimeSelect } from './TimeSelect'

// Open on a fixed month whatever today is.
const OPENS_ON = new CalendarDate(2026, 11, 18)

function Picker({ onChange }: { onChange: (value: CalendarDate | null) => void }) {
  const [value, setValue] = useState<CalendarDate | null>(null)
  return (
    <I18nProvider locale={DATE_LOCALE}>
      <DatePicker
        label="Contract start"
        description="Optional."
        value={value}
        placeholderValue={OPENS_ON}
        onChange={(v) => {
          setValue(v)
          onChange(v)
        }}
      />
    </I18nProvider>
  )
}

function setup() {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<Picker onChange={onChange} />)
  const field = screen.getByRole('group', { name: 'Contract start' })
  const opener = screen.getByRole('button', { name: 'Open calendar Contract start' })
  return { user, onChange, field, opener }
}

describe('DatePicker', () => {
  it('is a labelled field of day, month and year with its description', () => {
    const { field } = setup()
    expect(field).toHaveAccessibleDescription(/Optional\./)
    // en-GB: day first.
    const parts = within(field).getAllByRole('spinbutton')
    expect(parts).toHaveLength(3)
    ;['day', 'month', 'year'].forEach((part, i) => expect(parts[i]).toHaveAccessibleName(`${part}, Contract start`))
  })

  it('opens a calendar on the placeholder month, focused on its day', async () => {
    const { user, opener } = setup()
    await user.click(opener)
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('grid', { name: /November 2026/ })).toBeInTheDocument()
    // On a short screen the capped panel scrolls instead of spilling off the screen.
    expect(dialog.parentElement).toHaveClass('overflow-y-auto')
    await waitFor(() => expect(document.activeElement).toHaveAccessibleName(/18 November 2026/))
    expect(within(dialog).getByRole('button', { name: /Month/ })).toHaveTextContent('Nov')
    expect(within(dialog).getByRole('button', { name: /Year/ })).toHaveTextContent('2026')
  })

  it('picks a day with the arrow keys and Enter, then closes and returns focus', async () => {
    const { user, onChange, field, opener } = setup()
    await user.click(opener)
    await screen.findByRole('dialog')
    await waitFor(() => expect(document.activeElement).toHaveAccessibleName(/18 November 2026/))
    await user.keyboard('{ArrowRight}{ArrowDown}{Enter}')

    expect(onChange).toHaveBeenCalledTimes(1)
    const picked = onChange.mock.calls[0]?.[0]
    expect(picked).toBeInstanceOf(CalendarDate)
    expect(String(picked)).toBe('2026-11-26')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(opener).toHaveFocus())
    expect(within(field).getByRole('spinbutton', { name: /day/ })).toHaveAttribute('aria-valuenow', '26')
  })

  it('closes on Esc without a change', async () => {
    const { user, onChange, opener } = setup()
    await user.click(opener)
    await screen.findByRole('dialog')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(opener).toHaveFocus())
    expect(onChange).not.toHaveBeenCalled()
  })

  it('moves by month with the arrows and jumps with the month and year lists', async () => {
    const { user, onChange, opener } = setup()
    await user.click(opener)
    const dialog = await screen.findByRole('dialog')

    await user.click(within(dialog).getByRole('button', { name: 'Next month' }))
    expect(within(dialog).getByRole('grid', { name: /December 2026/ })).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Previous month' }))
    await user.click(within(dialog).getByRole('button', { name: 'Previous month' }))
    expect(within(dialog).getByRole('grid', { name: /October 2026/ })).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /Month/ }))
    await user.click(await screen.findByRole('option', { name: 'Mar' }))
    await user.click(within(dialog).getByRole('button', { name: /Year/ }))
    await user.click(await screen.findByRole('option', { name: '2028' }))
    const grid = within(dialog).getByRole('grid', { name: /March 2028/ })
    // The calendar stays open while the lists are used.
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    await user.click(within(grid).getByRole('button', { name: /14 March 2028/ }))
    expect(String(onChange.mock.calls.at(-1)?.[0])).toBe('2028-03-14')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('still takes a typed date, day first', async () => {
    const { user, onChange, field } = setup()
    await user.click(within(field).getByRole('spinbutton', { name: /day/ }))
    await user.keyboard('05032027')
    expect(String(onChange.mock.calls.at(-1)?.[0])).toBe('2027-03-05')
  })
})

function Times({ initial, edge, onChange }: { initial: string; edge: 'start' | 'end'; onChange: (t: string) => void }) {
  const [value, setValue] = useState(initial)
  return (
    <TimeSelect
      label="Closes"
      edge={edge}
      endOfDayLabel="24:00 (end of day)"
      value={value}
      onChange={(v) => {
        setValue(v)
        onChange(v)
      }}
    />
  )
}

describe('TimeSelect', () => {
  it('offers half hours up to the end of the day and keeps an off-step time', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Times initial="17:15" edge="end" onChange={onChange} />)
    const trigger = screen.getByRole('button', { name: /Closes/ })
    expect(trigger).toHaveTextContent('17:15')

    await user.click(trigger)
    const names = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(names[0]).toBe('00:30')
    expect(names).not.toContain('00:00')
    expect(names.slice(names.indexOf('17:00'), names.indexOf('17:00') + 3)).toEqual(['17:00', '17:15', '17:30'])
    expect(names.at(-1)).toBe('24:00 (end of day)')

    await user.click(screen.getByRole('option', { name: '24:00 (end of day)' }))
    expect(onChange).toHaveBeenCalledWith('24:00')
    expect(trigger).toHaveTextContent('24:00 (end of day)')
  })

  it('offers a start from midnight to half past eleven', async () => {
    const user = userEvent.setup()
    render(<Times initial="08:00" edge="start" onChange={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: /Closes/ }))
    const names = within(await screen.findByRole('listbox')).getAllByRole('option').map((o) => o.textContent)
    expect(names).toHaveLength(48)
    expect([names[0], names.at(-1)]).toEqual(['00:00', '23:30'])
  })
})
