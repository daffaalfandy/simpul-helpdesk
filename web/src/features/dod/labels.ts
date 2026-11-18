import type { TFunction } from 'i18next'

import type { AutoApproveDays, DodConfig, DodItem } from '../../api/client'

export const AUTO_APPROVE_DAYS: readonly AutoApproveDays[] = [0, 3, 5]
/** UAT's "Never" choice: the ticket waits for the client's answer however long it takes. */
export const AUTO_APPROVE_NEVER: AutoApproveDays = 0

/**
 * The Definition of Done a ticket is held to: code on a branch doesn't apply while the project has
 * no repository (API: dod.Config.ForRepository). Also where a form's draft starts then.
 */
export function forRepository(c: DodConfig, hasRepository: boolean): DodConfig {
  return hasRepository ? c : { ...c, codeOnBranch: { ...c.codeOnBranch, enabled: false } }
}

/** The condition is on but can't apply: the project has no repository. */
export function codePaused(c: DodConfig, hasRepository: boolean): boolean {
  return c.codeOnBranch.enabled && !hasRepository
}

/** The plain-language sentence after "Done means:" (Projects mock). */
export function doneMeans(t: TFunction, c: DodConfig, language: string): string {
  const parts: string[] = []
  if (c.codeOnBranch.enabled) parts.push(t('dod.conditions.code', { branch: c.codeOnBranch.branch }))
  if (c.uat.enabled) {
    const days = c.uat.autoApproveWorkingDays
    parts.push(days === AUTO_APPROVE_NEVER ? t('dod.conditions.uat') : t('dod.conditions.uatAuto', { count: days }))
  }
  if (c.requesterConfirms) parts.push(t('dod.conditions.confirm'))
  if (parts.length === 0) return t('dod.nothing')
  const list = new Intl.ListFormat(language, { style: 'long', type: 'conjunction' }).format(parts)
  return t('dod.sentence', { conditions: list })
}

/**
 * The statuses a ticket passes on its way to done. UAT and the requester's confirmation are one
 * client check In UAT, named after UAT when both are on.
 */
export function flowSteps(t: TFunction, c: DodConfig): string[] {
  const steps = [t('dod.flow.in_progress')]
  if (c.uat.enabled) steps.push(t('dod.flow.in_uat'))
  else if (c.requesterConfirms) steps.push(t('dod.flow.confirm'))
  steps.push(t('dod.flow.resolved'))
  return steps
}

/** The short chip on the project list, e.g. "Code on main + UAT". */
export function dodSummary(t: TFunction, c: DodConfig): string {
  const parts: string[] = []
  if (c.codeOnBranch.enabled) parts.push(t('dod.summary.code', { branch: c.codeOnBranch.branch }))
  if (c.uat.enabled) parts.push(t('dod.summary.uat'))
  if (c.requesterConfirms) parts.push(t('dod.summary.confirm'))
  return parts.length ? parts.join(t('dod.summary.separator')) : t('dod.summary.none')
}

/** A Definition of Done item, short, for sentences ("needs the client's UAT approval"). */
export function itemShort(t: TFunction, item: DodItem): string {
  return t(`dod.short.${item}`)
}

/** A list of items as one phrase, e.g. "the client's UAT approval and the requester's confirmation". */
export function itemsPhrase(t: TFunction, items: readonly DodItem[], language: string): string {
  return new Intl.ListFormat(language, { style: 'long', type: 'conjunction' }).format(items.map((i) => itemShort(t, i)))
}
