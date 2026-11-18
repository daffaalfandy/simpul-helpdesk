import type { TFunction } from 'i18next'

import { DATE_LOCALE } from '../../i18n/locale'
import { compactSpan } from '../tickets/slaFormat'
import { monthLong, type MonthKey } from './months'

const percent = new Intl.NumberFormat(DATE_LOCALE, { maximumFractionDigits: 1 })

/** "95.3", "100": one decimal at most, as in the Excel file and the PDF. */
export function formatPercent(value: number): string {
  return percent.format(value)
}

/**
 * Report times run on business hours, where a day has no single length, so they are written in
 * hours and minutes only: "16m", "5h", "26h 5m".
 */
export function reportSpan(t: TFunction, ms: number): string {
  return compactSpan(t, ms, 0)
}

/** "November 2026", or "November 2026 (to date)" while the month runs. */
export function reportMonthLabel(t: TFunction, month: MonthKey, toDate: boolean): string {
  return reportRangeLabel(t, month, month, toDate)
}

/**
 * The report's months as the Excel file and the PDF name them: "October 2026",
 * "September 2026 to November 2026 (to date)".
 */
export function reportRangeLabel(t: TFunction, from: MonthKey, to: MonthKey, toDate: boolean): string {
  const period = from === to ? monthLong(from) : t('reports.range.span', { from: monthLong(from), to: monthLong(to) })
  return toDate ? t('reports.range.toDate', { period }) : period
}
