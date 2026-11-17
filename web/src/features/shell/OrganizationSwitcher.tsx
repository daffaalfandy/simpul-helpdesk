import { useNavigate } from '@tanstack/react-router'
import { Check, ChevronDown } from 'lucide-react'
import type { Key } from 'react'
import { Button as AriaButton, Menu, MenuItem, MenuTrigger, Popover, Separator } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { Me, Organization } from '../../api/client'
import { useSwitchOrganization } from '../../api/queries'
import { OrgLogo } from '../../components/Logo'
import { BRAND_NAME, paths } from '../../lib/constants'
import { homePathFor, picksOrganization } from '../../lib/roles'

/** The menu row that opens Settings › Organizations (administrators). */
const MANAGE_KEY = '__manage-organizations__'

/**
 * The app header's brand: the organization the tab works in, with its logo (owner, 17 Nov). Users
 * who work in more than one organization switch here; administrators also reach Settings ›
 * Organizations.
 */
export function OrganizationSwitcher({ me, org }: { me: Me; org?: Organization }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const switchTo = useSwitchOrganization()
  const name = org?.name ?? me.organization?.name ?? ''
  const brand = (
    <>
      <OrgLogo org={org ?? (me.organization ? { name, logoUrl: me.organization.logoUrl } : undefined)} />
      <span className="flex min-w-0 flex-col text-left">
        <span className="truncate text-md font-extrabold">{name}</span>
        <span className="text-xs text-muted">{BRAND_NAME}</span>
      </span>
    </>
  )
  if (!picksOrganization(me) && !me.isAdmin) {
    return <span className="flex min-w-0 items-center gap-2.5 text-ink">{brand}</span>
  }
  const onAction = (key: Key) => {
    if (key === MANAGE_KEY) {
      void navigate({ to: paths.settingsOrganizations })
      return
    }
    if (key === me.organization?.id) return
    switchTo.mutate(String(key), { onSuccess: (next) => void navigate({ to: homePathFor(next.role), replace: true }) })
  }
  return (
    <MenuTrigger>
      <AriaButton
        aria-label={t('organizations.switchLabel', { org: name })}
        className="flex min-w-0 cursor-pointer items-center gap-2.5 rounded-lg p-1 text-ink outline-none data-hovered:bg-surface-subtle data-focus-visible:ring-2 data-focus-visible:ring-primary"
      >
        {brand}
        <ChevronDown aria-hidden="true" className="size-(--icon-md) shrink-0 text-muted" />
      </AriaButton>
      <Popover placement="bottom start" className="min-w-64 rounded-xl border border-border bg-surface p-1.5 shadow-popover outline-none">
        <Menu onAction={onAction} className="outline-none">
          {me.organizations.map((o) => (
            <MenuItem
              key={o.id}
              id={o.id}
              textValue={o.name}
              aria-label={t('organizations.option', { org: o.name, role: t(`roles.${o.role}`) })}
              className="flex min-h-(--control-h-md) cursor-pointer items-center gap-2.5 rounded-md px-2.5 text-base font-semibold text-ink outline-none data-focused:bg-primary-tint"
            >
              <OrgLogo org={o} size="sm" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate">{o.name}</span>
                <span className="text-xs font-regular text-muted">{t(`roles.${o.role}`)}</span>
              </span>
              {o.id === me.organization?.id && <Check aria-label={t('organizations.current')} className="size-(--icon-md) text-primary" />}
            </MenuItem>
          ))}
          {me.isAdmin && (
            <>
              <Separator className="my-1 border-t border-border-subtle" />
              <MenuItem
                id={MANAGE_KEY}
                className="flex min-h-(--control-h-md) cursor-pointer items-center rounded-md px-2.5 text-base font-semibold text-primary outline-none data-focused:bg-primary-tint"
              >
                {t('organizations.manage')}
              </MenuItem>
            </>
          )}
        </Menu>
      </Popover>
    </MenuTrigger>
  )
}
