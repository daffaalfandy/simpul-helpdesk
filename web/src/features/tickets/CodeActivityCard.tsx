import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import type { CodeRequest, Ticket } from '../../api/client'
import { Pill, type PillTone } from '../../components/Badges'
import { Card } from '../../components/Card'
import { ErrorAlert } from '../../components/ErrorAlert'
import { ExternalLink } from '../../components/ExternalLink'
import { Spinner } from '../../components/Spinner'
import { SLA_REFRESH_MS } from '../../lib/constants'
import { useCodeLabels } from '../code/labels'
import { ticketCodeQueryOptions } from './queries'
import { useTimestamp } from './time'

const stateTones: Record<CodeRequest['state'], PillTone> = { open: 'info', merged: 'success', closed: 'neutral' }

/**
 * Code activity (Ticket mock): the pull/merge requests and commits that named the ticket. Shown
 * once the ticket's project reads a repository, or once code reached the ticket.
 */
export function CodeActivityCard({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const { stamp } = useTimestamp()
  const { data, error, isPending } = useQuery({ ...ticketCodeQueryOptions(ticket.key), refetchInterval: SLA_REFRESH_MS })
  const empty = data && data.requests.length === 0 && data.commits.length === 0
  // Without a repository the card waits for code to show up, so most support tickets never see it.
  if (!ticket.dod?.hasRepository && (!data || empty)) return null
  const repo = data?.repository
  return (
    <Card
      variant="compact"
      as="h2"
      labelledBy={`code-${ticket.key}`}
      title={t('ticket.code.title')}
      actions={
        repo && (
          <ExternalLink href={repo.webUrl} className="text-xs font-bold text-primary">
            {t('ticket.code.repository', { provider: labels.provider(repo.provider), name: repo.fullName })}
          </ExternalLink>
        )
      }
    >
      <ErrorAlert error={error} />
      {isPending && (
        <p role="status" className="flex items-center gap-2 text-sm text-muted">
          <Spinner />
          {t('ticket.code.loading')}
        </p>
      )}
      {empty && <p className="text-sm text-muted">{t('ticket.code.empty', { key: ticket.key })}</p>}
      {data?.requests.map((r) => (
        <div key={`${r.provider}-${r.number}`} className="flex flex-col gap-1 rounded-lg bg-chip-code-bg px-3 py-2.5">
          <div className="flex items-start justify-between gap-2">
            <ExternalLink href={r.url} className="min-w-0 text-sm font-bold wrap-break-word text-ink">
              {t('ticket.code.request', { request: labels.request(r.provider, r.number), title: r.title })}
            </ExternalLink>
            <Pill tone={stateTones[r.state]}>{t(`ticket.code.state.${r.state}`)}</Pill>
          </div>
          <span className="text-xs text-ink-2">
            {t(r.state === 'merged' ? 'ticket.code.mergedLine' : 'ticket.code.openedLine', {
              branch: r.branch,
              target: r.target,
              author: r.author,
              time: stamp(r.at),
            })}
          </span>
        </div>
      ))}
      {data && data.commits.length > 0 && (
        <ul aria-label={t('ticket.code.commits')} className="flex flex-col gap-2 text-sm">
          {data.commits.map((c) => (
            <li key={c.sha} className="flex flex-wrap items-baseline gap-x-2">
              <ExternalLink href={c.url} className="font-mono text-xs font-semibold text-chip-code-fg">
                {c.sha}
              </ExternalLink>
              <span className="min-w-0 wrap-break-word">{c.title}</span>
              <span className="text-xs text-muted">{t('ticket.code.commitMeta', { author: c.author, time: stamp(c.at) })}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
