import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import type { CodeEventKind, CodeOutcome, GitProvider } from '../../api/client'

/** A code event as the activity log, notifications and the recent-events log name it. */
export type CodeEventRef = {
  provider: GitProvider
  kind: CodeEventKind
  ref: string
  branch: string
  target: string
  outcome?: CodeOutcome
}

/** "PR #42" on GitHub, "MR !57" on GitLab. */
export function requestRef(t: TFunction, provider: GitProvider, number: number | string): string {
  return t(`code.request.${provider}`, { number })
}

/** What a code event was, in plain words: "commit a1f9c2e on fix/HD-1 fixes this ticket", "PR #42 merged into main". */
export function codeEventText(t: TFunction, c: CodeEventRef): string {
  const request = requestRef(t, c.provider, c.ref)
  switch (c.kind) {
    case 'commit': {
      const values = { sha: c.ref, branch: c.branch }
      switch (c.outcome) {
        case 'reopened':
          return t('code.event.commitReopened', values)
        case 'merged':
          return t('code.event.commitLanded', values)
        case 'fix_committed':
          return t('code.event.commitFixes', values)
      }
      return t('code.event.commit', values)
    }
    case 'pr_opened':
      return t('code.event.pr_opened', { request, target: c.target })
    case 'pr_merged':
      return t('code.event.pr_merged', { request, target: c.target })
    case 'pr_closed':
      return t('code.event.pr_closed', { request })
    case 'pipeline':
      return t('code.event.pipeline', { branch: c.branch })
    case 'deployment':
      return t('code.event.deployment', { environment: c.ref })
  }
}

/** Provider names and pull/merge request references, for components. */
export function useCodeLabels() {
  const { t } = useTranslation()
  return {
    provider: (p: GitProvider) => t(`code.providers.${p}`),
    request: (p: GitProvider, number: number | string) => requestRef(t, p, number),
    event: (c: CodeEventRef) => codeEventText(t, c),
  }
}
