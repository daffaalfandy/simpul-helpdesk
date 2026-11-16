import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useLocation } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button as AriaButton } from 'react-aria-components'

import type { Audience, EmailFrequency, Notification, NotificationEventSetting, NotificationFilter } from '../../api/client'
import { errorMessage } from '../../api/errors'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox } from '../../components/Checkbox'
import { Dropdown } from '../../components/Dropdown'
import { FilterPills } from '../../components/FilterPills'
import { Spinner } from '../../components/Spinner'
import { Switch } from '../../components/Switch'
import { NOTIFICATION_SETTINGS_HASH } from '../../lib/constants'
import { NotificationItem } from './NotificationItem'
import { DEFAULT_FILTER, filtersFor } from './notice'
import {
  notificationSettingsQueryOptions,
  notificationsQueryOptions,
  unreadCountQueryOptions,
  useMarkAllRead,
  useMarkRead,
  useSetEmailFrequency,
  useSetEventPreference,
} from './queries'

const FREQUENCIES: readonly EmailFrequency[] = ['instant', 'hourly', 'daily']

/** The inbox's "Mark all as read": an outline button that greys out when nothing is unread. */
const markAllOutline =
  'hit-area inline-flex min-h-(--control-h-sm) cursor-pointer items-center rounded-md border border-border-strong bg-surface px-3 ' +
  'text-sm font-bold text-primary hover:bg-surface-subtle data-disabled:cursor-not-allowed data-disabled:border-border ' +
  'data-disabled:bg-surface-subtle data-disabled:text-disabled'

/** Notifications (agent app and portal, HANDOFF §4): the inbox beside the settings. */
export function NotificationsPage() {
  const { t } = useTranslation()
  const { data: settings } = useSuspenseQuery(notificationSettingsQueryOptions)
  const audience = settings.audience
  return (
    <div className="flex flex-col gap-4.5">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-extrabold tracking-title">{t('notifications.page.title')}</h1>
        <p className="text-base text-muted">{t(`notifications.page.intro.${audience}`)}</p>
      </div>
      <div className="flex flex-wrap items-start gap-4.5">
        <Inbox audience={audience} />
        <Settings audience={audience} />
      </div>
    </div>
  )
}

function Inbox({ audience }: { audience: Audience }) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<NotificationFilter>(DEFAULT_FILTER)
  const list = useInfiniteQuery(notificationsQueryOptions(filter))
  const unread = useQuery(unreadCountQueryOptions).data?.count ?? 0
  const markRead = useMarkRead()
  const markAll = useMarkAllRead()
  const items = list.data?.pages.flatMap((p) => p.items) ?? []

  const open = (n: Notification) => {
    if (n.readAt == null) markRead.mutate({ id: n.id, read: true })
  }
  const toggle = (n: Notification) => markRead.mutate({ id: n.id, read: n.readAt == null })

  return (
    <Card
      title={t('notifications.page.inbox')}
      actions={
        <AriaButton isDisabled={unread === 0 || markAll.isPending} onPress={() => markAll.mutate()} className={markAllOutline}>
          {t('notifications.bell.markAllRead')}
        </AriaButton>
      }
      className="min-w-0 flex-[999_1_var(--notif-inbox-basis)]"
    >
      <FilterPills
        label={t('notifications.page.filterLabel')}
        options={filtersFor(audience).map((f) => ({
          id: f,
          label: t(`notifications.page.filters.${f}`, { count: unread }),
        }))}
        value={filter}
        onChange={setFilter}
      />
      {list.isPending ? (
        <p role="status" className="flex items-center gap-2 py-7 text-base text-muted">
          <Spinner />
          {t('notifications.page.loading')}
        </p>
      ) : list.isError ? (
        <Alert tone="error">{t('notifications.page.loadError')}</Alert>
      ) : items.length === 0 ? (
        <p className="py-7 text-base text-muted">{t('notifications.page.empty')}</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {items.map((n) => (
            <NotificationItem
              key={n.id}
              notification={n}
              audience={audience}
              variant="inbox"
              onOpen={open}
              onToggleRead={toggle}
            />
          ))}
        </ul>
      )}
      {list.hasNextPage && (
        <Button
          variant="outline"
          isPending={list.isFetchingNextPage}
          onPress={() => void list.fetchNextPage()}
          className="self-start"
        >
          {t('notifications.page.showOlder')}
        </Button>
      )}
    </Card>
  )
}

