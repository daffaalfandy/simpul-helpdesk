import createClient from 'openapi-fetch'

import { withOrganization } from './organization'
import type { components, paths } from './schema.gen'

export const API_BASE_PATH = '/api/v1'

export type Me = components['schemas']['Me']
export type SetupRequest = components['schemas']['SetupRequest']
export type Organization = components['schemas']['Organization']
export type OrganizationSummary = components['schemas']['OrganizationSummary']
export type OrganizationChoice = components['schemas']['OrganizationChoice']
export type Install = components['schemas']['Install']
export type OrganizationInput = components['schemas']['OrganizationInput']
export type Member = components['schemas']['Member']
export type Members = components['schemas']['Members']
export type MemberInvite = components['schemas']['MemberInvite']
export type StaffRole = components['schemas']['StaffRole']
export type Problem = components['schemas']['Problem']
export type Role = components['schemas']['Role']
export type Client = components['schemas']['Client']
export type ClientInput = components['schemas']['ClientInput']
export type SlaCalendar = components['schemas']['SlaCalendar']
export type Priority = components['schemas']['Priority']
export type BusinessHours = components['schemas']['BusinessHours']
export type BusinessHoursInput = components['schemas']['BusinessHoursInput']
export type Weekday = components['schemas']['Weekday']
export type Holiday = components['schemas']['Holiday']
export type HolidayPreset = components['schemas']['HolidayPreset']
export type User = components['schemas']['User']
export type UserStatus = components['schemas']['UserStatus']
export type Ticket = components['schemas']['Ticket']
export type TicketSummary = components['schemas']['TicketSummary']
export type TicketInput = components['schemas']['TicketInput']
export type TicketType = components['schemas']['TicketType']
export type TicketStatus = components['schemas']['TicketStatus']
export type TicketChannel = components['schemas']['TicketChannel']
export type TicketSla = components['schemas']['TicketSla']
export type TicketProgress = components['schemas']['TicketProgress']
export type TicketProgressStep = components['schemas']['TicketProgressStep']
export type TicketMessage = components['schemas']['TicketMessage']
export type TicketMessageInput = components['schemas']['TicketMessageInput']
export type MessageVisibility = components['schemas']['MessageVisibility']
export type TicketActivity = components['schemas']['TicketActivity']
export type TicketAssigneeFilter = components['schemas']['TicketAssigneeFilter']
export type Attachment = components['schemas']['Attachment']
export type AttachmentPolicy = components['schemas']['AttachmentPolicy']
export type Intake = components['schemas']['Intake']
export type IntakeProject = components['schemas']['IntakeProject']
export type CatalogItem = components['schemas']['CatalogItem']
export type CatalogItemInput = components['schemas']['CatalogItemInput']
export type Requester = components['schemas']['Requester']
export type RequesterInput = components['schemas']['RequesterInput']
export type Assignee = components['schemas']['Assignee']
export type Ref = components['schemas']['Ref']
export type Board = components['schemas']['Board']
export type BoardCard = components['schemas']['BoardCard']
export type BoardColumn = components['schemas']['BoardColumn']
export type BoardColumnId = components['schemas']['BoardColumnId']
export type BoardStats = components['schemas']['BoardStats']
export type SlaReport = components['schemas']['SlaReport']
export type SlaReportProject = components['schemas']['SlaReportProject']
export type SlaFigures = components['schemas']['SlaFigures']
export type SlaWaitingPause = components['schemas']['SlaWaitingPause']
export type PriorityLevel = components['schemas']['PriorityLevel']
export type Project = components['schemas']['Project']
export type ProjectCreate = components['schemas']['ProjectCreate']
export type ProjectUpdate = components['schemas']['ProjectUpdate']
export type ProjectSla = components['schemas']['ProjectSla']
export type DodConfig = components['schemas']['DodConfig']
export type AutoApproveDays = components['schemas']['AutoApproveDays']
export type ProjectDodInput = components['schemas']['ProjectDodInput']
export type DodItem = components['schemas']['DodItem']
export type DodSource = components['schemas']['DodSource']
export type TicketDod = components['schemas']['TicketDod']
export type TicketDodItem = components['schemas']['TicketDodItem']
export type TicketDodInput = components['schemas']['TicketDodInput']
export type TicketUat = components['schemas']['TicketUat']
export type TicketReopen = components['schemas']['TicketReopen']
export type UatAnswer = components['schemas']['UatAnswer']
export type UatRequest = components['schemas']['UatRequest']
export type BoardCardUat = components['schemas']['BoardCardUat']
export type ProjectTarget = components['schemas']['ProjectTarget']
export type Duration = components['schemas']['Duration']
export type DurationUnit = components['schemas']['DurationUnit']
export type AtRiskPercent = components['schemas']['AtRiskPercent']
export type Notification = components['schemas']['Notification']
export type NotificationPage = components['schemas']['NotificationPage']
export type NotificationEvent = components['schemas']['NotificationEvent']
export type NotificationKind = components['schemas']['NotificationKind']
export type NotificationFilter = components['schemas']['NotificationFilter']
export type NotificationChange = components['schemas']['NotificationChange']
export type NotificationAlso = components['schemas']['NotificationAlso']
export type NotificationSettings = components['schemas']['NotificationSettings']
export type NotificationEventSetting = components['schemas']['NotificationEventSetting']
export type EmailFrequency = components['schemas']['EmailFrequency']
export type Audience = components['schemas']['Audience']
export type Colleague = components['schemas']['Colleague']
export type SlaClock = components['schemas']['SlaClock']
export type SlaClockName = components['schemas']['SlaClockName']
export type SlaPhase = components['schemas']['SlaPhase']
export type BoardCardSla = components['schemas']['BoardCardSla']
export type BoardSlaFilter = components['schemas']['BoardSlaFilter']
export type GitProvider = components['schemas']['GitProvider']
export type Integrations = components['schemas']['Integrations']
export type Repository = components['schemas']['Repository']
export type TokenAccess = components['schemas']['TokenAccess']
export type PersonalToken = components['schemas']['PersonalToken']
export type PersonalTokens = components['schemas']['PersonalTokens']
export type PersonalTokenInput = components['schemas']['PersonalTokenInput']
export type CreatedPersonalToken = components['schemas']['CreatedPersonalToken']
export type RepositoryRef = components['schemas']['RepositoryRef']
export type RepositoryCreate = components['schemas']['RepositoryCreate']
export type RepositoryWithSecret = components['schemas']['RepositoryWithSecret']
export type RepositoryStatus = components['schemas']['RepositoryStatus']
export type RepositoryError = components['schemas']['RepositoryError']
export type CodeEventKind = components['schemas']['CodeEventKind']
export type CodeOutcome = components['schemas']['CodeOutcome']
export type CodeEventLog = components['schemas']['CodeEventLog']
export type CommitTest = components['schemas']['CommitTest']
export type CommitTestRef = components['schemas']['CommitTestRef']
export type TicketCode = components['schemas']['TicketCode']
export type CodeCommit = components['schemas']['CodeCommit']
export type BoardCardCode = components['schemas']['BoardCardCode']
export type TicketActivityCode = components['schemas']['TicketActivityCode']
export type NotificationCode = components['schemas']['NotificationCode']

