import { parseDate, type CalendarDate } from '@internationalized/date'
import { useSuspenseQuery } from '@tanstack/react-query'
import { Building2, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Client, ClientInput } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox, CheckboxGroup } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { EmptyState } from '../../components/EmptyState'
import { DateInput } from '../../components/Fields'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { formatMonthYear, formatPeriod } from '../../lib/dates'
import { Roles } from '../../lib/roles'
import { clientsQueryOptions, useDeleteClient, usersQueryOptions, useSaveClient } from './queries'
import { ErrorAlert, SectionHeader } from './shared'

const ACTIVE = 'active'

export function ClientsPage() {
  const { t } = useTranslation()
  const { data: clients } = useSuspenseQuery(clientsQueryOptions)
  const [flash, setFlash] = useState<string | null>(null)
  const [editing, setEditing] = useState<Client | 'new' | null>(null)

  const open = (target: Client | 'new') => {
    setFlash(null)
    setEditing(target)
  }
  const addButton = (
    <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => open('new')}>
      {t('settings.clients.add')}
    </Button>
  )

  return (
    <>
      <SectionHeader section="clients" />
      {flash && <Alert tone="success">{flash}</Alert>}
      {clients.length === 0 ? (
        <>
          <EmptyState icon={Building2} title={t('settings.clients.emptyTitle')}>
            {t('settings.clients.empty')}
          </EmptyState>
          <div className="self-center">{addButton}</div>
        </>
      ) : (
        <Card as="h3" title={t('settings.clients.cardTitle')} actions={addButton}>
          <ClientsTable clients={clients} onEdit={open} />
        </Card>
      )}
      {editing && (
        <ClientDialog
          client={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onDone={(message) => {
            setEditing(null)
            setFlash(message)
          }}
        />
      )}
    </>
  )
}

function ClientsTable({ clients, onEdit }: { clients: Client[]; onEdit: (c: Client) => void }) {
  const { t } = useTranslation()
  const period = (c: Client) => {
    if (c.contractStart && c.contractEnd) return formatPeriod(c.contractStart, c.contractEnd)
    if (c.contractStart) return t('settings.clients.from', { date: formatMonthYear(c.contractStart) })
    if (c.contractEnd) return t('settings.clients.until', { date: formatMonthYear(c.contractEnd) })
    return t('common.none')
  }
  return (
    <Table caption={t('settings.clients.caption')}>
      <thead>
        <tr>
          <Th>{t('settings.clients.cols.client')}</Th>
          <Th>{t('settings.clients.cols.contractNo')}</Th>
          <Th>{t('settings.clients.cols.period')}</Th>
          <Th>{t('settings.clients.cols.projects')}</Th>
          <Th>{t('settings.clients.cols.requesters')}</Th>
          <Th>{t('settings.clients.cols.pic')}</Th>
          <Th>
            <span className="sr-only">{t('settings.clients.cols.actions')}</span>
          </Th>
        </tr>
      </thead>
      <tbody>
        {clients.map((c) => (
          <tr key={c.id}>
            <RowHeader>{c.name}</RowHeader>
            <Td className="text-ink-2">{c.contractNo ?? t('common.none')}</Td>
            <Td className="whitespace-nowrap">{period(c)}</Td>
            <Td>
              {c.projects.length > 0 ? (
                c.projects.map((p) => p.name).join(', ')
              ) : (
                <span className="text-muted">{t('settings.clients.noProjects')}</span>
              )}
            </Td>
            <Td className="tabular-nums">{c.requesterCount}</Td>
            <Td>{c.picName ?? t('common.none')}</Td>
            <Td className="text-right">
              <Button
                variant="outline"
                size="sm"
                onPress={() => onEdit(c)}
                aria-label={t('settings.clients.editLabel', { name: c.name })}
              >
                {t('common.edit')}
              </Button>
            </Td>
          </tr>
        ))}
      </tbody>
    </Table>
  )
}

const blankToNull = (s: string) => (s.trim() === '' ? null : s.trim())
const toDate = (iso?: string | null) => (iso ? parseDate(iso) : null)

type ClientDialogProps = { client: Client | null; onClose: () => void; onDone: (message: string) => void }

