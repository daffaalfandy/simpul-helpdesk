import { describe, expect, it } from 'vitest'

import { clockText, parseClock, timeOptions, type ClockEdge } from './clock'

describe('parseClock', () => {
  it.each([
    ['00:00', 0],
    ['08:30', 510],
    ['23:59', 1439],
    ['24:00', 1440],
    ['24:30', null],
    ['25:00', null],
    ['12:60', null],
    ['8:00', null],
    ['08:00:00', null],
    ['', null],
    ['noon', null],
  ])('%j is %j', (text, minutes) => {
    expect(parseClock(text)).toBe(minutes)
  })
})

describe('clockText', () => {
  it.each([
    [0, '00:00'],
    [5, '00:05'],
    [510, '08:30'],
    [1439, '23:59'],
    [1440, '24:00'],
  ])('%j is %j', (minutes, text) => {
    expect(clockText(minutes)).toBe(text)
  })
})

describe('timeOptions', () => {
  type Case = {
    name: string
    edge: ClockEdge
    current?: string | null
    step?: number
    count: number
    first: string
    last: string
    contains?: string[]
    absent?: string[]
  }
  const cases: Case[] = [
    { name: 'a start runs 00:00 to 23:30', edge: 'start', count: 48, first: '00:00', last: '23:30', absent: ['24:00'] },
    { name: 'an end runs 00:30 to 24:00', edge: 'end', count: 48, first: '00:30', last: '24:00', absent: ['00:00'] },
    { name: 'a time on the step adds nothing', edge: 'start', current: '08:00', count: 48, first: '00:00', last: '23:30' },
    { name: 'the end of the day adds nothing', edge: 'end', current: '24:00', count: 48, first: '00:30', last: '24:00' },
    {
      name: 'a time off the step is kept in order',
      edge: 'start',
      current: '08:15',
      count: 49,
      first: '00:00',
      last: '23:30',
      contains: ['08:00', '08:15', '08:30'],
    },
    {
      name: 'an off-step end before the first step is kept first',
      edge: 'end',
      current: '00:10',
      count: 49,
      first: '00:10',
      last: '24:00',
    },
    { name: 'text that is not a time is ignored', edge: 'end', current: 'late', count: 48, first: '00:30', last: '24:00' },
    { name: 'no current time', edge: 'start', current: null, count: 48, first: '00:00', last: '23:30' },
    { name: 'an hourly step', edge: 'end', step: 60, count: 24, first: '01:00', last: '24:00' },
  ]

  it.each(cases)('$name', ({ edge, current, step, count, first, last, contains = [], absent = [] }) => {
    const options = timeOptions(edge, current, step)
    expect(options).toHaveLength(count)
    expect(options[0]).toBe(first)
    expect(options.at(-1)).toBe(last)
    expect(new Set(options).size).toBe(options.length)
    for (const time of contains) expect(options).toContain(time)
    for (const time of absent) expect(options).not.toContain(time)
    const index = (time: string) => options.indexOf(time)
    for (let i = 1; i < contains.length; i++) expect(index(contains[i] ?? '')).toBe(index(contains[i - 1] ?? '') + 1)
  })
})
