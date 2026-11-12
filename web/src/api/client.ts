import createClient from 'openapi-fetch'

import type { components, paths } from './schema.gen'

export const API_BASE_PATH = '/api/v1'

export type Me = components['schemas']['Me']
export type Organization = components['schemas']['Organization']
export type Problem = components['schemas']['Problem']
export type Role = components['schemas']['Role']

/** Stable error codes returned by the API (api/internal/apperr). */
export const ErrorCode = {
  Unauthenticated: 'unauthenticated',
  InvalidCredentials: 'invalid_credentials',
  RateLimited: 'rate_limited',
  InvalidToken: 'invalid_token',
  WeakPassword: 'weak_password',
} as const

export const HttpStatus = {
  Unauthorized: 401,
} as const

const RETRY_AFTER_HEADER = 'Retry-After'

/** An error response from the API, carrying its problem details. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly retryAfterSeconds?: number

  constructor(response: Response, problem?: Partial<Problem>) {
    super(problem?.title ?? response.statusText)
    this.status = response.status
    this.code = problem?.code ?? ''
    const retry = Number(response.headers.get(RETRY_AFTER_HEADER))
    if (Number.isFinite(retry) && retry > 0) this.retryAfterSeconds = retry
  }
}

// Absolute base URL: same origin in the browser, and valid for fetch in tests.
// fetch is looked up per call (not captured once) so test mocks can intercept it.
export const api = createClient<paths>({
  baseUrl: new URL(API_BASE_PATH, window.location.origin).toString(),
  fetch: (request) => globalThis.fetch(request),
})
