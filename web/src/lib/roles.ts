import type { Role } from '../api/client'
import { paths } from './constants'

export const Roles = {
  Admin: 'admin',
  Supervisor: 'supervisor',
  Agent: 'agent',
  Requester: 'requester',
} as const satisfies Record<string, Role>

/** Mirrors the API's role table (api/internal/auth/authz.go, HANDOFF §3). */
export const can = {
  useAgentApp: (role: Role) => role !== Roles.Requester,
  usePortal: (role: Role) => role === Roles.Requester,
  manageProjects: (role: Role) => role === Roles.Admin || role === Roles.Supervisor,
  viewReports: (role: Role) => role === Roles.Admin || role === Roles.Supervisor,
  manageSettings: (role: Role) => role === Roles.Admin,
}

/** Where each role lands after signing in. */
export function homePathFor(role: Role): typeof paths.board | typeof paths.portal {
  return can.usePortal(role) ? paths.portal : paths.board
}
