import { AFTERNOON_STARTS_AT_HOUR, EVENING_STARTS_AT_HOUR, MAX_INITIALS } from './constants'

/** Joins class names, skipping falsy values. */
export function cx(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(' ')
}

/** "Rina Pratiwi" → "RP"; "Dinas Kominfo Jambi" → "DK". */
export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, MAX_INITIALS)
    .map((word) => word[0]?.toUpperCase() ?? '')
    .join('')
}

/** First word of a full name. */
export function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? name
}

export type DayPart = 'morning' | 'afternoon' | 'evening'

export function dayPart(date: Date): DayPart {
  const hour = date.getHours()
  if (hour < AFTERNOON_STARTS_AT_HOUR) return 'morning'
  if (hour < EVENING_STARTS_AT_HOUR) return 'afternoon'
  return 'evening'
}

/** Only allow same-site paths as post-login redirects (no open redirects). */
export function safeRedirect(target: unknown): string | undefined {
  if (typeof target !== 'string' || !target.startsWith('/') || target.startsWith('//')) return undefined
  return target
}
