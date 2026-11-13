import { describe, expect, it } from 'vitest'

import { formatLongDate, formatMonthYear, formatPeriod } from './dates'
import { closedDays, weekdayRuns } from './weekdays'


describe('dates', () => {
  it.each([
    ['2026-01-01', '2026-12-31', 'Jan–Dec 2026'],
    ['2026-03-01', '2027-02-28', 'Mar 2026–Feb 2027'],
    ['2026-06-01', '2026-06-30', 'Jun–Jun 2026'],
  ])('formats %s to %s', (start, end, want) => {
    expect(formatPeriod(start, end)).toBe(want)
  })

  it('formats civil dates without shifting the day', () => {
    expect(formatMonthYear('2026-01-01')).toBe('Jan 2026')
    expect(formatLongDate('2026-06-01')).toMatch(/Mon.*1 Jun 2026/)
  })
})

describe('weekdays', () => {
  it.each([
    [['mon', 'tue', 'wed', 'thu', 'fri'], [{ first: 'mon', last: 'fri' }]],
    [['fri', 'mon', 'wed', 'thu'], [{ first: 'mon', last: 'mon' }, { first: 'wed', last: 'fri' }]],
    [['sat', 'sun'], [{ first: 'sat', last: 'sun' }]],
    [[], []],
  ] as const)('groups %j into runs', (days, want) => {
    expect(weekdayRuns(days)).toEqual(want)
  })

  it('lists closed days', () => {
    expect(closedDays(['mon', 'tue', 'wed', 'thu', 'fri'])).toEqual(['sat', 'sun'])
  })
})
