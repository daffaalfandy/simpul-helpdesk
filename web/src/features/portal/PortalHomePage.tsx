import { useSuspenseInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRight, Inbox, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { TicketSummary } from '../../api/client'
import { Pill } from '../../components/Badges'
import { Button, buttonClasses } from '../../components/Button'
import { Card } from '../../components/Card'
import { FilterPills } from '../../components/FilterPills'
import { paths } from '../../lib/constants'
import { firstName } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import { requesterStatus } from '../tickets/labels'
import { ticketListQueryOptions } from '../tickets/queries'
import { PORTAL_FILTERS, STATUS_TONE, statusesForPortalFilter, type PortalFilter } from '../tickets/status'
import { useTimestamp } from '../tickets/time'

/** The list behind a portal filter; shared by the route loader and the page. */
export function portalListOptions(filter: PortalFilter) {
  const status = statusesForPortalFilter(filter)
  return ticketListQueryOptions(status.length > 0 ? { status: [...status] } : {})
}

type PortalHomePageProps = { filter: PortalFilter; onFilterChange: (filter: PortalFilter) => void }

/** Portal home (HANDOFF §4, PortalHome.dc.html): report an issue and follow the client's tickets. */
export function PortalHomePage({ filter, onFilterChange }: PortalHomePageProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const list = useSuspenseInfiniteQuery(portalListOptions(filter))
  const tickets = list.data.pages.flat()

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl leading-tight font-extrabold tracking-title">
            {t('portal.hello', { name: firstName(me.name) })}
          </h1>
          <p className="text-base text-muted">{t('portal.home.intro')}</p>
        </div>
        <Link to={paths.portalNew} className={buttonClasses('primary', 'lg')}>
          <Plus aria-hidden="true" className="size-(--icon-md)" />
          {t('portal.home.report')}
        </Link>
      </div>
      <Card
        as="h2"
        title={t('portal.home.myTickets')}
        actions={
          <FilterPills
            label={t('portal.home.filterLabel')}
            options={PORTAL_FILTERS.map((id) => ({ id, label: t(`portal.home.filters.${id}`) }))}
            value={filter}
            onChange={onFilterChange}
          />
        }
      >
        {tickets.length === 0 ? (
          <p className="flex flex-col items-center gap-2 py-8 text-center text-sm text-muted">
            <Inbox aria-hidden="true" className="size-(--icon-lg)" />
            {t(`portal.home.empty.${filter}`)}
          </p>
        ) : (
          <ul className="flex flex-col">
            {tickets.map((ticket) => (
              <TicketRow key={ticket.id} ticket={ticket} meId={me.id} />
            ))}
          </ul>
        )}
        {list.hasNextPage && (
          <Button variant="outline" className="self-center" onPress={() => list.fetchNextPage()} isPending={list.isFetchingNextPage}>
            {t('common.showMore')}
          </Button>
        )}
      </Card>
    </>
  )
}

function TicketRow({ ticket, meId }: { ticket: TicketSummary; meId: string }) {
  const { t } = useTranslation()
  const { stamp } = useTimestamp()
  const type = t(`ticket.type.${ticket.type}`)
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 border-t border-border-subtle py-3.5">
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-xs font-semibold text-muted">
          {ticket.type === 'incident'
            ? t('ticket.keyTypePriority', { key: ticket.key, type, priority: ticket.priority })
            : t('ticket.keyType', { key: ticket.key, type })}
        </span>
        <span className="text-md font-bold wrap-break-word text-ink">{ticket.subject}</span>
        <span className="flex flex-wrap gap-x-3 text-sm text-muted">
          {ticket.requester.id !== meId && <span>{t('portal.home.reportedBy', { name: ticket.requester.name })}</span>}
          <span>
            {ticket.resolvedAt
              ? t('portal.home.resolvedOn', { when: stamp(ticket.resolvedAt) })
              : t('portal.home.updated', { when: stamp(ticket.updatedAt) })}
          </span>
        </span>
      </div>
      <div className="flex items-center gap-3">
        <Pill tone={STATUS_TONE[ticket.status]}>{requesterStatus(t, ticket.status)}</Pill>
        <Link
          to={paths.portalTicket}
          params={{ key: ticket.key }}
          aria-label={t('portal.home.viewDetailsLabel', { key: ticket.key })}
          className="inline-flex min-h-(--control-h-md) items-center gap-1 rounded-md px-3.5 text-sm font-bold text-primary no-underline hover:bg-primary-tint hover:text-primary-hover"
        >
          {t('portal.home.viewDetails')}
          <ChevronRight aria-hidden="true" className="size-(--icon-sm)" />
        </Link>
      </div>
    </li>
  )
}
