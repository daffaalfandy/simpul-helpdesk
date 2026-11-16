import { ExternalLink as ExternalIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { cx } from '../lib/text'

/** A link to GitHub, GitLab or another site: opens in a new tab and says so to screen readers. */
export function ExternalLink({ href, children, className }: { href: string; children: ReactNode; className?: string }) {
  const { t } = useTranslation()
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={cx('inline-flex items-baseline gap-1 hover:text-primary-hover hover:underline', className)}
    >
      {children}
      <ExternalIcon aria-hidden="true" className="size-(--icon-sm) shrink-0 self-center" />
      <span className="sr-only">{t('common.newTab')}</span>
    </a>
  )
}
