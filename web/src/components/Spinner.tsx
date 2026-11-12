import { LoaderCircle } from 'lucide-react'

import { cx } from '../lib/text'

export function Spinner({ className }: { className?: string }) {
  return <LoaderCircle aria-hidden="true" className={cx('size-(--icon-md) animate-spin', className)} />
}
