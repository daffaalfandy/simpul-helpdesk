import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { TicketSummary } from '../../api/client'
import { Avatar } from '../../components/Avatar'
import { Pill, PriorityBadge } from '../../components/Badges'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { paths } from '../../lib/constants'
import { agentStatus } from '../tickets/labels'
import { STATUS_TONE } from '../tickets/status'
import { Timestamp } from '../tickets/Timestamp'

/** Temporary ticket list for M3; M4 replaces it with the kanban board. */
export function TicketTable({ tickets }: { tickets: ReadonlyArray<TicketSummary> }) {
  const { t } = useTranslation()
  return (
    <Table caption={t('agent.board.caption')}>
      <thead>
        <tr>
          <Th>{t('agent.board.cols.key')}</Th>
          <Th>{t('agent.board.cols.subject')}</Th>
          <Th>{t('agent.board.cols.client')}</Th>
          <Th>{t('agent.board.cols.priority')}</Th>
          <Th>{t('agent.board.cols.status')}</Th>
          <Th>{t('agent.board.cols.assignee')}</Th>
          <Th>{t('agent.board.cols.updated')}</Th>
        </tr>
      </thead>
      <tbody>
        {tickets.map((ticket) => (
          <tr key={ticket.id}>
            <RowHeader className="whitespace-nowrap">
              <Link to={paths.ticket} params={{ key: ticket.key }} className="font-bold text-primary hover:text-primary-hover">
                {ticket.key}
              </Link>
            </RowHeader>
            <Td className="min-w-60 font-semibold">{ticket.subject}</Td>
            <Td>{ticket.client.name}</Td>
            <Td>
              <PriorityBadge priority={ticket.priority} />
            </Td>
            <Td>
              <Pill tone={STATUS_TONE[ticket.status]}>{agentStatus(t, ticket.status)}</Pill>
            </Td>
            <Td>
              <span className="flex items-center gap-2 whitespace-nowrap">
                <Avatar name={ticket.assignee?.name} seed={ticket.assignee?.id} size="sm" />
                {ticket.assignee?.name ?? <span className="text-muted">{t('ticket.unassigned')}</span>}
              </span>
            </Td>
            <Td className="text-sm whitespace-nowrap text-muted">
              <Timestamp iso={ticket.updatedAt} />
            </Td>
          </tr>
        ))}
      </tbody>
    </Table>
  )
}
