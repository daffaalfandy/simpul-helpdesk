import createClient from 'openapi-fetch'

import type { components, paths } from './schema.gen'

export const API_BASE_PATH = '/api/v1'

export type Me = components['schemas']['Me']
export type Organization = components['schemas']['Organization']
export type Problem = components['schemas']['Problem']
export type Role = components['schemas']['Role']
export type Client = components['schemas']['Client']
export type ClientInput = components['schemas']['ClientInput']
export type SlaPlan = components['schemas']['SlaPlan']
export type SlaPlanInput = components['schemas']['SlaPlanInput']
export type SlaTarget = components['schemas']['SlaTarget']
export type SlaCalendar = components['schemas']['SlaCalendar']
export type Priority = components['schemas']['Priority']
export type BusinessHours = components['schemas']['BusinessHours']
export type BusinessHoursInput = components['schemas']['BusinessHoursInput']
export type Weekday = components['schemas']['Weekday']
export type Holiday = components['schemas']['Holiday']
export type HolidayPreset = components['schemas']['HolidayPreset']
export type User = components['schemas']['User']

/** Stable error codes returned by the API (api/internal/apperr). */
export const ErrorCode = {
  Unauthenticated: 'unauthenticated',
  InvalidCredentials: 'invalid_credentials',
  RateLimited: 'rate_limited',
  InvalidToken: 'invalid_token',
  WeakPassword: 'weak_password',
  ClientNameTaken: 'client_name_taken',
  ClientInUse: 'client_in_use',
  SlaPlanNameTaken: 'sla_plan_name_taken',
  SlaPlanInUse: 'sla_plan_in_use',
  SlaPlanIsDefault: 'sla_plan_is_default',
  HolidayDateTaken: 'holiday_date_taken',
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
  readonly detail?: string
  readonly fields: NonNullable<Problem['errors']>

  constructor(response: Response, problem?: Partial<Problem>) {
    super(problem?.title ?? response.statusText)
    this.status = response.status
    this.code = problem?.code ?? ''
    this.detail = problem?.detail
    this.fields = problem?.errors ?? []
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
