import type { Organization } from '../api/client'
import { BRAND_MONOGRAM } from '../lib/constants'
import { cx, initials } from '../lib/text'

/** The SimDesk monogram: the footer, and the sign-in, setup and picker pages. */
export function BrandMark({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  return (
    <span
      aria-hidden="true"
      className={cx(
        'inline-flex shrink-0 items-center justify-center bg-ink font-extrabold text-on-primary',
        size === 'sm' && 'size-6 rounded-sm text-2xs',
        size === 'md' && 'size-9 rounded-md text-base',
        size === 'lg' && 'size-16 rounded-2xl text-lg',
      )}
    >
      {BRAND_MONOGRAM}
    </span>
  )
}

/** An organization's logo, or its initials when no logo is set. */
export function OrgLogo({ org, size = 'md' }: { org?: Pick<Organization, 'name' | 'logoUrl'>; size?: 'sm' | 'md' | 'lg' }) {
  const box = cx(
    size === 'sm' && 'size-6 rounded-sm text-2xs',
    size === 'md' && 'size-9 rounded-md text-base',
    size === 'lg' && 'size-16 rounded-2xl text-lg',
  )
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
