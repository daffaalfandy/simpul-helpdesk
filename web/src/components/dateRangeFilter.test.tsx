import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { I18nProvider } from 'react-aria-components'
import { describe, expect, it, vi } from 'vitest'

import { DATE_LOCALE } from '../i18n/locale'
import type { DateRange } from '../lib/dateRange'
import { DateRangeFilter } from './DateRangeFilter'

// The test clock is 18 Nov 2026 (src/test/setup.ts).
const NOVEMBER = { from: '2026-11-01', to: '2026-11-30' }

function Filter({ onChange }: { onChange: (range: DateRange) => void }) {
  const [value, setValue] = useState<DateRange>(NOVEMBER)
  return (
    <I18nProvider locale={DATE_LOCALE}>
      <DateRangeFilter
        value={value}
        timeZone="Asia/Jakarta"
        description="Shows the tickets reported on these days."
        onChange={(range) => {
          setValue(range)
          onChange(range)
        }}
      />
    </I18nProvider>
  )
}

function setup() {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<Filter onChange={onChange} />)
  return { user, onChange, pill: screen.getByRole('button', { name: 'Reported 1–30 Nov 2026' }) }
}

/** A day of the calendar's month, by its accessible name ("Tuesday, 3 November 2026"). */
function day(dialog: HTMLElement, n: number) {
  return within(dialog).getByRole('button', { name: new RegExp(`\\b${n} November 2026`) })
}

describe('DateRangeFilter', () => {
  it('saves the second day picked, closes and returns focus', async () => {
    const { user, onChange, pill } = setup()
    await user.click(pill)
    const dialog = await screen.findByRole('dialog', { name: 'Reported between' })
    expect(dialog).toHaveAccessibleDescription('Shows the tickets reported on these days.')
    expect(within(dialog).getByRole('grid', { name: /November 2026/ })).toBeInTheDocument()
    expect(dialog.parentElement).toHaveClass('overflow-y-auto')

    await user.click(day(dialog, 3))
    expect(onChange).not.toHaveBeenCalled()
    await user.click(day(dialog, 12))
    expect(onChange).toHaveBeenCalledExactlyOnceWith({ from: '2026-11-03', to: '2026-11-12' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    const changed = screen.getByRole('button', { name: 'Reported 3–12 Nov 2026' })
    await waitFor(() => expect(changed).toHaveFocus())
  })

  it('picks a preset at once', async () => {
    const { user, onChange, pill } = setup()
    await user.click(pill)
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Last month' }))
    expect(onChange).toHaveBeenCalledExactlyOnceWith({ from: '2026-10-01', to: '2026-10-31' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /^Reported 1.31 Oct 2026$/ }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'This month' }))
    expect(onChange).toHaveBeenLastCalledWith(NOVEMBER)
  })

  it('closes on Esc without a change', async () => {
    const { user, onChange, pill } = setup()
    await user.click(pill)
    const dialog = await screen.findByRole('dialog')
    await user.click(day(dialog, 3))
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(onChange).not.toHaveBeenCalled()
    await waitFor(() => expect(pill).toHaveFocus())
    expect(pill).toHaveAccessibleName('Reported 1–30 Nov 2026')
  })
})
