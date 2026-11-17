/** Protocols a link in a message may use (the API's email renderer allows the same). */
export const LINK_PROTOCOLS: ReadonlyArray<string> = ['http:', 'https:', 'mailto:']

/** Added to an address typed without one ("example.com/help"). */
const DEFAULT_LINK_PROTOCOL = 'https://'

/** A scheme at the start of an address: letters first, then letters, digits, "+", "." or "-". */
const SCHEME = /^[a-z][a-z0-9+.-]*:/i

/** Whether href is an absolute http, https or mailto address. */
export function isSafeHref(href: string): boolean {
  try {
    return LINK_PROTOCOLS.includes(new URL(href).protocol)
  } catch {
    return false
  }
}

/**
 * What someone typed as a link address, made absolute ("example.com" gets https://), or null
 * when it is empty or uses another protocol (javascript:, data:, file:).
 */
export function normalizeHref(input: string): string | null {
  const raw = input.trim()
  if (raw === '') return null
  const href = SCHEME.test(raw) ? raw : DEFAULT_LINK_PROTOCOL + raw
  return isSafeHref(href) ? href : null
}
