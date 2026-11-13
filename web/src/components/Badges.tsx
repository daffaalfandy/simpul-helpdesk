import type { ReactNode } from 'react'

import type { Priority } from '../api/client'
import { cx } from '../lib/text'

/** Background and text colours for a priority (HANDOFF §14.2 contrast rules). */
export const priorityClasses: Record<Priority, string> = {
  P1: 'bg-priority-p1-bg text-priority-p1-fg',
  P2: 'bg-priority-p2-bg text-priority-p2-fg',
  P3: 'bg-priority-p3-bg text-priority-p3-fg',
  P4: 'bg-priority-p4-bg text-priority-p4-fg',
}

const badgeSizes = {
  sm: 'rounded-xs px-2 py-0.5 text-xs',
  /** As tall as a 44px field, for rows that pair the badge with inputs (Settings › Priorities). */
  lg: 'rounded-md px-3 py-2.5 text-sm',
}

/** Priority badge per HANDOFF §14.3. Pass a long label ("P1 · Critical") as children. */
export function PriorityBadge({
  priority,
  size = 'sm',
  children,
}: {
  priority: Priority
  size?: keyof typeof badgeSizes
  children?: ReactNode
}) {
  return (
    <span className={cx('inline-flex font-extrabold whitespace-nowrap', badgeSizes[size], priorityClasses[priority])}>
      {children ?? priority}
    </span>
  )
}

export type PillTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'paused' | 'code'

const pillClasses: Record<PillTone, string> = {
  neutral: 'bg-tint text-ink-2',
  code: 'bg-chip-code-bg text-chip-code-fg',
  info: 'bg-chip-info-bg text-chip-info-fg',
  success: 'bg-chip-success-bg text-chip-success-fg',
  warning: 'bg-chip-warning-bg text-chip-warning-fg',
  danger: 'bg-chip-danger-bg text-chip-danger-fg',
  paused: 'bg-chip-paused-bg text-chip-paused-fg',
}

/** Status pill: text always explains the state. md is the larger pill in page headers. */
export function Pill({ tone = 'info', size = 'sm', children }: { tone?: PillTone; size?: 'sm' | 'md'; children: ReactNode }) {
  return (
    <span
      className={cx(
        'inline-flex rounded-full font-bold whitespace-nowrap',
        size === 'md' ? 'px-3 py-1 text-sm' : 'px-2.5 py-0.5 text-xs',
        pillClasses[tone],
      )}
    >
      {children}
    </span>
  )
}