function Settings({ audience }: { audience: Audience }) {
  const { t } = useTranslation()
  const { data: settings } = useSuspenseQuery(notificationSettingsQueryOptions)
  const setPreference = useSetEventPreference()
  const setFrequency = useSetEmailFrequency()
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const ref = useRef<HTMLDivElement>(null)
  const { hash } = useLocation()

  // "Notification settings" in the bell opens the page at this card.
  useEffect(() => {
    if (hash !== NOTIFICATION_SETTINGS_HASH) return
    ref.current?.scrollIntoView?.({ block: 'start' }) // absent in some test DOMs
    ref.current?.focus({ preventScroll: true })
  }, [hash])

  const feedback = {
    onMutate: () => {
      setSaved(false)
      setError(null)
    },
    onSuccess: () => setSaved(true),
    onError: (err: unknown) => setError(err),
  }
  const toggle = (e: NotificationEventSetting, channel: 'web' | 'email', on: boolean) => {
    feedback.onMutate()
    setPreference.mutate({ event: e.event, web: e.web, email: e.email, [channel]: on }, feedback)
  }
  const chooseFrequency = (f: EmailFrequency) => {
    feedback.onMutate()
    setFrequency.mutate(f, feedback)
  }
  const label = (e: NotificationEventSetting) => t(`notifications.settings.events.${audience}.${e.event}.label` as never)
  // Not every event has a hint.
  const hint = (e: NotificationEventSetting) =>
    t(`notifications.settings.events.${audience}.${e.event}.hint` as never, { defaultValue: '' })

  return (
    <div
      ref={ref}
      id={NOTIFICATION_SETTINGS_HASH}
      tabIndex={-1}
      className="min-w-0 flex-[1_1_var(--notif-settings-basis)] rounded-3xl outline-none"
    >
      <Card title={t('notifications.settings.title')}>
        <p className="text-sm text-muted">{t(`notifications.settings.who.${audience}`)}</p>
        {saved && <Alert tone="success">{t('notifications.settings.saved')}</Alert>}
        {error != null && <Alert tone="error">{errorMessage(t, error)}</Alert>}
        <table className="w-full table-fixed border-collapse text-base">
          <caption className="sr-only">{t('notifications.settings.caption')}</caption>
          <thead>
            <tr className="text-left text-xs font-bold text-muted">
              <th scope="col" className="pb-2 font-bold">
                {t('notifications.settings.when')}
              </th>
              <th scope="col" className="w-16 pb-2 text-center font-bold">
                {t('notifications.settings.web')}
              </th>
              <th scope="col" className="w-16 pb-2 text-center font-bold">
                {t('notifications.settings.email')}
              </th>
            </tr>
          </thead>
          <tbody>
            {settings.events.map((e) => (
              <tr key={e.event} className="border-t border-border-subtle">
                <th scope="row" className="py-2.5 pr-3 text-left align-middle">
                  <span className="block font-bold">{label(e)}</span>
                  {hint(e) && <span className="block text-xs font-medium text-muted">{hint(e)}</span>}
                </th>
                <td className="text-center align-middle">
                  <Switch
                    aria-label={t('notifications.settings.webFor', { event: label(e) })}
                    isSelected={e.web}
                    isDisabled={e.locked}
                    onChange={(on) => toggle(e, 'web', on)}
                  />
                </td>
                <td className="text-center align-middle">
                  <Switch
                    aria-label={t('notifications.settings.emailFor', { event: label(e) })}
                    isSelected={e.email}
                    isDisabled={e.locked}
                    onChange={(on) => toggle(e, 'email', on)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="text-xs text-muted">{t(`notifications.settings.lockedNote.${audience}`)}</p>
        <div className="flex flex-col gap-3 border-t border-border-subtle pt-4">
          <Dropdown
            label={t('notifications.settings.sendEmails')}
            options={FREQUENCIES.map((f) => ({
              id: f,
              label: t(`notifications.settings.frequency.${f}`),
              description: t(`notifications.settings.frequencyHint.${f}`),
            }))}
            value={settings.emailFrequency}
            onChange={chooseFrequency}
          />
          <Checkbox isSelected isDisabled description={t('notifications.settings.alwaysOn')}>
            <span className="font-bold">{t('notifications.settings.ownChanges')}</span>
          </Checkbox>
        </div>
      </Card>
    </div>
  )
}
