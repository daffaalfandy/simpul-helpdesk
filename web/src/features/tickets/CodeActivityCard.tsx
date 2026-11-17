import { useQuery } from '@tanstack/react-query'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { Ticket } from '../../api/client'
import { Pill } from '../../components/Badges'
import { Card } from '../../components/Card'
import { ErrorAlert } from '../../components/ErrorAlert'
import { ExternalLink } from '../../components/ExternalLink'
import { Spinner } from '../../components/Spinner'
import { SLA_REFRESH_MS } from '../../lib/constants'
import { useCodeLabels } from '../code/labels'
import { ticketCodeQueryOptions } from './queries'
import { useTimestamp } from './time'

/**
 * Code activity (Ticket mock): the commits that named the ticket and the branches each one has
 * reached, e.g. the fix branch, then main once it merged. Shown once the ticket's project reads a
 * repository, or once a commit reached the ticket. Commits never move the ticket.
 */
export function CodeActivityCard({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const { stamp } = useTimestamp()
  const { data, error, isPending } = useQuery({ ...ticketCodeQueryOptions(ticket.key), refetchInterval: SLA_REFRESH_MS })
  const empty = data && data.commits.length === 0
  // Without a repository the card waits for a commit to show up, so most support tickets never see it.
  if (!data?.repository && (!data || empty)) return null
  const repo = data.repository
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
      {data.commits.length > 0 && (
        <ul aria-label={t('ticket.code.commits')} className="flex flex-col gap-3 text-sm">
          {data.commits.map((c) => (
            <li key={c.sha} className="flex flex-col gap-1">
              <span className="flex flex-wrap items-baseline gap-x-2">
                <ExternalLink href={c.url} className="font-mono text-xs font-semibold text-chip-code-fg">
                  {c.sha}
                </ExternalLink>
                <span className="min-w-0 wrap-break-word">{c.title}</span>
              </span>
              <ol aria-label={t('ticket.code.branches')} className="flex flex-wrap items-center gap-1">
                {c.branches.map((b, i) => (
                  <li key={b} className="flex items-center gap-1">
                    {i > 0 && <ArrowRight aria-hidden="true" className="size-(--icon-sm) text-muted" />}
                    <Pill tone="code">{b}</Pill>
                  </li>
                ))}
              </ol>
              <span className="text-xs text-muted">{t('ticket.code.commitMeta', { author: c.author, time: stamp(c.at) })}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
