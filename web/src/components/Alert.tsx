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
        'flex items-start gap-2.5 rounded-lg border px-4 py-3 animate-enter',
        tone === 'error'
          ? 'border-danger bg-danger-tint text-sm font-semibold text-ink'
          : 'border-success-border bg-success-tint text-base font-bold text-success-ink',
        className,
      )}
    >
      <Icon
        aria-hidden="true"
        className={cx('shrink-0', tone === 'error' ? 'mt-px size-(--icon-md) text-danger' : 'size-(--icon-lg)')}
      />
      <div>{children}</div>
    </div>
  )
}
