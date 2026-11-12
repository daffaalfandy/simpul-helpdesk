import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { Me, Organization } from '../../api/client'
import { OrgLogo } from '../../components/Logo'
import { paths } from '../../lib/constants'
import { UserMenu } from './UserMenu'

/** Client portal header: branded for the organisation, no nav pills (HANDOFF §14.3). */
export function PortalHeader({ me, org }: { me: Me; org?: Organization }) {
  const { t } = useTranslation()
  return (
    <header className="border-b border-border bg-surface">
      <div className="mx-auto flex max-w-portal flex-wrap items-center justify-between gap-4 px-7 py-3.5">
        <Link to={paths.portal} className="flex items-center gap-2.5 text-ink no-underline hover:text-ink">
          <OrgLogo org={org} />
          <span className="flex flex-col">
            <span className="text-base font-extrabold">{t('portal.support', { org: org?.name ?? '' })}</span>
            {me.client && <span className="text-xs text-muted">{me.client.name}</span>}
          </span>
        </Link>
        <UserMenu me={me} />
      </div>
    </header>
  )
}
