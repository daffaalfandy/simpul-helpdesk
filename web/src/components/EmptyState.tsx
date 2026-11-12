import { Inbox, type LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

type EmptyStateProps = { title: ReactNode; children: ReactNode; icon?: LucideIcon }

export function EmptyState({ title, children, icon: Icon = Inbox }: EmptyStateProps) {
  return (
    <section className="flex flex-col items-center gap-3 rounded-3xl border border-border bg-surface px-6 py-14 text-center shadow-card">
      <span className="inline-flex size-12 items-center justify-center rounded-full bg-tint text-muted">
        <Icon aria-hidden="true" className="size-(--icon-lg)" />
      </span>
      <h2 className="text-lg font-extrabold tracking-title">{title}</h2>
      <p className="max-w-md text-sm text-muted">{children}</p>
    </section>
  )
}
