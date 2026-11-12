import { useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Priority, SlaCalendar, SlaPlan, SlaPlanInput, SlaTarget } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Pill, PriorityBadge } from '../../components/Badges'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { NumberInput } from '../../components/Fields'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { dayUnit, fromParts, toParts, unitsFor, type DurationParts, type DurationUnit } from '../../lib/duration'
import {
  businessHoursQueryOptions,
  slaPlansQueryOptions,
  useDeletePlan,
  useSavePlan,
  useSetDefaultPlan,
} from './queries'
import { ErrorAlert, formatDuration, SectionHeader } from './shared'

const CALENDARS: readonly SlaCalendar[] = ['business_hours', 'always']
const TARGET_FIELDS = ['firstResponse', 'resolve', 'deliver'] as const
type TargetField = (typeof TARGET_FIELDS)[number]
const MINUTES_KEY: Record<TargetField, 'firstResponseMinutes' | 'resolveMinutes' | 'deliverMinutes'> = {
  firstResponse: 'firstResponseMinutes',
  resolve: 'resolveMinutes',
  deliver: 'deliverMinutes',
}
const MIN_TARGET = 1

export function SlaPage() {
  const { t } = useTranslation()
  const { data: plans } = useSuspenseQuery(slaPlansQueryOptions)
  const { data: hours } = useSuspenseQuery(businessHoursQueryOptions)
  const [flash, setFlash] = useState<string | null>(null)
  const [editing, setEditing] = useState<SlaPlan | 'new' | null>(null)
  const [deleting, setDeleting] = useState<SlaPlan | null>(null)
  const setDefault = useSetDefaultPlan()
  const remove = useDeletePlan()
  const defaultPlan = plans.find((p) => p.isDefault) ?? plans[0]

  const openDialog = (target: SlaPlan | 'new') => {
    setFlash(null)
    setEditing(target)
  }

  return (
    <>
      <SectionHeader
        section="sla"
        actions={
          <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => openDialog('new')}>
            {t('settings.sla.newPlan')}
          </Button>
        }
      />
      {flash && <Alert tone="success">{flash}</Alert>}
      <ErrorAlert error={setDefault.error} />
      {plans.map((plan) => (
        <PlanCard
          key={plan.id}
          plan={plan}
          workingDayMinutes={hours.workingDayMinutes}
          onEdit={() => openDialog(plan)}
          onDelete={() => {
            setFlash(null)
            setDeleting(plan)
          }}
          onMakeDefault={() =>
            setDefault.mutate(plan.id, { onSuccess: () => setFlash(t('settings.sla.defaultChanged', { name: plan.name })) })
          }
          isPending={setDefault.isPending && setDefault.variables === plan.id}
        />
      ))}
      <PriorityMatrix />
      {editing && defaultPlan && (
        <PlanDialog
          plan={editing === 'new' ? null : editing}
          template={editing === 'new' ? defaultPlan : editing}
          workingDayMinutes={hours.workingDayMinutes}
          onClose={() => setEditing(null)}
          onSaved={(saved, created) => {
            setEditing(null)
            setFlash(t(created ? 'settings.sla.created' : 'settings.sla.saved', { name: saved.name }))
          }}
        />
      )}
      <ConfirmDialog
        isOpen={deleting !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleting(null)
            remove.reset()
          }
        }}
        tone="danger"
        title={t('settings.sla.deleteTitle', { name: deleting?.name ?? '' })}
        confirmLabel={t('common.delete')}
        isPending={remove.isPending}
        error={<ErrorAlert error={remove.error} />}
        onConfirm={() => {
          if (!deleting) return
          remove.mutate(deleting.id, {
            onSuccess: () => {
              setFlash(t('settings.sla.deleted', { name: deleting.name }))
              setDeleting(null)
            },
          })
        }}
      >
        {t('settings.sla.deleteBody')}
      </ConfirmDialog>
    </>
  )
}

type PlanCardProps = {
  plan: SlaPlan
  workingDayMinutes: number
  onEdit: () => void
  onDelete: () => void
  onMakeDefault: () => void
  isPending: boolean
}

