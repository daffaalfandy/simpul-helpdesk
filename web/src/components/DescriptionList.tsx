import type { ReactNode } from 'react'

export type DescriptionItem = { term: string; value: ReactNode }

/** Label/value pairs in two columns, one column on narrow screens. */
export function DescriptionList({ items }: { items: ReadonlyArray<DescriptionItem> }) {
  return (
    <dl className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
      {items.map((item) => (
        <div key={item.term} className="flex min-w-0 flex-col gap-0.5">
          <dt className="text-xs font-semibold text-muted">{item.term}</dt>
          <dd className="text-sm font-bold wrap-break-word text-ink">{item.value}</dd>
        </div>
      ))}
    </dl>
  )
}
