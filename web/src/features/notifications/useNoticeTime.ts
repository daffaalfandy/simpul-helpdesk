import { useTranslation } from 'react-i18next'

import { useOrgTimeZone } from '../../api/queries'
import { agoKind, dayKind, formatClock, formatDayMonth } from '../../lib/dates'

/**
 * Notification times in the organisation's time zone. The bell says how long ago ("2 min ago",
 * "Yesterday", "9 Nov"); the inbox gives the time ("Today, 14:02", "Yesterday", "9 Nov").
 */
export function useNoticeTime() {
  const { t } = useTranslation()
  const timeZone = useOrgTimeZone()
  const ago = (iso: string, now = new Date()) => {
    const k = agoKind(iso, timeZone, now)
    switch (k.kind) {
      case 'justNow':
        return t('time.justNow')
      case 'minutes':
        return t('time.minutesAgo', { count: k.count })
      case 'hours':
        return t('time.hoursAgo', { count: k.count })
      case 'yesterday':
        return t('time.yesterday')
      case 'date':
        return formatDayMonth(iso, timeZone)
    }
  }
  const at = (iso: string, now = new Date()) => {
    switch (dayKind(iso, timeZone, now)) {
      case 'today':
        return t('time.today', { time: formatClock(iso, timeZone) })
      case 'yesterday':
        return t('time.yesterday')
      case 'other':
        return formatDayMonth(iso, timeZone)
    }
  }
  const clock = (iso: string) => formatClock(iso, timeZone)
  return { ago, at, clock }
}
