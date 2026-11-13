/** A ticket key such as HD-24871. Any prefix: keys keep the prefix they were created with. */
const TICKET_KEY_PATTERN = /^[A-Z][A-Z0-9]{0,9}-[1-9][0-9]*$/

/** " hd-24871" → "HD-24871"; null when it can't be a key. */
export function normalizeTicketKey(raw: string): string | null {
  const key = raw.trim().toUpperCase()
  return TICKET_KEY_PATTERN.test(key) ? key : null
}
