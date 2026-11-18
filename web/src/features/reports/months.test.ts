import { describe, expect, it } from 'vitest'

import {
  isMonthKey,
  monthLong,
  monthOptions,
  previousMonth,
  rangeOptions,
  searchFromRange,
  validateReportSearch,
} from './months'

describe('report months', () => {
  it('reads only months the API writes', () => {
    expect(isMonthKey('2026-11')).toBe(true)
    for (const bad of ['2026-13', '2026-00', '2026-1', '26-11', '2026-11-01', 202611, undefined]) {
      expect(isMonthKey(bad)).toBe(false)
    }
  })

  it('steps back across a year', () => {
    expect(previousMonth('2026-11')).toBe('2026-10')
    expect(previousMonth('2027-01')).toBe('2026-12')
  })

  it.each([
    {
      name: 'tickets since August',
      current: '2026-11',
      oldest: '2025-11',
      first: '2026-08',
      want: ['2026-11', '2026-10', '2026-09', '2026-08'],
    },
    {
      name: 'stops at the oldest month the API serves',
      current: '2026-11',
      oldest: '2025-11',
      first: '2024-03',
      want: ['2026-11', '2026-10', '2026-09', '2026-08', '2026-07', '2026-06', '2026-05', '2026-04', '2026-03', '2026-02', '2026-01', '2025-12', '2025-11'],
    },
    {
      name: 'across a year boundary',
      current: '2027-01',
      oldest: '2026-01',
      first: '2026-11',
      want: ['2027-01', '2026-12', '2026-11'],
    },
    { name: 'first ticket this month', current: '2026-11', oldest: '2025-11', first: '2026-11', want: ['2026-11'] },
    { name: 'no tickets yet', current: '2026-11', oldest: '2025-11', first: null, want: ['2026-11'] },
  ])('offers the current month back to the first ticket or the oldest month ($name)', ({ current, oldest, first, want }) => {
    expect(monthOptions(current, oldest, first)).toEqual(want)
  })

  it('names months without the browser time zone', () => {
    expect(monthLong('2027-01')).toBe('January 2027')
  })

  it.each([
    {
      name: 'the current month alone',
      range: { from: '2026-11', to: '2026-11' },
      from: ['2026-11', '2026-10', '2026-09', '2026-08'],
      to: ['2026-11'],
    },
    {
      name: 'an earlier month alone',
      range: { from: '2026-09', to: '2026-09' },
      from: ['2026-09', '2026-08'],
      to: ['2026-11', '2026-10', '2026-09'],
    },
    {
      name: 'several months',
      range: { from: '2026-09', to: '2026-10' },
      from: ['2026-10', '2026-09', '2026-08'],
      to: ['2026-11', '2026-10', '2026-09'],
    },
    {
      name: 'a link starting before the first ticket',
      range: { from: '2026-06', to: '2026-11' },
      from: ['2026-11', '2026-10', '2026-09', '2026-08', '2026-07', '2026-06'],
      to: ['2026-11', '2026-10', '2026-09', '2026-08', '2026-07', '2026-06'],
    },
  ])('keeps the start no later than the end ($name)', ({ range, from, to }) => {
    expect(rangeOptions('2026-11', '2025-11', '2026-08', range)).toEqual({ from, to })
  })

  it('offers only the current month without tickets', () => {
    expect(rangeOptions('2026-11', '2025-11', null, { from: '2026-11', to: '2026-11' })).toEqual({ from: ['2026-11'], to: ['2026-11'] })
  })

  it('reads the months from the URL and leaves the current month alone out of it', () => {
    expect(validateReportSearch({ from: '2026-09', to: '2026-11' })).toEqual({ from: '2026-09', to: '2026-11' })
    expect(validateReportSearch({ from: '2026-09' })).toEqual({ from: '2026-09' })
    expect(validateReportSearch({ from: '2026-13', to: 202611, month: '2026-10' })).toEqual({})

    expect(searchFromRange({ from: '2026-11', to: '2026-11' }, '2026-11')).toEqual({})
    expect(searchFromRange({ from: '2026-10', to: '2026-10' }, '2026-11')).toEqual({ from: '2026-10', to: '2026-10' })
    expect(searchFromRange({ from: '2026-09', to: '2026-11' }, '2026-11')).toEqual({ from: '2026-09', to: '2026-11' })
  })
})
