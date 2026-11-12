import type { ReactNode } from 'react'

import { cx } from '../lib/text'

/** A table that scrolls sideways in its own box on narrow screens (HANDOFF §14.1). */
export function Table({
  caption,
  minWidth = 'min-w-table',
  children,
}: {
  caption: string
  minWidth?: string
  children: ReactNode
}) {
  return (
    <div className="-mx-1 overflow-x-auto px-1">
      <table className={cx('w-full border-collapse text-left text-base', minWidth)}>
        <caption className="sr-only">{caption}</caption>
        {children}
      </table>
    </div>
  )
}

export function Th({ children, className }: { children?: ReactNode; className?: string }) {
  return <th scope="col" className={cx('px-3 py-2.5 text-xs font-bold text-muted', className)}>{children}</th>
}

export function RowHeader({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <th scope="row" className={cx('border-t border-border-subtle px-3 py-3 text-left align-top font-bold', className)}>
      {children}
    </th>
  )
}

export function Td({ children, className }: { children?: ReactNode; className?: string }) {
  return <td className={cx('border-t border-border-subtle px-3 py-3 align-top', className)}>{children}</td>
}
