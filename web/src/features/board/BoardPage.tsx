import { useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { BoardCard, BoardColumnId, Ticket } from '../../api/client'
import { clientsQueryOptions, useOrgTimeZone } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { FilterPills } from '../../components/FilterPills'
import { SearchField } from '../../components/SearchField'
import { BOARD_REFRESH_MS, paths, SEARCH_DEBOUNCE_MS } from '../../lib/constants'
import { formatToday } from '../../lib/dates'
import { dayPart, firstName } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import {
  BOARD_VIEWS,
  boardQueryOptions,
  cleanQuery,
  DEFAULT_VIEW,
  isFiltered,
  RISK_VIEW,
  type BoardSearch,
} from './boardSearch'
import { dropStatus } from './columns'
import { KanbanBoard } from './KanbanBoard'
import { LogTicketDialog } from './LogTicketDialog'
import { SlaBanner } from './SlaBanner'
import { StatTiles } from './StatTiles'
import { useMoveTicket } from './useMoveTicket'

type BoardPageProps = { search: BoardSearch; onSearchChange: (next: BoardSearch) => void }

/** What the last action did, shown above the board until the next one (the mock's toast). */
type Notice =
  | { kind: 'created'; ticket: Ticket }
  | { kind: 'moved'; key: string; column: BoardColumnId; paused: boolean }
  | { kind: 'error'; error: unknown }

/** Agent home (Main mock): stats, filters, search and the kanban board. */
export function BoardPage({ search, onSearchChange }: BoardPageProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const { data: clients } = useSuspenseQuery(clientsQueryOptions)
  const [dragging, setDragging] = useState<BoardCard | null>(null)
  const move = useMoveTicket(search)
  // Others move tickets too: refresh now and then, but never under a card being moved.
  const { data: board } = useSuspenseQuery({
    ...boardQueryOptions(search),
    refetchInterval: dragging || move.isPending ? false : BOARD_REFRESH_MS,
  })
  const timeZone = useOrgTimeZone()
  const [logging, setLogging] = useState(false)
  const [notice, setNotice] = useState<Notice | null>(null)

  const moveTo = (card: BoardCard, column: BoardColumnId) => {
    setDragging(null)
    setNotice(null)
    move.mutate(
      { card, column, status: dropStatus(column, card) },
      {
        onSuccess: (ticket) =>
          setNotice({
            kind: 'moved',
            key: card.key,
            column,
            paused: ticket.sla.resolutionClock?.phase === 'paused',
          }),
        onError: (error) => setNotice({ kind: 'error', error }),
      },
    )
  }

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl leading-tight font-extrabold tracking-title">
            {t(`agent.greeting.${dayPart(new Date())}`, { name: firstName(me.name) })}
          </h1>
          <p className="text-base text-muted">
            {t('agent.board.subtitle', { date: formatToday(timeZone), count: clients.length })}
          </p>
        </div>
        <Button
          icon={<Plus aria-hidden="true" className="size-(--icon-md)" />}
          onPress={() => {
            setNotice(null)
            setLogging(true)
          }}
        >
          {t('agent.board.logTicket')}
        </Button>
      </div>
      {notice?.kind === 'created' && (
        <Alert tone="success">
          <span className="flex flex-wrap gap-x-3">
            {notice.ticket.assignee?.id === me.id
              ? t('agent.board.createdAssigned', { key: notice.ticket.key })
              : t('agent.board.created', { key: notice.ticket.key })}
            <Link to={paths.ticket} params={{ key: notice.ticket.key }} className="underline">
              {t('agent.board.openTicket', { key: notice.ticket.key })}
            </Link>
          </span>
        </Alert>
      )}
      {notice?.kind === 'moved' && (
        <Alert tone="success">
          {t(notice.paused ? 'board.movedPaused' : 'board.moved', {
            key: notice.key,
            column: t(`board.moveTarget.${notice.column}`),
          })}
        </Alert>
      )}
      {notice?.kind === 'error' && <ErrorAlert error={notice.error} />}
      <StatTiles stats={board.stats} />
      <SlaBanner
        stats={board.stats}
        onShow={search.view === RISK_VIEW ? undefined : () => onSearchChange({ ...search, view: RISK_VIEW })}
      />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <FilterPills
          label={t('agent.board.view.label')}
          options={BOARD_VIEWS.map((id) => ({ id, label: t(`agent.board.view.${id}`) }))}
          value={search.view ?? DEFAULT_VIEW}
          onChange={(view) => onSearchChange({ ...search, view: view === DEFAULT_VIEW ? undefined : view })}
        />
        <BoardSearchBox value={search.q ?? ''} onCommit={(q) => onSearchChange({ ...search, q: q || undefined })} />
      </div>
      <p className="-mt-2 text-sm text-muted">{t('board.hint')}</p>
      <KanbanBoard
        board={board}
        filtered={isFiltered(search)}
        dragging={dragging}
        onDraggingChange={setDragging}
        onMove={moveTo}
      />
      {logging && (
        <LogTicketDialog
          onClose={() => setLogging(false)}
          onCreated={(ticket) => {
            setLogging(false)
            setNotice({ kind: 'created', ticket })
          }}
        />
      )}
    </>
  )
}

/** The search box asks the API once typing pauses, not on every key. */
function BoardSearchBox({ value, onCommit }: { value: string; onCommit: (q: string) => void }) {
  const { t } = useTranslation()
  const [text, setText] = useState(value)
  useEffect(() => {
    const q = cleanQuery(text)
    if (q === value) return
    const timer = setTimeout(() => onCommit(q), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [text, value, onCommit])
  return (
    <SearchField
      label={t('board.search.label')}
      placeholder={t('board.search.placeholder')}
      value={text}
      onChange={setText}
      className="w-full sm:w-(--board-search-w)"
    />
  )
}
