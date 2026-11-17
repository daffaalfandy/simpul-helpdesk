import { Link } from '@tanstack/react-router'
import { ArrowLeftRight, SearchX } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { OrganizationChoice } from '../../api/client'
import { useSwitchOrganization } from '../../api/queries'
import { Button, buttonClasses } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths } from '../../lib/constants'
import { useSignedInUser } from '../shell/useSignedInUser'

type TicketNotFoundProps = {
  audience: 'agent' | 'requester'
  /** Set when the ticket belongs to another organization the API says the user works in. */
  organizationId?: string
}

/** In-shell not-found for a ticket key that doesn't exist or isn't visible to the user. */
export function TicketNotFound({ audience, organizationId }: TicketNotFoundProps) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const elsewhere = organizationId ? me.organizations.find((o) => o.id === organizationId) : undefined
  if (elsewhere) return <InAnotherOrganization org={elsewhere} current={me.organization?.name ?? ''} />
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

/**
 * A link (often from an email) to a ticket of another organization the user works in: one press
 * moves this tab there and opens the ticket.
 */
function InAnotherOrganization({ org, current }: { org: OrganizationChoice; current: string }) {
  const { t } = useTranslation()
  const switchTo = useSwitchOrganization()
  return (
    <>
      <EmptyState icon={ArrowLeftRight} title={t('organizations.elsewhere.title', { org: org.name })}>
        {t('organizations.elsewhere.body', { org: org.name, current })}
      </EmptyState>
      <ErrorAlert error={switchTo.error} />
      <Button
        className="self-center"
        isPending={switchTo.isPending}
        onPress={() => switchTo.mutate(org.id)}
      >
        {t('organizations.elsewhere.switch', { org: org.name })}
      </Button>
    </>
  )
}
