import type { ReactNode } from 'react'

import { cx } from '../lib/text'

type CardProps = {
  title?: ReactNode
  /** Heading level for the title; cards inside a page section use h2 or h3. */
  as?: 'h2' | 'h3'
  description?: ReactNode
  actions?: ReactNode
  children?: ReactNode
  className?: string
  labelledBy?: string
}

/** Card per HANDOFF §14.3: surface, border, radius 18, padding 18–22. */
export function Card({ title, as: Title = 'h2', description, actions, children, className, labelledBy }: CardProps) {
  return (
    <section
      aria-labelledby={labelledBy}
      className={cx('flex flex-col gap-4 rounded-3xl border border-border bg-surface p-5 shadow-card', className)}
    >
      {(title || actions) && (
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            {title && (
              <Title id={labelledBy} className="text-lg font-extrabold tracking-title">
                {title}
              </Title>
            )}
            {description && <p className="max-w-3xl text-sm text-muted">{description}</p>}
          </div>
          {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  )
}