function ClientDialog({ client, onClose, onDone }: ClientDialogProps) {
  const { t } = useTranslation()
  const { data: users } = useSuspenseQuery(usersQueryOptions)
  const save = useSaveClient()
  const remove = useDeleteClient()

  const [name, setName] = useState(client?.name ?? '')
  const [contractNo, setContractNo] = useState(client?.contractNo ?? '')
  const [start, setStart] = useState<CalendarDate | null>(toDate(client?.contractStart))
  const [end, setEnd] = useState<CalendarDate | null>(toDate(client?.contractEnd))
  const [picName, setPicName] = useState(client?.picName ?? '')
  const [picEmail, setPicEmail] = useState(client?.picEmail ?? '')
  const previousAgentIds = client?.agents.map((a) => a.id) ?? []
  const [agentIds, setAgentIds] = useState<string[]>(previousAgentIds)
  const [submitted, setSubmitted] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const agents = users.filter((u) => u.role === Roles.Agent && u.status === ACTIVE)
  const periodInvalid = start !== null && end !== null && end.compare(start) < 0
  const nameMissing = name.trim() === ''

  const submit = () => {
    setSubmitted(true)
    if (nameMissing || periodInvalid) return
    const input: ClientInput = {
      name: name.trim(),
      contractNo: blankToNull(contractNo),
      contractStart: start?.toString() ?? null,
      contractEnd: end?.toString() ?? null,
      picName: blankToNull(picName),
      picEmail: blankToNull(picEmail),
    }
    save.mutate(
      { id: client?.id, input, agentIds, previousAgentIds },
      {
        onSuccess: (saved) =>
          onDone(t(client ? 'settings.clients.saved' : 'settings.clients.created', { name: saved.name })),
      },
    )
  }

  return (
    <>
      <FormDialog
        isOpen={!confirmingDelete}
        onOpenChange={(isOpen) => !isOpen && onClose()}
        title={client ? t('settings.clients.dialogEdit', { name: client.name }) : t('settings.clients.dialogNew')}
        submitLabel={client ? t('common.save') : t('settings.clients.add')}
        onSubmit={submit}
        isPending={save.isPending}
        error={<ErrorAlert error={save.error} />}
        footerStart={
          client && (
            <Button variant="danger" onPress={() => setConfirmingDelete(true)}>
              {t('settings.clients.delete')}
            </Button>
          )
        }
      >
        <TextInput
          label={t('settings.clients.name')}
          value={name}
          onChange={setName}
          isRequired
          isInvalid={submitted && nameMissing}
          errorMessage={t('settings.clients.nameRequired')}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <TextInput
            className="sm:col-span-2"
            label={t('settings.clients.contractNo')}
            value={contractNo}
            onChange={setContractNo}
          />
          <DateInput label={t('settings.clients.start')} value={start} onChange={setStart} />
          <DateInput
            label={t('settings.clients.end')}
            value={end}
            onChange={setEnd}
            isInvalid={submitted && periodInvalid}
            errorMessage={t('settings.clients.periodInvalid')}
          />
          <TextInput label={t('settings.clients.picName')} value={picName} onChange={setPicName} />
          <TextInput label={t('settings.clients.picEmail')} type="email" value={picEmail} onChange={setPicEmail} />
        </div>
        <CheckboxGroup
          label={t('settings.clients.agents')}
          description={agents.length > 0 ? t('settings.clients.agentsHelp') : t('settings.clients.noActiveAgents')}
          value={agentIds}
          onChange={setAgentIds}
        >
          {agents.map((a) => (
            <Checkbox key={a.id} value={a.id} description={a.jobTitle ?? undefined}>
              {a.name}
            </Checkbox>
          ))}
        </CheckboxGroup>
      </FormDialog>
      {client && (
        <ConfirmDialog
          isOpen={confirmingDelete}
          onOpenChange={(isOpen) => {
            if (!isOpen) {
              setConfirmingDelete(false)
              remove.reset()
            }
          }}
          tone="danger"
          title={t('settings.clients.deleteTitle', { name: client.name })}
          confirmLabel={t('settings.clients.delete')}
          isPending={remove.isPending}
          error={<ErrorAlert error={remove.error} />}
          onConfirm={() =>
            remove.mutate(client.id, { onSuccess: () => onDone(t('settings.clients.deleted', { name: client.name })) })
          }
        >
          {t('settings.clients.deleteBody')}
        </ConfirmDialog>
      )}
    </>
  )
}
