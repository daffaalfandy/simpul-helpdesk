import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import type { TicketType } from '../../api/client'
import { usePriorityNames } from '../../api/queries'
import { Card } from '../../components/Card'
import { ErrorAlert } from '../../components/ErrorAlert'
import { Spinner } from '../../components/Spinner'
import { activityText } from './labels'
import { ticketActivityQueryOptions } from './queries'
import { Timestamp } from './Timestamp'

/** The ticket's audit trail (staff only). */
export function ActivityLog({ ticketKey, type }: { ticketKey: string; type: TicketType }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const { data, error, isPending } = useQuery(ticketActivityQueryOptions(ticketKey))
  return (
    <Card as="h2" title={t('ticket.activity.title')}>
      <ErrorAlert error={error} />
      {isPending && (
        <p role="status" className="flex items-center gap-2 text-sm text-muted">
          <Spinner />
          {t('ticket.activity.loading')}
        </p>
      )}
      {data && (
        <ol className="flex flex-col gap-2.5">
          {data.map((entry) => (
            <li key={entry.id} className="grid grid-cols-[auto_1fr] gap-x-3 text-sm">
              <span className="text-xs whitespace-nowrap text-muted tabular-nums">
                <Timestamp iso={entry.at} />
              </span>
              <span className="text-ink">{activityText(t, names, entry, type)}</span>
            </li>
          ))}
        </ol>
      )}
      <p className="text-xs text-muted">{t('ticket.activity.footer')}</p>
    </Card>
  )
}
