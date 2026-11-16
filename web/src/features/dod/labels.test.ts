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
    expect(dodSummary(t, with_({ merged: { enabled: true, branch: 'main' }, ciPassed: true, uat: { enabled: true, autoApproveWorkingDays: 3 } }))).toBe(
      'Merged → main + UAT',
    )
    // CI alone isn't "by hand": the ticket still waits for it.
    expect(dodSummary(t, with_({ ciPassed: true, merged: { enabled: false, branch: 'release/*' } }))).toBe('CI passes on release/*')
    expect(dodSummary(t, with_({ requesterConfirms: true }))).toBe('Requester confirms')
  })

  it('writes the sentence and the flow', () => {
    const c = with_({ merged: { enabled: true, branch: 'main' }, deployedProduction: true })
    expect(doneMeans(t, c, 'en')).toBe('the pull/merge request is merged into main and it is deployed to production.')
    expect(flowSteps(t, c)).toEqual(['In progress', 'Merged & deployed', 'Resolved'])
    expect(flowSteps(t, with_({ requesterConfirms: true }))).toEqual(['In progress', 'Awaiting confirmation', 'Resolved'])
  })
})
