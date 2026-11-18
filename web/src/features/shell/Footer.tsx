import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { Organization } from '../../api/client'
import { ExternalLink } from '../../components/ExternalLink'
import { BrandMark } from '../../components/Logo'
import { BRAND_NAME, DOCS_URL, ISSUES_URL, SOURCE_URL } from '../../lib/constants'

type FooterProps = { variant: 'internal' | 'portal'; org?: Organization }

/**
 * Footer per HANDOFF §14.3 and the Footer mock. The internal footer links to the open-source
 * project; the portal is branded for the organisation. Links without a page yet (privacy, the
 * portal's help pages, the urgent-contact column) wait for their settings in M9.
 */
export function Footer({ variant, org }: FooterProps) {
  const { t } = useTranslation()
  const orgName = org?.name ?? ''
  const portal = variant === 'portal'
  const links = portal
    ? []
    : [
        { href: DOCS_URL, label: t('footer.docs') },
        { href: SOURCE_URL, label: t('footer.source') },
        { href: ISSUES_URL, label: t('footer.bug') },
      ]
  return (
    <footer className="border-t border-border bg-surface text-sm text-muted">
      <div className="flex flex-wrap items-start justify-between gap-x-8 gap-y-4 px-4 py-5.5 sm:px-7">
        <div className="flex flex-col gap-1.5">
          <span className="flex items-center gap-2 text-base font-extrabold text-ink">
            <BrandMark size="sm" />
            {portal ? t('portal.support', { org: orgName }) : BRAND_NAME}
          </span>
          <span>{portal ? t('footer.portalTagline', { org: orgName }) : org ? t('footer.tagline', { org: orgName }) : t('footer.productTagline')}</span>
        </div>
        {links.length > 0 && (
          <nav aria-label={t('footer.linksLabel')} className="flex flex-wrap gap-x-4.5 gap-y-1">
            {links.map((l) => (
              <ExternalLink key={l.href} href={l.href} className="hit-area min-h-(--control-h-sm) font-semibold text-muted underline">
                {l.label}
              </ExternalLink>
            ))}
          </nav>
        )}
      </div>
      <div className="border-t border-border-subtle">
        <div className="flex flex-wrap justify-between gap-x-6 gap-y-2 px-4 py-3 text-xs sm:px-7">
          <span>{portal ? t('footer.poweredBy') : t('footer.openSource')}</span>
          {org?.footerNote && (
            <span className="flex items-center gap-1.5">
              <ShieldCheck aria-hidden="true" className="size-(--icon-sm) shrink-0" />
              {org.footerNote}
            </span>
          )}
        </div>
      </div>
    </footer>
  )
}