/** Stable error codes returned by the API (api/internal/apperr). */
export const ErrorCode = {
  Unauthenticated: 'unauthenticated',
  InvalidCredentials: 'invalid_credentials',
  RateLimited: 'rate_limited',
  InvalidToken: 'invalid_token',
  WeakPassword: 'weak_password',
  ClientNameTaken: 'client_name_taken',
  ClientInUse: 'client_in_use',
  HolidayDateTaken: 'holiday_date_taken',
  EmailTaken: 'email_taken',
  ProjectNameTaken: 'project_name_taken',
  ProjectIsDefault: 'project_is_default',
  ProjectInUse: 'project_in_use',
  DodUnmet: 'dod_unmet',
  InvalidBranch: 'invalid_branch',
  UatNotWaiting: 'uat_not_waiting',
  ReopenNotAllowed: 'reopen_not_allowed',
  ClientHasNoProject: 'client_has_no_project',
  AttachmentTooLarge: 'attachment_too_large',
  AttachmentTypeNotAllowed: 'attachment_type_not_allowed',
  InvalidRepositoryUrl: 'invalid_repository_url',
  RepositoryTaken: 'repository_taken',
  LastAdmin: 'last_admin',
  CannotChangeSelf: 'cannot_change_self',
  NotInvited: 'not_invited',
  SetupClosed: 'setup_closed',
  OrganizationRequired: 'organization_required',
  OtherOrganization: 'other_organization',
  MonthInFuture: 'month_in_future',
  MonthOutOfRange: 'month_out_of_range',
  InvalidMonthRange: 'invalid_month_range',
} as const

export const HttpStatus = {
  Unauthorized: 401,
  NotFound: 404,
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
api.use({ onRequest: ({ request }) => withOrganization(request) })
