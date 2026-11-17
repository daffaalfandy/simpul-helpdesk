import { useSuspenseQuery } from '@tanstack/react-query'
import { MoreHorizontal, UserPlus } from 'lucide-react'
import { useState, type Key } from 'react'
import { Button as AriaButton, Menu, MenuItem, MenuTrigger, Popover } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { Member, Role, StaffRole } from '../../api/client'
import { useOrgTimeZone } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Pill, type PillTone } from '../../components/Badges'
import { Button } from '../../components/Button'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { formatDayMonthYear } from '../../lib/dates'
import { Roles } from '../../lib/roles'
import { useSignedInUser } from '../shell/useSignedInUser'
import { membersQueryOptions, useChangeMemberRole, useInviteMember, useMemberAction, type MemberAction } from './queries'
import { ErrorAlert, SectionHeader } from './shared'

/** The team's roles, as the invite dialog and each row offer them. */
const STAFF_ROLES: readonly StaffRole[] = [Roles.Admin, Roles.Supervisor, Roles.Agent]

/** The role cards (Settings mock), each with what that role can do. */
const ROLE_CARDS = [
  {
    role: Roles.Requester,
    can: [
      'settings.members.roles.requester.can.report',
      'settings.members.roles.requester.can.reply',
      'settings.members.roles.requester.can.reopen',
    ],
  },
  {
    role: Roles.Agent,
    can: ['settings.members.roles.agent.can.work', 'settings.members.roles.agent.can.reply', 'settings.members.roles.agent.can.override'],
  },
  {
    role: Roles.Supervisor,
    can: [
      'settings.members.roles.supervisor.can.agent',
      'settings.members.roles.supervisor.can.reassign',
      'settings.members.roles.supervisor.can.reports',
    ],
  },
  { role: Roles.Admin, can: ['settings.members.roles.admin.can.members', 'settings.members.roles.admin.can.settings'] },
] as const satisfies ReadonlyArray<{ role: Role; can: readonly string[] }>

const STATUS_TONES: Record<Member['status'], PillTone> = { active: 'success', invited: 'warning', suspended: 'neutral' }

/** Settings › Members & roles (Settings mock): role cards, then the team with what admins can change. */
export function MembersPage() {
  const { t } = useTranslation()
  const { data } = useSuspenseQuery(membersQueryOptions)
  const [inviting, setInviting] = useState(false)
  const [flash, setFlash] = useState<string | null>(null)
  const team = data.counts.admin + data.counts.supervisor + data.counts.agent

  return (
    <>
      <SectionHeader
        section="members"
        actions={
          <Button icon={<UserPlus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => setInviting(true)}>
            {t('settings.members.invite')}
          </Button>
        }
      />
      <p className="text-sm text-muted">{t('settings.members.summary', { team, requesters: data.counts.requester })}</p>
      {flash && <Alert tone="success">{flash}</Alert>}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(var(--role-card-min-w),1fr))] gap-3">
        {ROLE_CARDS.map(({ role, can }) => (
          <section key={role} aria-labelledby={`role-${role}`} className="flex flex-col gap-1.5 rounded-2xl border border-border bg-surface px-4.5 py-4">
            <div className="flex items-baseline justify-between gap-2">
              <h3 id={`role-${role}`} className="text-md font-extrabold">
                {t(`roles.${role}`)}
              </h3>
              <span className="text-sm text-muted">{t('settings.members.count', { count: data.counts[role] })}</span>
            </div>
            <p className="text-sm text-muted">{t(`settings.members.roles.${role}.who`)}</p>
            <ul className="list-disc pl-4.5 text-sm leading-relaxed text-ink-2">
              {can.map((c) => (
                <li key={c}>{t(c)}</li>
              ))}
            </ul>
          </section>
        ))}
      </div>
      <TeamTable team={data.team} onFlash={setFlash} />
      {inviting && (
        <InviteDialog
          onClose={() => setInviting(false)}
          onInvited={(email) => {
            setInviting(false)
            setFlash(t('settings.members.invited', { email }))
          }}
        />
      )}
    </>
  )
}

