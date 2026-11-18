import { Check, Minus } from 'lucide-react'
import type { ReactNode } from 'react'

import { cx } from '../lib/text'

export type TrackerState = 'done' | 'current' | 'upcoming' | 'skipped'

export type TrackerStep = { id: string; label: string; detail?: ReactNode; state: TrackerState }

// A skipped step is filled (the ticket went past it) but grey, so it never reads as done or as
// still to come.
const bars: Record<TrackerState, string> = {
  done: 'bg-success',
  current: 'bg-primary',
  upcoming: 'bg-tint',
  skipped: 'bg-disabled',
}

const dots: Record<TrackerState, string> = {
  done: 'bg-success text-on-primary',
  current: 'bg-primary text-on-primary',
  upcoming: 'bg-tint text-muted',
  skipped: 'bg-tint text-muted',
}

type ProgressTrackerProps = {
  label: string
  steps: ReadonlyArray<TrackerStep>
  /** Screen-reader text for finished steps ("Completed"). */
  doneLabel: string
  /** Screen-reader text for steps the ticket went past without stopping at ("Skipped"). */
  skippedLabel: string
}

/** Progress tracker per HANDOFF §14.3; the current step has aria-current="step". */
export function ProgressTracker({ label, steps, doneLabel, skippedLabel }: ProgressTrackerProps) {
  return (
    <ol aria-label={label} className="grid grid-cols-[repeat(auto-fit,minmax(var(--tracker-step-min-w),1fr))] gap-3">
      {steps.map((s, i) => (
        <li key={s.id} aria-current={s.state === 'current' ? 'step' : undefined} className="flex flex-col gap-2">
          <span aria-hidden="true" className={cx('h-(--tracker-bar-h) rounded-full', bars[s.state])} />
          <span className="flex items-center gap-2">
            <span
              aria-hidden="true"
              className={cx(
                'inline-flex size-(--tracker-dot) shrink-0 items-center justify-center rounded-full text-xs font-extrabold',
                dots[s.state],
              )}
            >
              {s.state === 'done' ? (
                <Check className="size-(--icon-sm)" />
              ) : s.state === 'skipped' ? (
                <Minus className="size-(--icon-sm)" />
              ) : (
                i + 1
              )}
            </span>
            <span className={cx('text-sm font-bold', s.state === 'upcoming' || s.state === 'skipped' ? 'text-muted' : 'text-ink')}>
              {s.label}
            </span>
            {s.state === 'done' && <span className="sr-only">{doneLabel}</span>}
            {s.state === 'skipped' && <span className="sr-only">{skippedLabel}</span>}
          </span>
          {s.detail && <span className="text-xs text-muted">{s.detail}</span>}
        </li>
      ))}
    </ol>
  )
}
