import { describe, expect, it } from 'vitest'

import { isMonthKey, monthChoices, monthLong, monthShort, previousMonth } from './months'

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
    { first: '2026-08', recent: ['2026-09', '2026-10', '2026-11'], earlier: ['2026-08'] },
    { first: '2025-12', recent: ['2026-09', '2026-10', '2026-11'], earlier: ['2026-08', '2026-07', '2026-06', '2026-05', '2026-04', '2026-03', '2026-02', '2026-01', '2025-12'] },
    { first: '2026-10', recent: ['2026-10', '2026-11'], earlier: [] },
    { first: '2026-11', recent: ['2026-11'], earlier: [] },
    { first: null, recent: ['2026-11'], earlier: [] },
  ])('offers buttons and older months from the first ticket ($first)', ({ first, recent, earlier }) => {
    expect(monthChoices('2026-11', first)).toEqual({ recent, earlier })
  })

  it('names months without the browser time zone', () => {
    expect(monthShort('2026-11')).toBe('Nov')
    expect(monthLong('2027-01')).toBe('January 2027')
  })
})
