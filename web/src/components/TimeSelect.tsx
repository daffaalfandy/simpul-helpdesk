import type { ReactNode } from 'react'

import { END_OF_DAY, timeOptions, type ClockEdge } from '../lib/clock'
import { Dropdown } from './Dropdown'

type TimeSelectProps = {
  label: ReactNode
  /** "HH:MM", as the API writes it. */
  value: string
  onChange: (time: string) => void
  /** A start is offered 00:00 to 23:30, an end 00:30 to 24:00 (with the default step). */
  edge: ClockEdge
  /** Shown for 24:00 when that means the end of the day. */
  endOfDayLabel?: string
  step?: number
  description?: ReactNode
  isInvalid?: boolean
  errorMessage?: string
  className?: string
}

/**
 * A time of day picked from a list (the Dropdown; type "07" to jump there). The current time stays
 * in the list even when it is off the step, so an older saved value still shows.
 */
export function TimeSelect({ value, edge, endOfDayLabel, step, ...props }: TimeSelectProps) {
  const options = timeOptions(edge, value, step).map((time) => ({
    id: time,
    label: time === END_OF_DAY && endOfDayLabel ? endOfDayLabel : time,
  }))
  return <Dropdown {...props} options={options} value={value} />
}