function TeamTable({ team, onFlash }: { team: Member[]; onFlash: (message: string | null) => void }) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const timeZone = useOrgTimeZone()
  const role = useChangeMemberRole()
  const action = useMemberAction()
  const [suspending, setSuspending] = useState<Member | null>(null)

  const act = (m: Member, a: MemberAction) => {
    onFlash(null)
    action.mutate(
      { userId: m.id, action: a },
      {
        onSuccess: () => {
          setSuspending(null)
          onFlash(t(`settings.members.done.${a}`, { name: m.name }))
        },
      },
    )
  }

  return (
    <section aria-labelledby="team-title" className="flex flex-col gap-3 rounded-3xl border border-border bg-surface p-5 shadow-card">
      <h3 id="team-title" className="text-lg font-extrabold tracking-title">
        {t('settings.members.teamTitle')}
      </h3>
      <ErrorAlert error={role.error ?? action.error} />
      <Table caption={t('settings.members.teamTitle')}>
        <thead>
          <tr>
            <Th>{t('settings.members.cols.name')}</Th>
            <Th>{t('settings.members.cols.role')}</Th>
            <Th>{t('settings.members.cols.status')}</Th>
            <Th>{t('settings.members.cols.lastSeen')}</Th>
            <Th>
              <span className="sr-only">{t('settings.members.cols.actions')}</span>
            </Th>
          </tr>
        </thead>
        <tbody>
          {team.map((m) => {
            const self = m.id === me.id
            return (
              <tr key={m.id}>
                <RowHeader>
                  <span className="flex flex-col">
                    <span>
                      {m.name}
                      {self && <span className="font-normal text-muted">{t('settings.members.you')}</span>}
                    </span>
                    <span className="text-sm font-normal text-muted">{m.email}</span>
                  </span>
                </RowHeader>
                <Td>
                  {self || m.role === Roles.Requester ? (
                    t(`roles.${m.role}`)
                  ) : (
                    <Dropdown<StaffRole>
                      label={t('settings.members.roleFor', { name: m.name })}
                      hideLabel
                      options={STAFF_ROLES.map((r) => ({ id: r, label: t(`roles.${r}`) }))}
                      value={m.role as StaffRole}
                      onChange={(r) => {
                        onFlash(null)
                        role.mutate({ userId: m.id, role: r }, { onSuccess: () => onFlash(t('settings.members.done.role', { name: m.name })) })
                      }}
                      className="w-(--member-role-w)"
                    />
                  )}
                </Td>
                <Td>
                  <Pill tone={STATUS_TONES[m.status]}>{t(`settings.members.status.${m.status}`)}</Pill>
                </Td>
                <Td className="text-sm text-ink-2">
                  {m.status === 'invited' && m.inviteSentAt
                    ? t('settings.members.inviteSent', { date: formatDayMonthYear(m.inviteSentAt, timeZone) })
                    : m.lastLoginAt
                      ? formatDayMonthYear(m.lastLoginAt, timeZone)
                      : t('settings.members.never')}
                </Td>
                <Td className="text-right">
                  {!self && <RowActions member={m} onAction={(a) => (a === 'suspend' ? setSuspending(m) : act(m, a))} />}
                </Td>
              </tr>
            )
          })}
        </tbody>
      </Table>
      {suspending && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (open) return
            setSuspending(null)
            action.reset()
          }}
          tone="danger"
          title={t('settings.members.suspendTitle', { name: suspending.name })}
          confirmLabel={t('settings.members.actions.suspend')}
          isPending={action.isPending}
          error={<ErrorAlert error={action.error} />}
          onConfirm={() => act(suspending, 'suspend')}
        >
          {t('settings.members.suspendBody')}
        </ConfirmDialog>
      )}
    </section>
  )
}

/** The row's menu: resend an invite, suspend, or let someone back in. */
export function RowActions({ member: m, onAction }: { member: Pick<Member, 'name' | 'status'>; onAction: (a: MemberAction) => void }) {
  const { t } = useTranslation()
  const actions: MemberAction[] = m.status === 'suspended' ? ['reactivate'] : m.status === 'invited' ? ['invite', 'suspend'] : ['suspend']
  return (
    <MenuTrigger>
      <AriaButton
        aria-label={t('settings.members.actionsFor', { name: m.name })}
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

function InviteDialog({ onClose, onInvited }: { onClose: () => void; onInvited: (email: string) => void }) {
  const { t } = useTranslation()
  const invite = useInviteMember()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<StaffRole>(Roles.Agent)
  const [jobTitle, setJobTitle] = useState('')
  const [tried, setTried] = useState(false)
  const missing = name.trim() === '' || email.trim() === ''

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.members.inviteTitle')}
      subtitle={t('settings.members.inviteSubtitle')}
      submitLabel={t('settings.members.inviteSubmit')}
      isPending={invite.isPending}
      error={<ErrorAlert error={invite.error} />}
      onSubmit={() => {
        setTried(true)
        if (missing) return
        const body = { name: name.trim(), email: email.trim(), role, jobTitle: jobTitle.trim() || undefined }
        invite.mutate(body, { onSuccess: () => onInvited(body.email) })
      }}
    >
      <TextInput label={t('settings.members.name')} value={name} onChange={setName} isRequired isInvalid={tried && name.trim() === ''} errorMessage={t('settings.members.nameRequired')} />
      <TextInput
        label={t('settings.members.email')}
        type="email"
        value={email}
        onChange={setEmail}
        isRequired
        isInvalid={tried && email.trim() === ''}
        errorMessage={t('settings.members.emailRequired')}
      />
      <Dropdown<StaffRole>
        label={t('settings.members.role')}
        options={STAFF_ROLES.map((r) => ({ id: r, label: t(`roles.${r}`), description: t(`settings.members.roles.${r}.who`) }))}
        value={role}
        onChange={setRole}
      />
      <TextInput label={t('settings.members.jobTitle')} value={jobTitle} onChange={setJobTitle} description={t('settings.members.jobTitleHint')} />
    </FormDialog>
  )
}
