import { useSuspenseInfiniteQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Check, ChevronRight, Inbox, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { TicketSummary, UatAnswer, UatRequest } from '../../api/client'
import { Alert } from '../../components/Alert'
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
import { uatRequestsQueryOptions } from './queries'
import { UatAnswerDialog } from './UatAnswerDialog'

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
  const { data: tests } = useSuspenseQuery(uatRequestsQueryOptions)

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl leading-tight font-extrabold tracking-title">
            {t('portal.hello', { name: firstName(me.name) })}
          </h1>
          <p className="text-base text-muted">
            {tests.length > 0 ? t('portal.home.waiting', { count: tests.length }) : t('portal.home.intro')}
          </p>
        </div>
        <Link to={paths.portalNew} className={buttonClasses('primary', 'lg')}>
          <Plus aria-hidden="true" className="size-(--icon-md)" />
          {t('portal.home.report')}
        </Link>
      </div>
      <TestRequests tests={tests} meId={me.id} />
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

/** "Waiting for your testing (UAT)" (PortalHome mock): one navy card per test, with the answers. */
function TestRequests({ tests, meId }: { tests: UatRequest[]; meId: string }) {
  const { t } = useTranslation()
  const [answering, setAnswering] = useState<{ test: UatRequest; answer: UatAnswer } | null>(null)
  const [answered, setAnswered] = useState<{ key: string; answer: UatAnswer } | null>(null)
  if (tests.length === 0 && !answered) return null
  return (
    <section aria-labelledby="uat-requests" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h2 id="uat-requests" className="text-lg font-extrabold tracking-title">
          {t('portal.uat.title')}
        </h2>
        <p className="text-sm text-muted">{t('portal.uat.emailed')}</p>
      </div>
      {answered && (
        <Alert tone="success">
          {answered.answer === 'approve' ? t('portal.uat.approved', { key: answered.key }) : t('portal.uat.rejected')}
        </Alert>
      )}
      {tests.length > 0 && (
        <ul className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--uat-card-min-w)),1fr))] gap-3.5">
          {tests.map((test) => (
            <TestCard
              key={test.key}
              test={test}
              meId={meId}
              onAnswer={(answer) => {
                setAnswered(null)
                setAnswering({ test, answer })
              }}
            />
          ))}
        </ul>
      )}
      {answering && (
        <UatAnswerDialog
          ticketKey={answering.test.key}
          subject={answering.test.subject}
          answer={answering.answer}
          onClose={() => setAnswering(null)}
          onAnswered={() => {
            setAnswered({ key: answering.test.key, answer: answering.answer })
            setAnswering(null)
          }}
        />
      )}
    </section>
  )
}

function TestCard({ test, meId, onAnswer }: { test: UatRequest; meId: string; onAnswer: (a: UatAnswer) => void }) {
  const { t } = useTranslation()
  const { day } = useTimestamp()
  return (
    <li className="flex flex-col gap-2.5 rounded-2xl bg-ink px-5 py-4.5 text-on-primary shadow-card">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-xs font-bold">
        <span className="text-on-dark-accent">{t('portal.uat.ticketProject', { key: test.key, project: test.project.name })}</span>
        {test.autoApproveAt && <span className="text-on-dark-muted">{t('portal.uat.autoApproves', { date: day(test.autoApproveAt) })}</span>}
      </div>
      <Link
        to={paths.portalTicket}
        params={{ key: test.key }}
        className="text-md font-extrabold text-on-primary underline underline-offset-2 hover:text-on-dark-accent"
      >
        {test.subject}
      </Link>
      {test.requester.id !== meId && (
        <span className="text-xs text-on-dark-muted">{t('portal.uat.reportedBy', { name: test.requester.name })}</span>
      )}
      <p className="line-clamp-3 text-sm leading-relaxed text-on-dark-muted">{test.instructions ?? t('portal.uat.noInstructions')}</p>
      <div className="mt-auto flex flex-wrap gap-2.5 pt-1">
        <Button variant="success" icon={<Check aria-hidden="true" className="size-(--icon-md)" />} onPress={() => onAnswer('approve')}>
          {t('portal.uat.approve')}
        </Button>
        <Button variant="danger" icon={<X aria-hidden="true" className="size-(--icon-md)" />} onPress={() => onAnswer('reject')}>
          {t('portal.uat.reject')}
        </Button>
      </div>
    </li>
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
