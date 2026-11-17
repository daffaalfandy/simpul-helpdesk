import { useSuspenseQuery } from '@tanstack/react-query'
import { UserPlus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Member, Role, StaffRole } from '../../api/client'
import { useOrgTimeZone } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { formatDayMonthYear } from '../../lib/dates'
import { Roles } from '../../lib/roles'
import { useSignedInUser } from '../shell/useSignedInUser'
import { MemberActions, MemberStatusPill } from './MemberControls'
import { membersQueryOptions, useChangeMemberRole, useInviteMember, useMemberAction, type MemberAction } from './queries'
import { ErrorAlert, SectionHeader } from './shared'

/** The actions that ask before they run. */
type ConfirmedAction = Extract<MemberAction, 'suspend' | 'remove'>

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
  {
    role: Roles.Admin,
    can: ['settings.members.roles.admin.can.members', 'settings.members.roles.admin.can.settings'],
  },
] as const satisfies ReadonlyArray<{ role: Role; can: readonly string[] }>

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
      <p className="text-sm text-muted">
        {t('settings.members.summary', {
          team,
          requesters: data.counts.requester,
        })}
      </p>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(var(--role-card-min-w),1fr))] gap-3">
        {ROLE_CARDS.map(({ role, can }) => (
          <section
            key={role}
            aria-labelledby={`role-${role}`}
            className="flex flex-col gap-1.5 rounded-2xl border border-border bg-surface px-4.5 py-4"
          >
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
      <TeamTable team={data.team} flash={flash} onFlash={setFlash} />
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

/** The team, with what admins can change. Confirmations show here, beside the rows they are about. */
function TeamTable({ team, flash, onFlash }: { team: Member[]; flash: string | null; onFlash: (message: string | null) => void }) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const timeZone = useOrgTimeZone()
  const role = useChangeMemberRole()
  const action = useMemberAction()
  // Suspending and removing ask first.
  const [confirming, setConfirming] = useState<{ member: Member; action: ConfirmedAction } | null>(null)

  const act = (m: Member, a: MemberAction) => {
    onFlash(null)
    action.mutate(
      { userId: m.id, action: a },
      {
        onSuccess: () => {
          setConfirming(null)
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
      {flash && <Alert tone="success">{flash}</Alert>}
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
                      {self && <span className="font-regular text-muted">{t('settings.members.you')}</span>}
                    </span>
                    <span className="text-sm font-regular text-muted">{m.email}</span>
                  </span>
                </RowHeader>
                <Td>
                  {self || m.role === Roles.Requester ? (
                    t(`roles.${m.role}`)
                  ) : (
                    <Dropdown<StaffRole>
                      label={t('settings.members.roleFor', { name: m.name })}
                      hideLabel
                      options={STAFF_ROLES.map((r) => ({
                        id: r,
                        label: t(`roles.${r}`),
                      }))}
                      value={m.role as StaffRole}
                      onChange={(r) => {
                        onFlash(null)
                        role.mutate(
                          { userId: m.id, role: r },
                          {
                            onSuccess: () =>
                              onFlash(
                                t('settings.members.done.role', {
                                  name: m.name,
                                }),
                              ),
                          },
                        )
                      }}
                      className="w-(--member-role-w)"
                    />
                  )}
                </Td>
                <Td>
                  <MemberStatusPill status={m.status} />
                </Td>
                <Td className="text-sm text-ink-2">
                  {m.status === 'invited' && m.inviteSentAt
                    ? t('settings.members.inviteSent', {
                        date: formatDayMonthYear(m.inviteSentAt, timeZone),
                      })
                    : m.lastLoginAt
                      ? formatDayMonthYear(m.lastLoginAt, timeZone)
                      : t('settings.members.never')}
                </Td>
                <Td className="text-right">
                  {!self && (
                    <MemberActions
                      name={m.name}
                      status={m.status}
                      removable={m.role !== Roles.Admin}
                      onAction={(a) => (a === 'suspend' || a === 'remove' ? setConfirming({ member: m, action: a }) : act(m, a))}
                    />
                  )}
                </Td>
              </tr>
            )
          })}
        </tbody>
      </Table>
      {confirming && (
        <ConfirmDialog
          isOpen
          onOpenChange={(open) => {
            if (open) return
            setConfirming(null)
            action.reset()
          }}
          tone="danger"
          title={t(`settings.members.confirm.${confirming.action}.title`, { name: confirming.member.name })}
          confirmLabel={t(`settings.members.actions.${confirming.action}`)}
          isPending={action.isPending}
          error={<ErrorAlert error={action.error} />}
          onConfirm={() => act(confirming.member, confirming.action)}
        >
          {t(`settings.members.confirm.${confirming.action}.body`)}
        </ConfirmDialog>
      )}
    </section>
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
        const body = {
          name: name.trim(),
          email: email.trim(),
          role,
          jobTitle: jobTitle.trim() || undefined,
        }
        invite.mutate(body, { onSuccess: () => onInvited(body.email) })
      }}
    >
      <TextInput
        label={t('settings.members.name')}
        value={name}
        onChange={setName}
        isRequired
        isInvalid={tried && name.trim() === ''}
        errorMessage={t('settings.members.nameRequired')}
      />
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
        options={STAFF_ROLES.map((r) => ({
          id: r,
          label: t(`roles.${r}`),
          description: t(`settings.members.roles.${r}.who`),
        }))}
        value={role}
        onChange={setRole}
      />
      <TextInput
        label={t('settings.members.jobTitle')}
        value={jobTitle}
        onChange={setJobTitle}
        description={t('settings.members.jobTitleHint')}
      />
    </FormDialog>
  )
}
