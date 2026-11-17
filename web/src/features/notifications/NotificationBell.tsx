import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Bell } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button as AriaButton, Dialog, DialogTrigger, Heading, Popover } from 'react-aria-components'

import type { Audience, Notification, NotificationFilter } from '../../api/client'
import { FilterPills } from '../../components/FilterPills'
import { Spinner } from '../../components/Spinner'
import { NOTIFICATION_SETTINGS_HASH, paths, POPOVER_OFFSET_PX } from '../../lib/constants'
import { cx } from '../../lib/text'
import { NotificationItem } from './NotificationItem'
import { BELL_FILTERS, DEFAULT_FILTER } from './notice'
import { bellQueryOptions, unreadCountQueryOptions, useMarkAllRead, useMarkRead } from './queries'


const footerLink = 'inline-flex min-h-(--control-h-md) items-center text-sm font-bold text-primary no-underline'

/** "Mark all as read" as a text action; greyed out, not hidden, when nothing is unread. */
const markAllText =
  'hit-area inline-flex min-h-(--control-h-sm) cursor-pointer items-center rounded-md px-2 text-sm font-bold text-primary ' +
  'hover:text-primary-hover data-disabled:cursor-not-allowed data-disabled:text-disabled'

/**
 * The bell in every header (HANDOFF §14.3 NotifBell): a red unread count, and a panel with the
 * newest items. Esc and a click outside close it, and focus goes back to the bell.
 */
export function NotificationBell({ audience }: { audience: Audience }) {
  const { t } = useTranslation()
  const [isOpen, setOpen] = useState(false)
  const [filter, setFilter] = useState<NotificationFilter>(DEFAULT_FILTER)
  const unread = useQuery(unreadCountQueryOptions).data?.count ?? 0
  const list = useQuery({ ...bellQueryOptions(filter), enabled: isOpen })
  const markRead = useMarkRead()
  const markAll = useMarkAllRead()
  const page = audience === 'team' ? paths.notifications : paths.portalNotifications

  const open = (n: Notification) => {
    if (n.readAt == null) markRead.mutate({ id: n.id, read: true })
    setOpen(false)
  }

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={setOpen}>
      <AriaButton
        aria-label={unread > 0 ? t('notifications.bell.labelUnread', { count: unread }) : t('notifications.bell.label')}
        className={cx(
          'relative inline-flex size-(--control-h-md) cursor-pointer items-center justify-center rounded-lg border',
          'transition-colors duration-(--duration-fast) ease-standard',
          isOpen
            ? 'border-primary bg-primary-tint text-primary'
            : 'border-border bg-surface text-ink hover:bg-surface-subtle',
        )}
      >
        <Bell aria-hidden="true" className="size-(--icon-lg)" />
        {unread > 0 && (
          <span
            aria-hidden="true"
            className={cx(
              'absolute -top-1 -right-1 inline-flex h-(--count-badge) min-w-(--count-badge) items-center justify-center',
              'rounded-full bg-danger px-1 text-2xs font-extrabold text-on-primary tabular-nums ring-2 ring-surface',
            )}
          >
            {unread}
          </span>
        )}
      </AriaButton>
      <Popover
        placement="bottom end"
        offset={POPOVER_OFFSET_PX}
        className={cx(
          'w-(--bell-panel-w) max-w-[calc(100vw-2*var(--space-4))] rounded-2xl border border-border-strong bg-surface',
          'shadow-popover outline-none data-entering:animate-enter',
        )}
      >
        <Dialog aria-label={t('notifications.bell.label')} className="flex flex-col outline-none">
          <div className="flex items-center justify-between gap-3 px-4 pt-3.5 pb-2.5">
            <Heading slot="title" className="text-md font-extrabold">
              {t('notifications.bell.label')}
            </Heading>
            <AriaButton
              isDisabled={unread === 0 || markAll.isPending}
              onPress={() => markAll.mutate()}
              className={markAllText}
            >
              {t('notifications.bell.markAllRead')}
            </AriaButton>
          </div>
          <FilterPills
            label={t('notifications.bell.show')}
            options={BELL_FILTERS.map((f) => ({
              id: f,
              label: f === 'unread' ? t('notifications.bell.unread', { count: unread }) : t('notifications.bell.all'),
            }))}
            value={filter}
            onChange={setFilter}
            className="border-b border-border-subtle px-4 pb-3"
          />
          <BellList
            audience={audience}
            items={list.data?.items}
            isLoading={list.isPending}
            isError={list.isError}
            onOpen={open}
          />
          <div className="flex items-center justify-between gap-3 border-t border-border-subtle px-4">
            <Link to={page} onClick={() => setOpen(false)} className={footerLink}>
              {t('notifications.bell.seeAll')}
            </Link>
            <Link to={page} hash={NOTIFICATION_SETTINGS_HASH} onClick={() => setOpen(false)} className={footerLink}>
              {t('notifications.bell.settings')}
            </Link>
          </div>
        </Dialog>
      </Popover>
    </DialogTrigger>
  )
}

type BellListProps = {
  audience: Audience
  items?: Notification[]
  isLoading: boolean
  isError: boolean
  onOpen: (n: Notification) => void
}

function BellList({ audience, items, isLoading, isError, onOpen }: BellListProps) {
  const { t } = useTranslation()
  const message = 'px-4 py-7 text-center text-sm text-muted'
  if (isLoading) {
    return (
      <p role="status" className={cx(message, 'flex items-center justify-center gap-2')}>
        <Spinner />
        {t('notifications.bell.loading')}
      </p>
    )
  }
  if (isError) return <p className={message}>{t('notifications.bell.error')}</p>
  if (!items?.length) return <p className={message}>{t('notifications.bell.empty')}</p>
  return (
    <ul className="flex max-h-(--bell-list-max-h) flex-col gap-1 overflow-y-auto p-1.5">
      {items.map((n) => (
        <NotificationItem key={n.id} notification={n} audience={audience} variant="bell" onOpen={onOpen} />
      ))}
    </ul>
  )
}
