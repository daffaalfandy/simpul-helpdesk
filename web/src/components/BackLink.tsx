import { Link, type LinkProps } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import type { ReactNode } from 'react'

/** "‹ Back to …" link at the top of a page. */
export function BackLink({ children, ...props }: LinkProps & { children: ReactNode }) {
  return (
    <Link
      {...props}
      className="inline-flex min-h-(--control-h-md) items-center gap-1 self-start font-bold text-primary hover:text-primary-hover"
    >
      <ChevronLeft aria-hidden="true" className="size-(--icon-md)" />
      {children}
    </Link>
  )
}
