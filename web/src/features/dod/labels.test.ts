import { describe, expect, it } from 'vitest'

import type { DodConfig } from '../../api/client'
import i18n from '../../i18n'
import { noDod } from '../../test/projectsApi'
import { dodSummary, doneMeans, flowSteps } from './labels'

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
})
