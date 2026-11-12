import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link, Outlet } from '@tanstack/react-router'
import { Compass, Hammer } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { meQueryOptions, organizationQueryOptions } from '../../api/queries'
import { buttonClasses } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { paths } from '../../lib/constants'
import { dayPart, firstName } from '../../lib/text'
import { AppHeader } from './AppHeader'
import { Footer } from './Footer'
import { PortalHeader } from './PortalHeader'

function useSignedInUser() {
  const { data: me } = useSuspenseQuery(meQueryOptions)
  if (!me) throw new Error('route guard should have redirected anonymous users')
  return me
}

export function AgentLayout() {
  const me = useSignedInUser()
  const { data: org } = useQuery(organizationQueryOptions)
  return (
    <div className="flex min-h-screen flex-col">
      <AppHeader me={me} org={org} />
      <main className="mx-auto flex w-full max-w-app flex-1 flex-col gap-5 px-7 pt-6 pb-12">
        <Outlet />
      </main>
      <Footer variant="internal" org={org} />
    </div>
  )
}

export function PortalLayout() {
  const me = useSignedInUser()
  const { data: org } = useQuery(organizationQueryOptions)
  return (
    <div className="flex min-h-screen flex-col">
      <PortalHeader me={me} org={org} />
      <main className="mx-auto flex w-full max-w-portal flex-1 flex-col gap-6 px-7 pt-7 pb-14">
        <Outlet />
      </main>
      <Footer variant="portal" org={org} />
    </div>
  )
}

function PageTitle({ children }: { children: React.ReactNode }) {
  return <h1 className="text-3xl leading-tight font-extrabold tracking-title">{children}</h1>
}

export function BoardPage() {
  const { t } = useTranslation()
  const me = useSignedInUser()
  return (
    <>
      <PageTitle>{t(`agent.greeting.${dayPart(new Date())}`, { name: firstName(me.name) })}</PageTitle>
      <EmptyState title={t('agent.boardEmptyTitle')}>{t('agent.boardEmpty')}</EmptyState>
    </>
  )
}

export function PortalHomePage() {
  const { t } = useTranslation()
  const me = useSignedInUser()
  return (
    <>
      <PageTitle>{t('portal.hello', { name: firstName(me.name) })}</PageTitle>
      <EmptyState title={t('portal.emptyTitle')}>{t('portal.empty')}</EmptyState>
    </>
  )
}

export function ComingSoonPage({ title }: { title: string }) {
  const { t } = useTranslation()
  return (
    <>
      <PageTitle>{title}</PageTitle>
      <EmptyState icon={Hammer} title={t('common.comingSoonTitle')}>
        {t('common.comingSoon')}
      </EmptyState>
    </>
  )
}

export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <main className="mx-auto flex min-h-screen max-w-auth flex-col justify-center gap-4 px-5">
      <EmptyState icon={Compass} title={t('notFound.title')}>
        {t('notFound.body')}
      </EmptyState>
      <Link to={paths.home} className={buttonClasses('outline', 'md', 'self-center')}>
        {t('notFound.home')}
      </Link>
    </main>
  )
}
