import { useSuspenseInfiniteQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Ticket } from '../../api/client'
import { clientsQueryOptions } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Dropdown } from '../../components/Dropdown'
import { FilterPills } from '../../components/FilterPills'
import { paths } from '../../lib/constants'
import { dayPart, firstName } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import { agentStatus } from '../tickets/labels'
import { TICKET_STATUSES } from '../tickets/status'
import { BOARD_VIEWS, boardListOptions, DEFAULT_STATUS, DEFAULT_VIEW, type BoardSearch, type BoardStatus } from './boardSearch'
import { LogTicketDialog } from './LogTicketDialog'
import { TicketTable } from './TicketTable'

const ALL_CLIENTS = 'all'

type BoardPageProps = { search: BoardSearch; onSearchChange: (next: BoardSearch) => void }

/** Agent home. M3 shows a ticket table; the kanban board replaces it in M4. */
export function BoardPage({ search, onSearchChange }: BoardPageProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const { data: clients } = useSuspenseQuery(clientsQueryOptions)
  const list = useSuspenseInfiniteQuery(boardListOptions(search))
  const tickets = list.data.pages.flat()
  const [logging, setLogging] = useState(false)
  const [created, setCreated] = useState<Ticket | null>(null)

  const statusOptions = [
    { id: 'open' as BoardStatus, label: t('agent.board.statusFilter.open') },
    { id: 'all' as BoardStatus, label: t('agent.board.statusFilter.all') },
    ...TICKET_STATUSES.map((s) => ({ id: s as BoardStatus, label: agentStatus(t, s) })),
  ]

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h1 className="text-3xl leading-tight font-extrabold tracking-title">
          {t(`agent.greeting.${dayPart(new Date())}`, { name: firstName(me.name) })}
        </h1>
        <Button
          size="lg"
          icon={<Plus aria-hidden="true" className="size-(--icon-md)" />}
          onPress={() => {
            setCreated(null)
            setLogging(true)
          }}
        >
          {t('agent.board.logTicket')}
        </Button>
      </div>
      {created && (
        <Alert tone="success">
          <span className="flex flex-wrap gap-x-3">
            {created.assignee?.id === me.id
              ? t('agent.board.createdAssigned', { key: created.key })
              : t('agent.board.created', { key: created.key })}
            <Link to={paths.ticket} params={{ key: created.key }} className="underline">
              {t('agent.board.openTicket', { key: created.key })}
            </Link>
          </span>
        </Alert>
      )}
      <Card>
        <div className="flex flex-wrap items-end gap-3">
          <FilterPills
            label={t('agent.board.view.label')}
            options={BOARD_VIEWS.map((id) => ({ id, label: t(`agent.board.view.${id}`) }))}
            value={search.view ?? DEFAULT_VIEW}
            onChange={(view) => onSearchChange({ ...search, view: view === DEFAULT_VIEW ? undefined : view })}
          />
          <Dropdown
            label={t('agent.board.statusFilter.label')}
            options={statusOptions}
            value={search.status ?? DEFAULT_STATUS}
            onChange={(status) => onSearchChange({ ...search, status: status === DEFAULT_STATUS ? undefined : status })}
            className="min-w-48"
          />
          <Dropdown
            label={t('agent.board.clientFilter.label')}
            options={[
              { id: ALL_CLIENTS, label: t('agent.board.clientFilter.all') },
              ...clients.map((c) => ({ id: c.id, label: c.name })),
            ]}
            value={search.client ?? ALL_CLIENTS}
            onChange={(client) => onSearchChange({ ...search, client: client === ALL_CLIENTS ? undefined : client })}
            className="min-w-56"
          />
        </div>
        {tickets.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">{t('agent.board.empty')}</p>
        ) : (
          <TicketTable tickets={tickets} />
        )}
        {list.hasNextPage && (
          <Button variant="outline" className="self-center" onPress={() => list.fetchNextPage()} isPending={list.isFetchingNextPage}>
            {t('common.showMore')}
          </Button>
        )}
      </Card>
      {logging && (
        <LogTicketDialog
          onClose={() => setLogging(false)}
          onCreated={(ticket) => {
            setLogging(false)
            setCreated(ticket)
          }}
        />
      )}
    </>
  )
}
