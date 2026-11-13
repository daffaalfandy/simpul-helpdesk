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
  /** "compact" is the side-column card in the mock: radius 16, tighter padding, 14px title. */
  variant?: 'default' | 'compact'
}

const variants = {
  default: { box: 'gap-4 rounded-3xl p-5', title: 'text-lg tracking-title' },
  compact: { box: 'gap-3 rounded-2xl px-4.5 py-4', title: 'text-base' },
}

/** Card per HANDOFF §14.3: surface, border, radius 18, padding 18–22. */
export function Card({
  title,
  as: Title = 'h2',
  description,
  actions,
  children,
  className,
  labelledBy,
  variant = 'default',
}: CardProps) {
  const v = variants[variant]
  return (
    <section
      aria-labelledby={labelledBy}
      className={cx('flex flex-col border border-border bg-surface shadow-card', v.box, className)}
    >
      {(title || actions) && (
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            {title && (
              <Title id={labelledBy} className={cx('font-extrabold', v.title)}>
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
