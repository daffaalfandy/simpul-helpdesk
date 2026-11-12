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
  portal: '/portal',
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
