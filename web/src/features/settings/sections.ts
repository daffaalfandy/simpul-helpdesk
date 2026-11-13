import { Building2, Flag, GitBranch, ListChecks, Settings2, Users, type LucideIcon } from 'lucide-react'

import { paths } from '../../lib/constants'

export type SectionId = 'general' | 'members' | 'priorities' | 'clients' | 'catalog' | 'integrations'
export type GroupId = 'organisation' | 'serviceDesk' | 'integrations'

export type Section = { id: SectionId; path: string; icon: LucideIcon }
export type Group = { id: GroupId; sections: Section[] }

/** Settings sidebar per HANDOFF §4 (the mock's Workspace and Plan & billing items are dropped). */
export const GROUPS: Group[] = [
  {
    id: 'organisation',
    sections: [
      { id: 'general', path: paths.settingsGeneral, icon: Settings2 },
      { id: 'members', path: paths.settingsMembers, icon: Users },
    ],
  },
  {
    id: 'serviceDesk',
    sections: [
      { id: 'priorities', path: paths.settingsPriorities, icon: Flag },
      { id: 'clients', path: paths.settingsClients, icon: Building2 },
      { id: 'catalog', path: paths.settingsCatalog, icon: ListChecks },
    ],
  },
  { id: 'integrations', sections: [{ id: 'integrations', path: paths.settingsIntegrations, icon: GitBranch }] },
]

export function groupOfPath(pathname: string): GroupId | undefined {
  return GROUPS.find((g) => g.sections.some((s) => pathname.startsWith(s.path)))?.id
}
