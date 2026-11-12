import type { Organization } from '../api/client'
import { BRAND_MONOGRAM } from '../lib/constants'
import { cx, initials } from '../lib/text'

/** The SimDesk monogram used in the agent app header and footer. */
export function BrandMark({ size = 'md' }: { size?: 'sm' | 'md' }) {
  return (
    <span
      aria-hidden="true"
      className={cx(
        'inline-flex shrink-0 items-center justify-center bg-ink font-extrabold text-on-primary',
        size === 'sm' ? 'size-6 rounded-sm text-2xs' : 'size-9 rounded-md text-base',
      )}
    >
      {BRAND_MONOGRAM}
    </span>
  )
}

/** The organisation's logo, or its initials when no logo is set. */
export function OrgLogo({ org, size = 'md' }: { org?: Organization; size?: 'md' | 'lg' }) {
  const box = size === 'lg' ? 'size-16 rounded-2xl text-lg' : 'size-9 rounded-md text-base'
  if (org?.logoUrl) {
    return <img src={org.logoUrl} alt="" className={cx('shrink-0 object-contain', box)} />
  }
  return (
    <span
      aria-hidden="true"
      className={cx('inline-flex shrink-0 items-center justify-center bg-ink font-extrabold text-on-primary', box)}
    >
      {initials(org?.name ?? '')}
    </span>
  )
}
