import { CircleAlert, CircleCheck } from 'lucide-react'
import type { ReactNode } from 'react'

import { cx } from '../lib/text'

type AlertProps = { tone: 'error' | 'success'; children: ReactNode; className?: string }

/** Inline message. Errors are announced immediately (role=alert); success politely (role=status). */
export function Alert({ tone, children, className }: AlertProps) {
  const Icon = tone === 'error' ? CircleAlert : CircleCheck
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={cx(
        'flex items-start gap-2.5 rounded-lg border px-4 py-3 text-sm font-semibold',
        tone === 'error'
          ? 'border-danger bg-danger-tint text-ink'
          : 'border-success-border bg-success-tint text-success-ink',
        className,
      )}
    >
      <Icon aria-hidden="true" className={cx('mt-px size-(--icon-md) shrink-0', tone === 'error' && 'text-danger')} />
      <div>{children}</div>
    </div>
  )
}
