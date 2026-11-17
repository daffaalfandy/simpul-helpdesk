import { MoreHorizontal } from 'lucide-react'
import type { Key } from 'react'
import { Button as AriaButton, Menu, MenuItem, MenuTrigger, Popover } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { UserStatus } from '../../api/client'
import { Pill, type PillTone } from '../../components/Badges'
import type { MemberAction } from './queries'

const STATUS_TONES: Record<UserStatus, PillTone> = {
  active: 'success',
  invited: 'warning',
  suspended: 'neutral',
}

/** Whether someone can sign in: active, invited (no password yet) or suspended. */
export function MemberStatusPill({ status }: { status: UserStatus }) {
  const { t } = useTranslation()
  return <Pill tone={STATUS_TONES[status]}>{t(`settings.members.status.${status}`)}</Pill>
}

type MemberActionsProps = {
  name: string
  status: UserStatus
  /** A supervisor or agent can be taken off the organization; administrators work in every one. */
  removable?: boolean
  onAction: (a: MemberAction) => void
}

/**
 * A person's menu, for the team and for a client's requesters: resend an invite, suspend the
 * account or let them back in, and take a member off the organization.
 */
export function MemberActions({ name, status, removable = false, onAction }: MemberActionsProps) {
  const { t } = useTranslation()
  const account: MemberAction[] = status === 'suspended' ? ['reactivate'] : status === 'invited' ? ['invite', 'suspend'] : ['suspend']
  const actions: MemberAction[] = removable ? [...account, 'remove'] : account
  return (
    <MenuTrigger>
      <AriaButton
        aria-label={t('settings.members.actionsFor', { name })}
        className="hit-area inline-flex size-(--control-h-sm) cursor-pointer items-center justify-center rounded-md text-muted data-hovered:bg-surface-subtle data-hovered:text-ink"
      >
        <MoreHorizontal aria-hidden="true" className="size-(--icon-md)" />
      </AriaButton>
      <Popover placement="bottom end" className="min-w-48 rounded-xl border border-border bg-surface p-1.5 shadow-popover outline-none">
        <Menu onAction={(key: Key) => onAction(key as MemberAction)} className="outline-none">
          {actions.map((a) => (
            <MenuItem
              key={a}
              id={a}
              className="flex min-h-(--control-h-md) cursor-pointer items-center rounded-md px-3 text-base font-semibold text-ink outline-none data-focused:bg-primary-tint"
            >
              {t(`settings.members.actions.${a}`)}
            </MenuItem>
          ))}
        </Menu>
      </Popover>
    </MenuTrigger>
  )
}
