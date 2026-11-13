import { Link } from '@tanstack/react-router'
import { SearchX } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { buttonClasses } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { paths } from '../../lib/constants'

/** In-shell not-found for a ticket key that doesn't exist or isn't visible to the user. */
export function TicketNotFound({ audience }: { audience: 'agent' | 'requester' }) {
  const { t } = useTranslation()
  return (
    <>
      <EmptyState icon={SearchX} title={t('ticket.notFound.title')}>
        {t('ticket.notFound.body')}
      </EmptyState>
      <Link
        to={audience === 'agent' ? paths.board : paths.portal}
        className={buttonClasses('outline', 'md', 'self-center')}
      >
        {audience === 'agent' ? t('ticket.notFound.backBoard') : t('ticket.notFound.backPortal')}
      </Link>
    </>
  )
}
