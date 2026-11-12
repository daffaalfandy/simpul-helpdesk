import { useNavigate } from '@tanstack/react-router'
import { ChevronDown, LogOut } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button, Menu, MenuItem, MenuTrigger, Popover } from 'react-aria-components'

import type { Me } from '../../api/client'
import { useLogout } from '../../api/queries'
import { Avatar } from '../../components/Avatar'
import { paths } from '../../lib/constants'

const SIGN_OUT_ACTION = 'sign-out'

/** User chip with an account menu (sign out). */
export function UserMenu({ me }: { me: Me }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const logout = useLogout()
  const roleLine = me.jobTitle
    ? t('roleWithTitle', { role: t(`roles.${me.role}`), title: me.jobTitle })
    : t(`roles.${me.role}`)

  const onAction = (key: React.Key) => {
    if (key !== SIGN_OUT_ACTION) return
    logout.mutate(undefined, { onSettled: () => void navigate({ to: paths.login }) })
  }

  return (
    <MenuTrigger>
      <Button
        aria-label={t('nav.account', { name: me.name })}
        className="flex min-h-(--control-h-md) cursor-pointer items-center gap-2.5 rounded-lg px-2 text-left data-hovered:bg-surface-subtle"
      >
        <Avatar name={me.name} seed={me.id} />
        <span className="flex flex-col text-sm">
          <span className="font-bold text-ink">{me.name}</span>
          <span className="text-muted">{roleLine}</span>
        </span>
        <ChevronDown aria-hidden="true" className="size-(--icon-md) text-muted" />
      </Button>
      <Popover
        placement="bottom end"
        className="min-w-48 rounded-xl border border-border bg-surface p-1.5 shadow-popover outline-none"
      >
        <Menu onAction={onAction} className="outline-none">
          <MenuItem
            id={SIGN_OUT_ACTION}
            className="flex min-h-(--control-h-md) cursor-pointer items-center gap-2.5 rounded-md px-3 text-base font-semibold text-ink outline-none data-focused:bg-primary-tint"
          >
            <LogOut aria-hidden="true" className="size-(--icon-md)" />
            {t('common.signOut')}
          </MenuItem>
        </Menu>
      </Popover>
    </MenuTrigger>
  )
}
