import { useInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { Board, TicketStatus } from '../../api/client'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths } from '../../lib/constants'
import { agentStatus } from '../tickets/labels'
import { ticketListQueryOptions } from '../tickets/queries'
import { STATUS_TONE } from '../tickets/status'
import { useTimestamp } from '../tickets/time'

/** The statuses that can be off the board: open tickets always show on it. */
const OFF_BOARD: TicketStatus[] = ['resolved', 'closed']

/**
 * Search matches the board can't show: its resolved column holds the done tickets reported on
 * the chosen days only, so one reported on other days stays findable by key, subject or client.
 * The list has no date limit; the cards already on the board are left out.
 */
export function EarlierTickets({ q, board }: { q: string; board: Board }) {
  const { t } = useTranslation()
  const { stamp } = useTimestamp()
  const list = useInfiniteQuery(ticketListQueryOptions({ q, status: OFF_BOARD }))
  const onBoard = new Set(board.columns.flatMap((c) => c.cards.map((card) => card.key)))
  const found = (list.data?.pages.flat() ?? []).filter((ticket) => !onBoard.has(ticket.key))

  if (list.isPending) return <p className="text-sm text-muted">{t('board.earlier.loading')}</p>
  if (list.isError) return <ErrorAlert error={list.error} />
  if (found.length === 0) return null
  return (
    <Card as="h2" title={t('board.earlier.title')} description={t('board.earlier.description')} labelledBy="earlier-tickets">
      <ul className="flex flex-col">
        {found.map((ticket) => (
          <li key={ticket.id} className="flex flex-wrap items-center justify-between gap-3 border-t border-border-subtle py-3">
            <div className="flex min-w-0 flex-col gap-0.5">
              <Link to={paths.ticket} params={{ key: ticket.key }} className="font-bold wrap-break-word">
                {t('board.earlier.ticket', { key: ticket.key, subject: ticket.subject })}
              </Link>
              <span className="text-sm text-muted">
                {[ticket.client.name, ticket.resolvedAt && t('board.earlier.resolvedOn', { when: stamp(ticket.resolvedAt) })]
                  .filter(Boolean)
                  .join(t('common.separator'))}
              </span>
            </div>
            <Pill tone={STATUS_TONE[ticket.status]}>{agentStatus(t, ticket.status)}</Pill>
          </li>
        ))}
      </ul>
      {list.hasNextPage && (
        <Button variant="outline" className="self-center" onPress={() => list.fetchNextPage()} isPending={list.isFetchingNextPage}>
          {t('common.showMore')}
        </Button>
      )}
    </Card>
  )
}
