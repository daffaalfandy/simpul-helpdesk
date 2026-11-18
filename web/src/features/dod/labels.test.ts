import { describe, expect, it } from 'vitest'

import type { DodConfig } from '../../api/client'
import i18n from '../../i18n'
import { noDod } from '../../test/projectsApi'
import { codePaused, dodSummary, doneMeans, flowSteps, forRepository } from './labels'

const t = i18n.t.bind(i18n)
const with_ = (over: Partial<DodConfig>): DodConfig => ({ ...noDod, ...over })

describe('Definition of Done labels', () => {
  it('summarises a configuration as the project list chip', () => {
    expect(dodSummary(t, noDod)).toBe('Resolved by hand')
    expect(dodSummary(t, with_({ uat: { enabled: true, autoApproveWorkingDays: 3 } }))).toBe('UAT')
    expect(dodSummary(t, with_({ uat: { enabled: true, autoApproveWorkingDays: 0 }, requesterConfirms: true }))).toBe(
      'UAT + Requester confirms',
    )
  })

  it('writes the sentence and the flow', () => {
    const c = with_({ uat: { enabled: true, autoApproveWorkingDays: 3 } })
    expect(doneMeans(t, c, 'en')).toBe('the client approves UAT (auto-approved after 3 working days).')
    expect(flowSteps(t, c)).toEqual(['In progress', 'In UAT', 'Resolved'])
    expect(flowSteps(t, with_({ requesterConfirms: true }))).toEqual(['In progress', 'Awaiting confirmation', 'Resolved'])
    expect(doneMeans(t, noDod, 'en')).toBe('nothing automatic. An agent resolves each ticket by hand.')
  })

  it('puts code on a branch first, and adds no status for it', () => {
    const c = with_({ codeOnBranch: { enabled: true, branch: 'main' }, uat: { enabled: true, autoApproveWorkingDays: 0 } })
    expect(dodSummary(t, c)).toBe('Code on main + UAT')
    expect(doneMeans(t, c, 'en')).toBe('a commit naming the ticket reaches main and the client approves UAT.')
    expect(flowSteps(t, c)).toEqual(['In progress', 'In UAT', 'Resolved'])
  })

  it('drops code on a branch while the project has no repository', () => {
    const c = with_({ codeOnBranch: { enabled: true, branch: 'main' } })
    expect(forRepository(c, true)).toBe(c)
    expect(forRepository(c, false).codeOnBranch).toEqual({ enabled: false, branch: 'main' })
    expect(codePaused(c, false)).toBe(true)
    expect(codePaused(c, true)).toBe(false)
    expect(codePaused(noDod, false)).toBe(false)
  })
})
