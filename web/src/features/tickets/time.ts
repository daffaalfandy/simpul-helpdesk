import { useTranslation } from 'react-i18next'

import { useOrgTimeZone } from '../../api/queries'
import { formatClock, formatDayMonth, formatDayMonthYear, stampKind } from '../../lib/dates'

/**
 * Formats API timestamps in the organisation's time zone: "Just now", "08:14" today,
 * "12 Nov, 08:14" this year, "12 Nov 2026, 08:14" before.
 */
export function useTimestamp() {
  const { t } = useTranslation()
  const timeZone = useOrgTimeZone()
  const full = (iso: string) => t('time.dayTime', { date: formatDayMonthYear(iso, timeZone), time: formatClock(iso, timeZone) })
  const stamp = (iso: string, now = new Date()) => {
    switch (stampKind(iso, timeZone, now)) {
      case 'justNow':
        return t('time.justNow')
      case 'today':
        return formatClock(iso, timeZone)
      case 'thisYear':
        return t('time.dayTime', { date: formatDayMonth(iso, timeZone), time: formatClock(iso, timeZone) })
      case 'older':
        return full(iso)
    }
  }
  const day = (iso: string) => formatDayMonth(iso, timeZone)
  return { stamp, full, day, timeZone }
}
