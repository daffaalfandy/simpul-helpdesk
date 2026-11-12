import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import { organizationQueryOptions } from '../../api/queries'
import { OrgLogo } from '../../components/Logo'
import { Footer } from '../shell/Footer'

/** Card class shared by the sign-in and password forms (HANDOFF §14.3 card, raised). */
export const authCard = 'flex flex-col gap-3.5 rounded-dialog border border-border bg-surface p-6 shadow-raised'

/** Centered page for sign-in and password flows, branded with the organisation. */
export function AuthLayout({ title, intro, children }: { title: string; intro?: ReactNode; children: ReactNode }) {
  const { data: org } = useQuery(organizationQueryOptions)
  return (
    <div className="flex min-h-screen flex-col">
      <main className="flex flex-1 items-center justify-center px-5 py-12">
        <div className="flex w-full max-w-auth flex-col gap-5">
          <div className="flex flex-col items-center gap-3 text-center">
            <OrgLogo org={org} size="lg" />
            <h1 className="text-2xl leading-tight font-extrabold tracking-title">{title}</h1>
            {intro && <p className="text-base text-muted">{intro}</p>}
          </div>
          {children}
        </div>
      </main>
      <Footer variant="internal" org={org} />
    </div>
  )
}
