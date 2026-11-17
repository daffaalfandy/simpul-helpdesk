import type { ReactNode } from 'react'

import { BrandMark } from '../../components/Logo'
import { Footer } from '../shell/Footer'

/** Card class shared by the sign-in and password forms (HANDOFF §14.3 card, raised). */
export const authCard = 'flex flex-col gap-3.5 rounded-dialog border border-border bg-surface p-6 shadow-raised'

/**
 * Centered page for sign-in, setup, password and organization-picker flows. It shows SimDesk:
 * an organization's branding starts once one is picked (owner, 17 Nov).
 */
export function AuthLayout({ title, intro, children }: { title: string; intro?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <main className="flex flex-1 items-center justify-center px-5 py-12">
        <div className="flex w-full max-w-auth flex-col gap-5">
          <div className="flex flex-col items-center gap-3 text-center">
            <BrandMark size="lg" />
            <h1 className="text-2xl leading-tight font-extrabold tracking-title">{title}</h1>
            {intro && <p className="text-base text-muted">{intro}</p>}
          </div>
          {children}
        </div>
      </main>
      <Footer variant="internal" />
    </div>
  )
}
