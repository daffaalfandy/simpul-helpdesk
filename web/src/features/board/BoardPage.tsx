import { useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Ticket } from '../../api/client'
import { clientsQueryOptions, useOrgTimeZone } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { FilterPills } from '../../components/FilterPills'
import { SearchField } from '../../components/SearchField'
import { paths, SEARCH_DEBOUNCE_MS } from '../../lib/constants'
import { formatToday } from '../../lib/dates'
import { dayPart, firstName } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import { BOARD_VIEWS, boardQueryOptions, cleanQuery, DEFAULT_VIEW, isFiltered, type BoardSearch } from './boardSearch'
import { KanbanBoard } from './KanbanBoard'
import { LogTicketDialog } from './LogTicketDialog'
import { StatTiles } from './StatTiles'

type BoardPageProps = { search: BoardSearch; onSearchChange: (next: BoardSearch) => void }

/** Agent home (Main mock): stats, filters, search and the kanban board. */
export function BoardPage({ search, onSearchChange }: BoardPageProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const { data: clients } = useSuspenseQuery(clientsQueryOptions)
  const { data: board } = useSuspenseQuery(boardQueryOptions(search))
  const timeZone = useOrgTimeZone()
  const [logging, setLogging] = useState(false)
  const [created, setCreated] = useState<Ticket | null>(null)

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
      <StatTiles stats={board.stats} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <FilterPills
          label={t('agent.board.view.label')}
          options={BOARD_VIEWS.map((id) => ({ id, label: t(`agent.board.view.${id}`) }))}
          value={search.view ?? DEFAULT_VIEW}
          onChange={(view) => onSearchChange({ ...search, view: view === DEFAULT_VIEW ? undefined : view })}
        />
        <BoardSearchBox value={search.q ?? ''} onCommit={(q) => onSearchChange({ ...search, q: q || undefined })} />
      </div>
      <KanbanBoard board={board} filtered={isFiltered(search)} />
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
