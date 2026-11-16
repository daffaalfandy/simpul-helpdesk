import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { GitBranch } from 'lucide-react'
import { Trans, useTranslation } from 'react-i18next'

import type { CodeEventLog } from '../../../api/client'
import { Button } from '../../../components/Button'
import { Card } from '../../../components/Card'
import { ErrorAlert } from '../../../components/ErrorAlert'
import { Spinner } from '../../../components/Spinner'
import { useCodeLabels } from '../../code/labels'
import { useTimestamp } from '../../tickets/time'
import { codeEventsQueryOptions } from './queries'

/** Outcomes that left the ticket alone: the key is shown, not linked, for unknown tickets. */
const IGNORED = ['ticket_closed', 'unknown_ticket', 'other_repository']

/** How much of a commit's first line the log quotes. */
const TITLE_CHARS = 48

/** "Recent events" (Integrations mock): what the latest code events did, newest first. */
export function RecentEvents() {
  const { t } = useTranslation()
  const events = useQuery(codeEventsQueryOptions)
  return (
    <Card as="h3" title={t('settings.integrations.log.title')} description={t('settings.integrations.log.description')}>
      {events.isPending ? (
        <p className="flex items-center gap-2 text-sm text-muted">
          <Spinner />
          {t('settings.integrations.log.loading')}
        </p>
      ) : events.isError ? (
        <div className="flex flex-col items-start gap-3">
          <ErrorAlert error={events.error} />
          <Button variant="outline" size="sm" onPress={() => void events.refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      ) : events.data.length === 0 ? (
        <p className="flex items-start gap-2 text-sm text-muted">
          <GitBranch aria-hidden="true" className="mt-0.5 size-(--icon-md) shrink-0" />
          {t('settings.integrations.log.empty')}
        </p>
      ) : (
        <ul className="flex flex-col gap-2.5 text-sm">
          {events.data.map((e) => (
            <LogRow key={e.id} entry={e} />
          ))}
        </ul>
      )}
    </Card>
  )
}

function LogRow({ entry: e }: { entry: CodeEventLog }) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const { stamp } = useTimestamp()
  const ignored = IGNORED.includes(e.outcome)
  const title = e.title.length > TITLE_CHARS ? t('settings.integrations.log.cut', { text: e.title.slice(0, TITLE_CHARS) }) : e.title
  const what = t(`settings.integrations.log.kind.${e.kind}`, {
    ref: e.ref,
    request: labels.request(e.provider, e.ref),
    title,
    branch: e.branch,
    target: e.target,
    repo: e.repository.split('/').pop(),
  })
  const key = e.ticketFound ? (
    <Link to="/tickets/$key" params={{ key: e.ticketKey }} className="font-bold text-primary hover:text-primary-hover" />
  ) : (
    <span className="font-bold" />
  )
  const outcome = e.statusTo ? 'moved' : e.outcome
  const item = e.items.at(-1)
  return (
    <li className="flex flex-wrap gap-x-2.5 gap-y-0.5">
      <span className="min-w-(--log-time-w) font-semibold text-muted tabular-nums">{stamp(e.at)}</span>
      <span className="min-w-(--log-provider-w) font-bold">{labels.provider(e.provider)}</span>
      <span className={ignored ? 'text-chip-danger-fg' : undefined}>
        <Trans
          i18nKey={`settings.integrations.log.outcome.${outcome}`}
          values={{
            what,
            key: e.ticketKey,
            status: e.statusTo ? t(`ticket.status.agent.${e.statusTo}`) : '',
            item: item ? t(`dod.check.${item}`, { branch: e.target || e.branch }) : '',
          }}
          components={{ key }}
        />
      </span>
    </li>
  )
}
