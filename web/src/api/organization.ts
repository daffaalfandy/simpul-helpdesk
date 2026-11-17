/**
 * The organization this browser tab works in. Every API request names it
 * (X-Organization-Id), so two tabs can work in two organizations at once; each tab keeps its own
 * choice in sessionStorage, and a new tab starts where the user last worked (the API's default).
 */
const STORAGE_KEY = 'simdesk.organization'
const HEADER = 'X-Organization-Id'

let current: string | null = read()

function read(): string | null {
  try {
    return window.sessionStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

export function currentOrganization(): string | null {
  return current
}

export function setCurrentOrganization(id: string | null): void {
  current = id
  try {
    if (id) window.sessionStorage.setItem(STORAGE_KEY, id)
    else window.sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // Storage blocked: the choice still holds for this page's life.
  }
}

/** Adds the tab's organization to an outgoing request. */
export function withOrganization(request: Request): Request {
  if (current) request.headers.set(HEADER, current)
  return request
}
