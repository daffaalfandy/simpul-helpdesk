import type { Me, Role } from '../api/client'
import { paths } from './constants'

export const Roles = {
  Admin: 'admin',
  Supervisor: 'supervisor',
  Agent: 'agent',
  Requester: 'requester',
} as const satisfies Record<string, Role>

/**
 * A role in the organization the user works in; null for staff who belong to no organization
 * yet (they can only wait to be added to one).
 */
export type OrgRole = Role | null

/** Mirrors the API's role table (api/internal/auth/authz.go, HANDOFF §3). */
export const can = {
  useAgentApp: (role: OrgRole) => role !== Roles.Requester,
  usePortal: (role: OrgRole) => role === Roles.Requester,
  /** Agents read the projects of their clients; supervisors and admins also change them. */
  viewProjects: (role: OrgRole) => role !== null && role !== Roles.Requester,
  manageProjects: (role: OrgRole) => role === Roles.Admin || role === Roles.Supervisor,
  /** Agents only take an unassigned ticket or release their own; supervisors and admins assign anyone. */
  assignAnyone: (role: OrgRole) => role === Roles.Admin || role === Roles.Supervisor,
  viewReports: (role: OrgRole) => role === Roles.Admin || role === Roles.Supervisor,
  manageSettings: (role: OrgRole) => role === Roles.Admin,
}

/** People who work in more than one organization pick one after signing in, and can switch. */
export function picksOrganization(me: Me): boolean {
  return me.organizations.length > 1
}

/** Where each role lands after signing in. */
export function homePathFor(role: OrgRole): typeof paths.board | typeof paths.portal {
  return can.usePortal(role) ? paths.portal : paths.board
}
