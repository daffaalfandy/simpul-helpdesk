import { describe, expect, it } from 'vitest'

import type { SlaTarget } from '../api/client'
import { DONE_STATUSES, OPEN_STATUSES, statusesForPortalFilter, TICKET_STATUSES } from '../features/tickets/status'
import { targetFor } from '../features/tickets/sla'
import { formatClock, formatDayMonth, stampKind, zoneAbbreviation } from './dates'
import { checkFile, fileExtension, formatFileSize } from './files'
import { normalizeTicketKey } from './ticketKey'

const JAKARTA = 'Asia/Jakarta'

describe('dates in the organisation time zone', () => {
  it('formats in WIB, not UTC', () => {
    // 17:30 UTC on 12 Nov is 00:30 WIB on 13 Nov.
    expect(formatClock('2026-11-12T17:30:00Z', JAKARTA)).toBe('00:30')
    expect(formatDayMonth('2026-11-12T17:30:00Z', JAKARTA)).toBe('13 Nov')
  })

  it('picks the stamp by age and calendar day in the zone', () => {
    const now = new Date('2026-11-13T02:00:00Z') // 09:00 WIB
    expect(stampKind('2026-11-13T01:59:30Z', JAKARTA, now)).toBe('justNow')
    expect(stampKind('2026-11-12T17:30:00Z', JAKARTA, now)).toBe('today')
    expect(stampKind('2026-11-12T16:30:00Z', JAKARTA, now)).toBe('thisYear')
    expect(stampKind('2025-12-31T01:00:00Z', JAKARTA, now)).toBe('older')
  })

  it('names Indonesian zones', () => {
    expect(zoneAbbreviation(JAKARTA)).toBe('WIB')
    expect(zoneAbbreviation('Asia/Makassar')).toBe('WITA')
    expect(zoneAbbreviation('Asia/Jayapura')).toBe('WIT')
  })
})

describe('ticket keys', () => {
  it('normalises and rejects', () => {
    expect(normalizeTicketKey(' hd-24871 ')).toBe('HD-24871')
    expect(normalizeTicketKey('HD-0')).toBeNull()
    expect(normalizeTicketKey('../etc')).toBeNull()
  })
})

describe('files', () => {
  const policy = { maxBytes: 10, maxPerMessage: 2, extensions: ['.png', '.pdf'] }
  const file = (name: string, size: number) => new File([new Uint8Array(size)], name)

  it('checks size and extension', () => {
    expect(checkFile(file('a.PNG', 10), policy)).toBeNull()
    expect(checkFile(file('a.png', 11), policy)).toBe('tooLarge')
    expect(checkFile(file('a.svg', 1), policy)).toBe('typeNotAllowed')
    expect(checkFile(file('png', 1), policy)).toBe('typeNotAllowed')
    expect(fileExtension('scan.final.PDF')).toBe('.pdf')
  })

  it('formats sizes', () => {
    expect(formatFileSize(84_000)).toBe('82 kB')
    expect(formatFileSize(1_258_291)).toBe('1.2 MB')
  })
})

describe('ticket statuses', () => {
  it('splits every status into open or done', () => {
    for (const s of TICKET_STATUSES) expect(OPEN_STATUSES.includes(s)).toBe(!DONE_STATUSES.includes(s))
    expect(statusesForPortalFilter('all')).toEqual([])
    expect(statusesForPortalFilter('resolved')).toEqual(['resolved', 'closed'])
  })
})

describe('SLA preview', () => {
  const p3: SlaTarget = {
    priority: 'P3',
    firstResponseMinutes: 120,
    resolveMinutes: 1080,
    deliverMinutes: 1500,
    deliverFromCatalog: true,
    calendar: 'business_hours',
  }

  it.each([
    ['incident uses resolve', 'incident' as const, 540, 1080],
    ['request uses the catalogue item', 'request' as const, 540, 540],
    ['request without an item uses deliver', 'request' as const, undefined, 1500],
  ])('%s', (_, type, catalog, want) => {
    expect(targetFor(p3, type, catalog).resolutionMinutes).toBe(want)
  })

  it('ignores the catalogue when the target says so', () => {
    expect(targetFor({ ...p3, deliverFromCatalog: false }, 'request', 540).resolutionMinutes).toBe(1500)
  })
})
