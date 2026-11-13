import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { Me, Organization } from '../../api/client'
import { BrandMark } from '../../components/Logo'
import { BRAND_NAME, paths } from '../../lib/constants'
import { can } from '../../lib/roles'
import { UserMenu } from './UserMenu'

const navLink =
  'rounded-md px-3.5 py-2.5 text-base font-semibold text-ink-2 no-underline hover:text-ink hover:bg-surface ' +
  'data-[status=active]:bg-ink data-[status=active]:font-bold data-[status=active]:text-on-primary'

/** Agent app top navigation per HANDOFF §14.3. Items depend on the user's role. */
export function AppHeader({ me, org }: { me: Me; org?: Organization }) {
  const { t } = useTranslation()
  const items = [
    { to: paths.board, label: t('nav.board'), show: true },
    { to: paths.projects, label: t('nav.projects'), show: can.manageProjects(me.role) },
    { to: paths.reports, label: t('nav.reports'), show: can.viewReports(me.role) },
    { to: paths.settings, label: t('nav.settings'), show: can.manageSettings(me.role) },
  ].filter((item) => item.show)

  return (
    <header className="border-b border-border bg-surface">
      <div className="mx-auto flex max-w-app flex-wrap items-center justify-between gap-4 px-4 py-3.5 sm:px-7">
        <Link to={paths.board} className="flex items-center gap-2.5 text-ink no-underline hover:text-ink">
          <BrandMark />
          <span className="flex flex-col">
            <span className="text-base font-extrabold">{BRAND_NAME}</span>
            <span className="text-xs text-muted">{t('agent.organization', { org: org?.name ?? '' })}</span>
          </span>
        </Link>
        <nav aria-label={t('nav.main')} className="flex flex-wrap gap-1 rounded-lg bg-tint p-1">
          {items.map((item) => (
            <Link key={item.to} to={item.to} className={navLink}>
              {item.label}
            </Link>
          ))}
        </nav>
        <UserMenu me={me} />
      </div>
    </header>
  )
}
