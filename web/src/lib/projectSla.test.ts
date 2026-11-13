import i18n from 'i18next'
import { describe, expect, it } from 'vitest'

import type { ProjectTarget } from '../api/client'
import { formatTarget, formatTargetShort } from './formatDuration'
import { durationMinutes, orderError, validValue } from './projectSla'

const WORKING_DAY = 540
const t = i18n.t.bind(i18n)

const target = (first: ProjectTarget['firstResponse'], resolve: ProjectTarget['resolve'], deliver = resolve): ProjectTarget => ({
  priority: 'P2',
  firstResponse: first,
  resolve,
  deliver,
  calendar: 'business_hours',
})

describe('project SLA rules', () => {
  it.each([
    [{ value: 15, unit: 'min' as const }, 15],
    [{ value: 4, unit: 'h' as const }, 240],
    [{ value: 2, unit: 'wd' as const }, 1080],
    [{ value: 2, unit: 'd' as const }, 2880],
  ])('%o is %i minutes', (d, minutes) => {
    expect(durationMinutes(d, WORKING_DAY)).toBe(minutes)
  })

  it('needs first response strictly shorter than resolve, then deliver', () => {
    expect(orderError(target({ value: 30, unit: 'min' }, { value: 8, unit: 'h' }), WORKING_DAY)).toBeNull()
    expect(orderError(target({ value: 8, unit: 'h' }, { value: 8, unit: 'h' }), WORKING_DAY)).toBe('resolve')
    expect(orderError(target({ value: 9, unit: 'h' }, { value: 10, unit: 'h' }, { value: 1, unit: 'wd' }), WORKING_DAY)).toBe('deliver')
    expect(orderError(target({ value: 1, unit: 'wd' }, { value: 1, unit: 'd' }), WORKING_DAY)).toBeNull()
  })

  it('accepts whole numbers from 1', () => {
    expect(validValue({ value: 1, unit: 'h' })).toBe(true)
    expect(validValue({ value: 0, unit: 'h' })).toBe(false)
    expect(validValue({ value: 1.5, unit: 'h' })).toBe(false)
  })

  it('writes targets out in words and for chips', () => {
    expect(formatTarget(t, { value: 1, unit: 'wd' })).toBe('1 working day')
    expect(formatTarget(t, { value: 2, unit: 'h' })).toBe('2 hours')
    expect(formatTarget(t, { value: 15, unit: 'min' })).toBe('15 min')
    expect(formatTargetShort(t, { value: 2, unit: 'h' })).toBe('2 h')
    expect(formatTargetShort(t, { value: 3, unit: 'wd' })).toBe('3 wd')
  })
})
