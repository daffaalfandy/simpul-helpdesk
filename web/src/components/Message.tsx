import { Lock } from 'lucide-react'
import type { ReactNode } from 'react'

import { cx } from '../lib/text'

export type MessageVariant = 'public' | 'support' | 'internal'

const variants: Record<MessageVariant, string> = {
  public: 'border-border bg-surface',
  support: 'border-border bg-surface-subtle',
  internal: 'border-note-internal-border bg-note-internal-bg',
}

type MessageProps = {
  variant: MessageVariant
  /** Author line, already translated; the author's name is wrapped in <strong> ("Rina Pratiwi · replied to client"). */
  header: ReactNode
  time: ReactNode
  children: ReactNode
  /** Attachment list under the body. */
  footer?: ReactNode
}

/** Conversation entry per HANDOFF §14.3. Internal notes are yellow and say so in text. */
export function Message({ variant, header, time, children, footer }: MessageProps) {
  return (
    <article className={cx('flex flex-col gap-2 rounded-xl border px-4.5 py-4', variants[variant])}>
      <header
        className={cx(
          'flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm [&_strong]:font-bold',
          variant === 'internal' ? 'text-note-internal-fg' : 'text-muted [&_strong]:text-ink',
        )}
      >
        <p className="flex items-center gap-1.5">
          {variant === 'internal' && <Lock aria-hidden="true" className="size-(--icon-sm) shrink-0" />}
          <span>{header}</span>
        </p>
        <span>{time}</span>
      </header>
      <div className="text-md leading-relaxed wrap-break-word whitespace-pre-line text-ink">{children}</div>
      {footer}
    </article>
  )
}