function PlanCard({ plan, workingDayMinutes, onEdit, onDelete, onMakeDefault, isPending }: PlanCardProps) {
  const { t } = useTranslation()
  const titleId = `plan-${plan.id}`
  const duration = (target: SlaTarget, minutes: number) => formatDuration(t, minutes, target.calendar, workingDayMinutes)
  return (
    <Card
      as="h3"
      labelledBy={titleId}
      title={
        <span className="flex flex-wrap items-center gap-2">
          {plan.name}
          {plan.isDefault && <Pill tone="info">{t('settings.sla.default')}</Pill>}
        </span>
      }
      description={
        <>
          {t('settings.sla.usedBy', { count: plan.clientCount })}
          {plan.isDefault && <> · {t('settings.sla.helper')}</>}
        </>
      }
      actions={
        <>
          <Button variant="outline" size="sm" onPress={onEdit} aria-label={t('settings.sla.editLabel', { name: plan.name })}>
            {t('common.edit')}
          </Button>
          {!plan.isDefault && (
            <Button
              variant="outline"
              size="sm"
              onPress={onMakeDefault}
              isPending={isPending}
              aria-label={t('settings.sla.makeDefaultLabel', { name: plan.name })}
            >
              {t('settings.sla.makeDefault')}
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            onPress={onDelete}
            isDisabled={plan.isDefault}
            aria-label={t('settings.sla.deleteLabel', { name: plan.name })}
          >
            {t('common.delete')}
          </Button>
        </>
      }
    >
      <Table caption={t('settings.sla.caption', { name: plan.name })}>
        <thead>
          <tr>
            <Th>{t('settings.sla.cols.priority')}</Th>
            <Th>{t('settings.sla.cols.example')}</Th>
            <Th>{t('settings.sla.cols.firstResponse')}</Th>
            <Th>{t('settings.sla.cols.resolve')}</Th>
            <Th>{t('settings.sla.cols.deliver')}</Th>
            <Th>{t('settings.sla.cols.hours')}</Th>
            <Th>{t('settings.sla.cols.escalate')}</Th>
          </tr>
        </thead>
        <tbody>
          {plan.targets.map((target) => (
            <tr key={target.priority}>
              <RowHeader>
                <PriorityBadge priority={target.priority}>
                  {t('priority.long', { priority: target.priority, name: t(`priority.${target.priority}.name`) })}
                </PriorityBadge>
              </RowHeader>
              <Td className="text-ink-2">{t(`priority.${target.priority}.example`)}</Td>
              <Td>{duration(target, target.firstResponseMinutes)}</Td>
              <Td>{duration(target, target.resolveMinutes)}</Td>
              <Td>
                {target.deliverFromCatalog
                  ? t('settings.sla.fromCatalog', { duration: duration(target, target.deliverMinutes) })
                  : duration(target, target.deliverMinutes)}
              </Td>
              <Td>{target.calendar === 'always' ? t('settings.sla.hoursAlways') : t('settings.sla.hoursBusiness')}</Td>
              <Td className="text-ink-2">{t(`priority.${target.priority}.escalation`)}</Td>
            </tr>
          ))}
        </tbody>
      </Table>
    </Card>
  )
}

type Row = {
  priority: Priority
  calendar: SlaCalendar
  deliverFromCatalog: boolean
  parts: Record<TargetField, DurationParts>
}

function rowsFrom(plan: SlaPlan, workingDayMinutes: number): Row[] {
  return plan.targets.map((target) => ({
    priority: target.priority,
    calendar: target.calendar,
    deliverFromCatalog: target.deliverFromCatalog,
    parts: Object.fromEntries(
      TARGET_FIELDS.map((f) => [f, toParts(target[MINUTES_KEY[f]], target.calendar, workingDayMinutes)]),
    ) as Record<TargetField, DurationParts>,
  }))
}

/** Swap a working-day unit for a calendar-day unit (and back) when a target's hours change. */
function switchCalendar(row: Row, calendar: SlaCalendar): Row {
  const parts = { ...row.parts }
  for (const f of TARGET_FIELDS) {
    const unit = parts[f].unit
    if (unit === 'workingDays' || unit === 'days') parts[f] = { ...parts[f], unit: dayUnit(calendar) }
  }
  return { ...row, calendar, parts }
}

type PlanDialogProps = {
  plan: SlaPlan | null
  template: SlaPlan
  workingDayMinutes: number
  onClose: () => void
  onSaved: (plan: SlaPlan, created: boolean) => void
}

function PlanDialog({ plan, template, workingDayMinutes, onClose, onSaved }: PlanDialogProps) {
  const { t } = useTranslation()
  const save = useSavePlan()
  const [name, setName] = useState(plan?.name ?? '')
  const [rows, setRows] = useState<Row[]>(() => rowsFrom(template, workingDayMinutes))
  const [submitted, setSubmitted] = useState(false)

  const update = (i: number, row: Row) => setRows((prev) => prev.map((r, j) => (j === i ? row : r)))
  const validValue = (v: number) => Number.isInteger(v) && v >= MIN_TARGET
  const allValid = name.trim() !== '' && rows.every((r) => TARGET_FIELDS.every((f) => validValue(r.parts[f].value)))

  const submit = () => {
    setSubmitted(true)
    if (!allValid) return
    const input: SlaPlanInput = {
      name: name.trim(),
      targets: rows.map((r) => ({
        priority: r.priority,
        calendar: r.calendar,
        deliverFromCatalog: r.deliverFromCatalog,
        firstResponseMinutes: fromParts(r.parts.firstResponse, workingDayMinutes),
        resolveMinutes: fromParts(r.parts.resolve, workingDayMinutes),
        deliverMinutes: fromParts(r.parts.deliver, workingDayMinutes),
      })),
    }
    save.mutate({ id: plan?.id, input }, { onSuccess: (saved) => onSaved(saved, plan === null) })
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={plan ? t('settings.sla.dialogEdit', { name: plan.name }) : t('settings.sla.dialogNew')}
      subtitle={plan ? undefined : t('settings.sla.copyOf', { name: template.name })}
      submitLabel={t('common.save')}
      onSubmit={submit}
      isPending={save.isPending}
      error={<ErrorAlert error={save.error} />}
    >
      <TextInput
        label={t('settings.sla.planName')}
        value={name}
        onChange={setName}
        isRequired
        isInvalid={submitted && name.trim() === ''}
        errorMessage={t('settings.sla.nameRequired')}
      />
      {rows.map((row, i) => {
        const priorityName = t('priority.long', { priority: row.priority, name: t(`priority.${row.priority}.name`) })
        return (
          <fieldset key={row.priority} className="flex flex-col gap-3 rounded-2xl border border-border p-4">
            <legend className="px-1">
              <PriorityBadge priority={row.priority}>{priorityName}</PriorityBadge>
            </legend>
            <div className="grid gap-3 sm:grid-cols-3">
              {TARGET_FIELDS.map((f) => {
                const field = t(`settings.sla.cols.${f}`)
                const parts = row.parts[f]
                return (
                  <div key={f} className="flex items-end gap-2">
                    <NumberInput
                      label={field}
                      value={parts.value}
                      onChange={(value) => update(i, { ...row, parts: { ...row.parts, [f]: { ...parts, value } } })}
                      minValue={MIN_TARGET}
                      step={1}
                      isInvalid={submitted && !validValue(parts.value)}
                      className="w-20 shrink-0"
                    />
                    <Dropdown<DurationUnit>
                      label={t('settings.sla.unit', { field, priority: row.priority })}
                      hideLabel
                      options={unitsFor(row.calendar).map((u) => ({ id: u, label: t(`duration.unit.${u}`) }))}
                      value={parts.unit}
                      onChange={(unit) => update(i, { ...row, parts: { ...row.parts, [f]: { ...parts, unit } } })}
                      className="min-w-0 flex-1"
                    />
                  </div>
                )
              })}
            </div>
            {submitted && TARGET_FIELDS.some((f) => !validValue(row.parts[f].value)) && (
              <p className="text-xs font-semibold text-danger">{t('settings.sla.positive')}</p>
            )}
            <div className="grid items-end gap-3 sm:grid-cols-2">
              <Dropdown<SlaCalendar>
                label={t('settings.sla.cols.hours')}
                options={CALENDARS.map((c) => ({
                  id: c,
                  label: c === 'always' ? t('settings.sla.hoursAlways') : t('settings.sla.hoursBusiness'),
                }))}
                value={row.calendar}
                onChange={(calendar) => update(i, switchCalendar(row, calendar))}
              />
              <Checkbox
                isSelected={row.deliverFromCatalog}
                onChange={(deliverFromCatalog) => update(i, { ...row, deliverFromCatalog })}
              >
                {t('settings.sla.deliverFromCatalog')}
              </Checkbox>
            </div>
          </fieldset>
        )
      })}
    </FormDialog>
  )
}

// HANDOFF §6 priority matrix: rows are impact, columns urgency.
const IMPACTS = ['office', 'team', 'user'] as const
const URGENCIES = ['canWait', 'slowed', 'stopped'] as const
const MATRIX: Record<(typeof IMPACTS)[number], readonly Priority[]> = {
  office: ['P2', 'P1', 'P1'],
  team: ['P3', 'P2', 'P1'],
  user: ['P4', 'P3', 'P2'],
}

function PriorityMatrix() {
  const { t } = useTranslation()
  return (
    <Card as="h3" title={t('settings.sla.matrix.title')} description={t('settings.sla.matrix.helper')}>
      <Table caption={t('settings.sla.matrix.caption')} minWidth="min-w-0">
        <thead>
          <tr>
            <Th>{t('settings.sla.matrix.impact')}</Th>
            {URGENCIES.map((u) => (
              <Th key={u}>{t(`settings.sla.matrix.urgencies.${u}`)}</Th>
            ))}
          </tr>
        </thead>
        <tbody>
          {IMPACTS.map((impact) => (
            <tr key={impact}>
              <RowHeader>{t(`settings.sla.matrix.impacts.${impact}`)}</RowHeader>
              {MATRIX[impact].map((p, i) => (
                <Td key={URGENCIES[i]}>
                  <PriorityBadge priority={p} />
                </Td>
              ))}
            </tr>
          ))}
        </tbody>
      </Table>
    </Card>
  )
}
