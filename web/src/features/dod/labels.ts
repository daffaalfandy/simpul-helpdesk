import type { TFunction } from 'i18next'

import type { AutoApproveDays, DodBranch, DodConfig, DodItem } from '../../api/client'

export const DOD_BRANCHES: readonly DodBranch[] = ['main', 'develop', 'release/*', 'staging']
export const AUTO_APPROVE_DAYS: readonly AutoApproveDays[] = [0, 3, 5]
/** UAT's "Never" choice: the ticket waits for the client's answer however long it takes. */
export const AUTO_APPROVE_NEVER: AutoApproveDays = 0

/** The conditions read from a repository: merge, CI, deploy. */
export function hasCodeConditions(c: DodConfig): boolean {
  return c.merged.enabled || c.ciPassed || c.deployedProduction
}

/** The plain-language sentence after "Done means:" (Projects mock). */
export function doneMeans(t: TFunction, c: DodConfig, language: string): string {
  const branch = c.merged.branch
  const parts: string[] = []
  if (c.merged.enabled) parts.push(t('dod.conditions.merged', { branch }))
  if (c.ciPassed) parts.push(t('dod.conditions.ci', { branch }))
  if (c.deployedProduction) parts.push(t('dod.conditions.deployed'))
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
  if (c.deployedProduction) steps.push(t('dod.flow.mergedDeployed'))
  else if (c.merged.enabled || c.ciPassed) steps.push(t('dod.flow.merged'))
  if (c.uat.enabled) steps.push(t('dod.flow.in_uat'))
  else if (c.requesterConfirms) steps.push(t('dod.flow.confirm'))
  steps.push(t('dod.flow.resolved'))
  return steps
}

/** The short chip on the project list, e.g. "Merged → main + UAT". */
export function dodSummary(t: TFunction, c: DodConfig): string {
  const parts: string[] = []
  if (c.merged.enabled) parts.push(t('dod.summary.merged', { branch: c.merged.branch }))
  // The mock's chip leaves CI out next to the merge it follows; alone, it has to show.
  else if (c.ciPassed) parts.push(t('dod.summary.ci', { branch: c.merged.branch }))
  if (c.deployedProduction) parts.push(t('dod.summary.deployed'))
  if (c.uat.enabled) parts.push(t('dod.summary.uat'))
  if (c.requesterConfirms) parts.push(t('dod.summary.confirm'))
  return parts.length ? parts.join(t('dod.summary.separator')) : t('dod.summary.none')
}

/** A Definition of Done item, short, for sentences ("needs code merged and the client's UAT approval"). */
export function itemShort(t: TFunction, item: DodItem): string {
  return t(`dod.short.${item}`)
}

/** A list of items as one phrase, e.g. "code merged and the client's UAT approval". */
export function itemsPhrase(t: TFunction, items: readonly DodItem[], language: string): string {
  return new Intl.ListFormat(language, { style: 'long', type: 'conjunction' }).format(items.map((i) => itemShort(t, i)))
}
