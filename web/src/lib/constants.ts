/** Product brand. Not translated. */
export const BRAND_NAME = 'SimDesk'
export const BRAND_MONOGRAM = 'SD'

/** App routes, shared by navigation and redirects. */
export const paths = {
  home: '/',
  login: '/login',
  forgotPassword: '/forgot-password',
  resetPassword: '/reset-password',
  board: '/board',
  projects: '/projects',
  reports: '/reports',
  settings: '/settings',
  settingsGeneral: '/settings/general',
  settingsMembers: '/settings/members',
  settingsPriorities: '/settings/priorities',
  settingsClients: '/settings/clients',
  settingsIntegrations: '/settings/integrations',
  settingsCatalog: '/settings/catalog',
  ticket: '/tickets/$key',
  portal: '/portal',
  portalNew: '/portal/new',
  portalTicket: '/portal/tickets/$key',
} as const

/** Must match the API's password policy (api/internal/auth). */
export const MIN_PASSWORD_LENGTH = 10
export const MAX_PASSWORD_LENGTH = 128

/** Prefix of the reset token in the reset link's URL fragment. */
export const RESET_TOKEN_HASH_KEY = 'token'

export const SECONDS_PER_MINUTE = 60

/** Greeting changes at these local hours. */
export const AFTERNOON_STARTS_AT_HOUR = 12
export const EVENING_STARTS_AT_HOUR = 18

/** Initials shown in avatars and logo placeholders. */
export const MAX_INITIALS = 2

/** How long the signed-in user is considered fresh before refetching. */
export const ME_STALE_TIME_MS = 60_000

export const MINUTES_PER_HOUR = 60
export const HOURS_PER_DAY = 24
export const MINUTES_PER_DAY = MINUTES_PER_HOUR * HOURS_PER_DAY

/** Holiday years offered around the current year, besides years with a bundled preset. */
export const HOLIDAY_YEARS_AROUND = 1

/** A timestamp this recent reads "Just now". */
export const JUST_NOW_MS = 60_000

/** Tickets fetched per page in lists. */
export const TICKET_PAGE_SIZE = 50
