import { describe, expect, it } from 'vitest'

import {
  currentMonthRange,
  lastMonthRange,
  rangeFromSearch,
  searchFromRange,
  validateDateRangeSearch,
  withRange,
  type DateRangeSearch,
} from './dateRange'
import { formatDateRange } from './dates'

const JAKARTA = 'Asia/Jakarta'
// 00:30 on 1 Nov in Jakarta, still 31 Oct in UTC.
const EARLY_NOVEMBER = new Date('2026-10-31T17:30:00Z')
const MID_NOVEMBER = new Date('2026-11-18T03:00:00Z')
const NOVEMBER = { from: '2026-11-01', to: '2026-11-30' }
const OCTOBER = { from: '2026-10-01', to: '2026-10-31' }

describe('currentMonthRange and lastMonthRange', () => {
  it.each([
    ['in the time zone', EARLY_NOVEMBER, JAKARTA, NOVEMBER, OCTOBER],
    ['in UTC', EARLY_NOVEMBER, 'UTC', OCTOBER, { from: '2026-09-01', to: '2026-09-30' }],
    ['across a year', new Date('2027-01-10T03:00:00Z'), JAKARTA, { from: '2027-01-01', to: '2027-01-31' }, { from: '2026-12-01', to: '2026-12-31' }],
    ['a leap February', new Date('2028-03-05T03:00:00Z'), JAKARTA, { from: '2028-03-01', to: '2028-03-31' }, { from: '2028-02-01', to: '2028-02-29' }],
  ])('%s', (_, now, zone, current, last) => {
    expect(currentMonthRange(zone, now)).toEqual(current)
    expect(lastMonthRange(zone, now)).toEqual(last)
  })
})

describe('validateDateRangeSearch', () => {
  it.each([
    ['both days', { from: '2026-10-01', to: '2026-10-31' }, OCTOBER],
    ['one day', { from: '2026-10-05', to: '2026-10-05' }, { from: '2026-10-05', to: '2026-10-05' }],
    ['nothing', {}, {}],
    ['from only', { from: '2026-10-01' }, {}],
    ['to only', { to: '2026-10-31' }, {}],
    ['reversed', { from: '2026-10-31', to: '2026-10-01' }, {}],
    ['not a day', { from: '2026-02-30', to: '2026-03-01' }, {}],
    ['another format', { from: '01/10/2026', to: '2026-10-31' }, {}],
    ['not text', { from: 20261001, to: '2026-10-31' }, {}],
  ])('%s', (_, search, want) => {
    expect(validateDateRangeSearch({ ...search, view: 'mine' })).toEqual(want)
  })
})

describe('the range in the URL', () => {
  it('reads this month when the URL has none', () => {
    expect(rangeFromSearch({}, JAKARTA, MID_NOVEMBER)).toEqual(NOVEMBER)
    expect(rangeFromSearch(OCTOBER, JAKARTA, MID_NOVEMBER)).toEqual(OCTOBER)
  })

  it('keeps this month out of the URL', () => {
    expect(searchFromRange(NOVEMBER, JAKARTA, MID_NOVEMBER)).toEqual({})
    expect(searchFromRange(OCTOBER, JAKARTA, MID_NOVEMBER)).toEqual(OCTOBER)
  })

  it('replaces the range and keeps the other filters', () => {
    const search: DateRangeSearch & { view?: string; q?: string } = { view: 'mine', q: 'vpn' }
    expect(withRange(search, OCTOBER, JAKARTA, MID_NOVEMBER)).toEqual({ view: 'mine', q: 'vpn', ...OCTOBER })
    expect(withRange({ ...search, ...OCTOBER }, NOVEMBER, JAKARTA, MID_NOVEMBER)).toEqual({ view: 'mine', q: 'vpn' })
  })
})

describe('formatDateRange', () => {
  // Intl puts thin spaces around the dash between two months.
  const spaces = (s: string) => s.replace(/\u2009/g, ' ')
  it.each([
    ['one month', '2026-11-01', '2026-11-30', '1–30 Nov 2026'],
    ['one day', '2026-11-18', '2026-11-18', '18 Nov 2026'],
    ['two months', '2026-10-01', '2026-11-30', '1 Oct – 30 Nov 2026'],
    ['two years', '2025-12-01', '2026-01-31', '1 Dec 2025 – 31 Jan 2026'],
  ])('%s', (_, from, to, want) => {
    expect(spaces(formatDateRange(from, to))).toBe(want)
  })
})
