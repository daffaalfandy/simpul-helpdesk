import { Link } from '@tanstack/react-router'
import { CheckCheck, Mail } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button as AriaButton } from 'react-aria-components'

import type { Audience, Notification } from '../../api/client'
import { paths } from '../../lib/constants'
import { cx } from '../../lib/text'
import { noticeIcon, noticeQuote, noticeText } from './notice'
import { useNoticeTime } from './useNoticeTime'

type NotificationItemProps = {
  notification: Notification
  audience: Audience
  /** "bell": compact, one-line quote, how long ago. "inbox": full quote, time and channels, read toggle. */
  variant: 'bell' | 'inbox'
  onOpen: (n: Notification) => void
  onToggleRead?: (n: Notification) => void
}

/** One notification: what kind of change, on which ticket, what changed, and whether it's read. */
export function NotificationItem({ notification: n, audience, variant, onOpen, onToggleRead }: NotificationItemProps) {
  const { t } = useTranslation()
  const time = useNoticeTime()
  const unread = n.readAt == null
  const inbox = variant === 'inbox'
  const { Icon, tone } = noticeIcon(n)
  const text = noticeText(t, n, audience, time.clock)
  const quote = noticeQuote(n)
  const ticketLine = t('notifications.page.ticketLine', { key: n.ticket.key, subject: n.ticket.subject })
  const label = t(unread ? 'notifications.page.itemLabelUnread' : 'notifications.page.itemLabel', {
    ticket: ticketLine,
    text,
  })
  const channels = t(n.channels.email ? 'notifications.page.channels.webEmail' : 'notifications.page.channels.web')
  const to = audience === 'team' ? paths.ticket : paths.portalTicket

  return (
    <li className={cx('flex items-stretch gap-1 rounded-lg', unread && 'bg-bg')}>
      <Link
        to={to}
        params={{ key: n.ticket.key }}
        aria-label={label}
        onClick={() => onOpen(n)}
        className={cx(
          'flex min-w-0 flex-1 items-start gap-3 rounded-lg text-ink no-underline hover:bg-surface-subtle hover:text-ink',
          inbox ? 'p-3' : 'p-2.5',
        )}
      >
        <span
          aria-hidden="true"
          className={cx(
            'inline-flex shrink-0 items-center justify-center rounded-full',
            inbox ? 'size-(--notice-icon)' : 'size-(--notice-icon-sm)',
            tone,
          )}
        >
          <Icon className="size-(--icon-md)" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-xs font-bold text-muted">{ticketLine}</span>
          <span className={cx('leading-snug', inbox ? 'text-base' : 'text-sm', unread ? 'font-bold' : 'font-medium')}>
            {text}
          </span>
          {quote && (
            <span className={cx('text-muted', inbox ? 'text-sm' : 'truncate text-xs')}>
              {t('notifications.text.quote', { text: quote })}
            </span>
          )}
          <span className="text-xs text-muted">
            {inbox ? t('notifications.page.meta', { time: time.at(n.createdAt), channels }) : time.ago(n.createdAt)}
          </span>
        </span>
        {unread && (
          <span
            role="img"
            aria-label={t('notifications.page.unread')}
            className="mt-1.5 size-(--unread-dot) shrink-0 rounded-full bg-primary"
          />
        )}
      </Link>
      {inbox && onToggleRead && (
        <AriaButton
          aria-label={t(unread ? 'notifications.page.markRead' : 'notifications.page.markUnread')}
          onPress={() => onToggleRead(n)}
          className={cx(
            'hit-area mr-2.5 inline-flex size-(--control-h-sm) shrink-0 cursor-pointer items-center justify-center self-center',
            'rounded-md border border-border bg-surface text-ink-2 hover:bg-surface-subtle data-pressed:bg-tint',
          )}
        >
          {unread ? (
            <CheckCheck aria-hidden="true" className="size-(--icon-md)" />
          ) : (
            <Mail aria-hidden="true" className="size-(--icon-md)" />
          )}
        </AriaButton>
      )}
    </li>
  )
}
