import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, useRouter } from '@tanstack/react-router'
import { Compass, Hammer, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { organizationQueryOptions } from '../../api/queries'
import { Button, buttonClasses } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { paths } from '../../lib/constants'
import { AppHeader } from './AppHeader'
import { Footer } from './Footer'
import { PortalHeader } from './PortalHeader'
import { useSignedInUser } from './useSignedInUser'


export function AgentLayout() {
  const me = useSignedInUser()
  const { data: org } = useQuery(organizationQueryOptions)
  return (
    <div className="flex min-h-screen flex-col">
      <AppHeader me={me} org={org} />
      <main className="mx-auto flex w-full max-w-app flex-1 flex-col gap-5 px-4 pt-6 pb-12 sm:px-7">
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
      <main className="mx-auto flex w-full max-w-portal flex-1 flex-col gap-6 px-4 pt-7 pb-14 sm:px-7">
        <Outlet />
      </main>
      <Footer variant="portal" org={org} />
    </div>
  )
}

function PageTitle({ children }: { children: React.ReactNode }) {
  return <h1 className="text-3xl leading-tight font-extrabold tracking-title">{children}</h1>
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

/** Shown when a route's loader or component fails unexpectedly. */
export function RouteError() {
  const { t } = useTranslation()
  const router = useRouter()
  return (
    <main className="mx-auto flex max-w-auth flex-col justify-center gap-4 px-5 py-16">
      <EmptyState icon={TriangleAlert} title={t('common.errorTitle')}>
        {t('common.errorGeneric')}
      </EmptyState>
      <Button variant="outline" className="self-center" onPress={() => router.invalidate()}>
        {t('common.retry')}
      </Button>
    </main>
  )
}
