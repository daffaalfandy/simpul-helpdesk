import { Link, useLocation } from '@tanstack/react-router'
import { ChevronsLeft, ChevronsRight, Minus, Plus } from 'lucide-react'
import { useState } from 'react'
import { Button } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { cx } from '../../lib/text'
import { GROUPS, groupOfPath, type GroupId } from './sections'

const iconButton =
  'inline-flex size-(--control-h-md) shrink-0 cursor-pointer items-center justify-center rounded-md text-muted ' +
  'data-hovered:bg-surface-subtle data-hovered:text-ink'

/** Small outline controls in the sidebar header (Settings mock): 36px to see, 44px to tap. */
const outlineSmall =
  'hit-area inline-flex min-h-(--control-h-sm) cursor-pointer items-center justify-center rounded-sm border ' +
  'border-border-strong bg-surface text-ink-2 data-hovered:bg-surface-subtle'

const itemLink =
  'relative flex min-h-(--control-h-md) items-center gap-2.5 rounded-md px-3 text-base font-semibold text-ink-2 ' +
  'no-underline hover:bg-surface-subtle hover:text-ink ' +
  'data-[status=active]:bg-primary-tint data-[status=active]:font-extrabold data-[status=active]:text-primary-hover ' +
  "before:absolute before:inset-y-2 before:left-0 before:w-(--nav-indicator-w) before:rounded-xs before:content-[''] " +
  'data-[status=active]:before:bg-primary'

/**
 * Settings sidebar per HANDOFF §14.3: collapsible groups, "Collapse all", and an icon rail.
 * Choosing an item collapses the other groups.
 */
export function SettingsNav() {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const [rail, setRail] = useState(false)
  const [open, setOpen] = useState<ReadonlySet<GroupId>>(() => {
    const current = groupOfPath(pathname)
    return new Set(current ? [current] : [])
  })
  const anyOpen = open.size > 0
  const toggle = (id: GroupId) =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  if (rail) {
    return (
      <nav
        aria-label={t('settings.nav.label')}
        className="flex shrink-0 flex-row flex-wrap items-center gap-1 rounded-3xl border border-border bg-surface p-3 shadow-card lg:w-(--sidebar-rail-w) lg:flex-col"
      >
        <Button aria-label={t('settings.nav.expandMenu')} onPress={() => setRail(false)} className={iconButton}>
          <ChevronsRight aria-hidden="true" className="size-(--icon-lg)" />
        </Button>
        <span aria-hidden="true" className="mx-1 h-6 w-px bg-border lg:mx-0 lg:my-1 lg:h-px lg:w-7" />
        {GROUPS.flatMap((g) => g.sections).map((s) => {
          const label = t(`settings.sections.${s.id}.title`)
          return (
            <Link
              key={s.id}
              to={s.path}
              aria-label={label}
              title={label}
              className={cx(
                iconButton,
                'no-underline data-[status=active]:bg-primary-tint data-[status=active]:text-primary',
                'data-[status=active]:shadow-[inset_var(--nav-indicator-w)_0_0_var(--color-primary)]',
              )}
            >
              <s.icon aria-hidden="true" className="size-(--icon-lg)" />
            </Link>
          )
        })}
      </nav>
    )
  }

  return (
    <nav
      aria-label={t('settings.nav.label')}
      className="flex w-full shrink-0 flex-col gap-5 rounded-3xl border border-border bg-surface px-3 py-4.5 shadow-card lg:w-(--sidebar-w)"
    >
      <div className="flex items-center justify-between gap-2">
        <h1 className="text-lg font-extrabold">{t('settings.title')}</h1>
        <div className="flex items-center gap-1.5">
          <Button
            onPress={() => setOpen(anyOpen ? new Set() : new Set(GROUPS.map((g) => g.id)))}
            className={cx(outlineSmall, 'px-2.5 text-xs font-bold whitespace-nowrap')}
          >
            {anyOpen ? t('settings.nav.collapseAll') : t('settings.nav.expandAll')}
          </Button>
          <Button
            aria-label={t('settings.nav.collapseMenu')}
            onPress={() => setRail(true)}
            className={cx(outlineSmall, 'size-(--control-h-sm)')}
          >
            <ChevronsLeft aria-hidden="true" className="size-(--icon-md)" />
          </Button>
        </div>
      </div>
      {GROUPS.map((g) => {
        const isOpen = open.has(g.id)
        const label = t(`settings.nav.groups.${g.id}`)
        const listId = `settings-group-${g.id}`
        return (
          <div key={g.id} className="flex flex-col gap-1">
            <Button
              onPress={() => toggle(g.id)}
              aria-expanded={isOpen}
              aria-controls={listId}
              aria-label={t(isOpen ? 'settings.nav.collapseGroup' : 'settings.nav.expandGroup', { group: label })}
              className="hit-area flex min-h-(--control-h-sm) cursor-pointer items-center justify-between gap-2 rounded-md px-2 text-left data-hovered:bg-surface-subtle"
            >
              <span className="text-2xs font-extrabold tracking-caps text-muted uppercase">{label}</span>
              <span className="flex items-center gap-1.5">
                {!isOpen && (
                  <span
                    aria-hidden="true"
                    className="rounded-full bg-tint px-1.5 text-2xs font-bold text-ink-2"
                    title={t('settings.nav.itemCount', { count: g.sections.length })}
                  >
                    {g.sections.length}
                  </span>
                )}
                <span aria-hidden="true" className="inline-flex size-5.5 items-center justify-center rounded-xs border border-border-strong bg-surface text-ink-2">
                  {isOpen ? <Minus className="size-3" /> : <Plus className="size-3" />}
                </span>
              </span>
            </Button>
            <ul id={listId} hidden={!isOpen} className="flex animate-fade-in flex-col gap-0.5">
              {g.sections.map((s) => (
                <li key={s.id}>
                  <Link to={s.path} onClick={() => setOpen(new Set([g.id]))} className={itemLink}>
                    {t(`settings.sections.${s.id}.title`)}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        )
      })}
    </nav>
  )
}
