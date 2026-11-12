import { describe, expect, it } from 'vitest'

import { formatLongDate, formatMonthYear, formatPeriod } from './dates'
import { fromParts, toParts, unitsFor, type DurationParts } from './duration'
import { closedDays, weekdayRuns } from './weekdays'

const WORKING_DAY = 540

describe('toParts', () => {
  it.each([
    [15, 'business_hours', { value: 15, unit: 'minutes' }],
    [240, 'business_hours', { value: 4, unit: 'hours' }],
    [540, 'business_hours', { value: 1, unit: 'workingDays' }],
    [1080, 'business_hours', { value: 2, unit: 'workingDays' }],
    [600, 'business_hours', { value: 10, unit: 'hours' }],
    [90, 'business_hours', { value: 90, unit: 'minutes' }],
    [540, 'always', { value: 9, unit: 'hours' }],
    [1440, 'always', { value: 1, unit: 'days' }],
    [2880, 'always', { value: 2, unit: 'days' }],
  ] as const)('%i minutes on %s', (minutes, calendar, want) => {
    expect(toParts(minutes, calendar, WORKING_DAY)).toEqual(want)
  })

  it('round-trips through fromParts', () => {
    for (const minutes of [15, 30, 120, 240, 480, 540, 1080, 2700]) {
      expect(fromParts(toParts(minutes, 'business_hours', WORKING_DAY), WORKING_DAY)).toBe(minutes)
    }
  })

  it('uses the current working day length', () => {
    const twoDays: DurationParts = { value: 2, unit: 'workingDays' }
    expect(fromParts(twoDays, 510)).toBe(1020)
  })

  it('offers working days on business hours and days on 24/7', () => {
    expect(unitsFor('business_hours')).toEqual(['minutes', 'hours', 'workingDays'])
    expect(unitsFor('always')).toEqual(['minutes', 'hours', 'days'])
  })
})

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
