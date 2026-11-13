import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { Organization } from '../../api/client'
import { BrandMark } from '../../components/Logo'
import { BRAND_NAME } from '../../lib/constants'

type FooterProps = { variant: 'internal' | 'portal'; org?: Organization }

/** Footer per HANDOFF §14.3. The portal variant is branded for the organisation. */
export function Footer({ variant, org }: FooterProps) {
  const { t } = useTranslation()
  const orgName = org?.name ?? ''
  const portal = variant === 'portal'
  return (
    <footer className="border-t border-border bg-surface text-sm text-muted">
      <div className="mx-auto flex max-w-app flex-wrap items-start justify-between gap-x-8 gap-y-4 px-4 py-5 sm:px-7">
        <div className="flex flex-col gap-1.5">
          <span className="flex items-center gap-2 text-base font-extrabold text-ink">
            <BrandMark size="sm" />
            {portal ? t('portal.support', { org: orgName }) : BRAND_NAME}
          </span>
          <span>{portal ? t('footer.portalTagline', { org: orgName }) : t('footer.tagline', { org: orgName })}</span>
        </div>
      </div>
      <div className="border-t border-border-subtle">
        <div className="mx-auto flex max-w-app flex-wrap justify-between gap-x-6 gap-y-2 px-4 py-3 text-xs sm:px-7">
          <span>{portal ? t('footer.poweredBy') : t('footer.copyright', { year: new Date().getFullYear() })}</span>
          {org?.footerNote && (
            <span className="inline-flex items-center gap-1.5">
              <ShieldCheck aria-hidden="true" className="size-(--icon-sm)" />
              {org.footerNote}
            </span>
          )}
        </div>
      </div>
    </footer>
  )
}
