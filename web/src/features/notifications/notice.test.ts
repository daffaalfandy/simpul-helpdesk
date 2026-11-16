import { describe, expect, it } from 'vitest'

import type { Notification } from '../../api/client'
import i18n from '../../i18n'
import { noticeQuote, noticeText } from './notice'

const t = i18n.t.bind(i18n)
const times = { clock: (iso: string) => iso.slice(11, 16), day: () => '18 Nov' }

function notice(over: Partial<Notification>): Notification {
  return {
    id: 'n',
    event: 'status',
    kind: null,
    ticket: { key: 'HD-24871', subject: 'e-Office login fails' },
    actorName: 'Hendra Gunawan',
    statusFrom: null,
    statusTo: null,
    priorityFrom: null,
    priorityTo: null,
    reason: null,
    assigneeName: null,
    slaClock: null,
    slaDueAt: null,
    slaMinutesLeft: null,
    cause: null,
    autoApproveAt: null,
    reopenUntil: null,
    quote: null,
    also: [],
    channels: { web: true, email: true },
    createdAt: '2026-11-16T07:02:00Z',
    readAt: null,
    ...over,
  }
}

describe('UAT and Definition of Done notices', () => {
  it('dates the test request and the reopen window for the client', () => {
    const request = notice({ event: 'uat_requested', statusFrom: 'in_progress', statusTo: 'in_uat', autoApproveAt: '2026-11-18T08:00:00Z' })
    expect(noticeText(t, request, 'portal', times)).toBe(
      "Status changed: In progress → Ready for your test. If we don't hear back, it's approved on 18 Nov.",
    )
    expect(noticeText(t, { ...request, autoApproveAt: null }, 'portal', times)).toBe(
      'Status changed: In progress → Ready for your test',
    )
    const resolved = notice({ event: 'resolved', statusFrom: 'in_uat', statusTo: 'resolved', reopenUntil: '2026-11-18T08:00:00Z' })
    expect(noticeText(t, resolved, 'portal', times)).toBe(
      'Status changed: Ready for your test → Resolved. You can reopen it until 18 Nov.',
    )
  })

  it("tells the team the client's answer", () => {
    const result = notice({ event: 'uat_result', statusFrom: 'in_uat', statusTo: 'resolved' })
    expect(noticeText(t, { ...result, kind: 'approved' }, 'team', times)).toBe('Hendra Gunawan approved UAT: In UAT → Resolved')
    expect(noticeText(t, { ...result, kind: 'rejected', statusTo: 'in_progress' }, 'team', times)).toBe(
      'Hendra Gunawan rejected UAT: In UAT → In progress',
    )
    expect(noticeText(t, { ...result, kind: 'auto_approved', actorName: null }, 'team', times)).toBe(
      'UAT approved automatically, nobody answered in time: In UAT → Resolved',
    )
  })

  it('quotes the reason a supervisor is told about', () => {
    const skipped = notice({ event: 'dod', kind: 'skipped', actorName: 'Rina Pratiwi', reason: 'Client confirmed by phone' })
    expect(noticeText(t, skipped, 'team', times)).toBe('Rina Pratiwi resolved this ticket without meeting its Definition of Done')
    expect(noticeQuote(skipped)).toBe('Client confirmed by phone')
    expect(noticeText(t, { ...skipped, kind: 'override' }, 'team', times)).toBe('Rina Pratiwi set a Definition of Done for this ticket')
  })
})
