import { Link } from '@tanstack/react-router'
import { useId, useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'

import type { AtRiskPercent, Duration, Project, ProjectSla, ProjectTarget, SlaCalendar } from '../../api/client'
import { usePriorityNames } from '../../api/queries'
import { PriorityBadge } from '../../components/Badges'
import { Checkbox } from '../../components/Checkbox'
import { Dropdown } from '../../components/Dropdown'
import { EditableCard } from '../../components/EditableCard'
import { ErrorAlert } from '../../components/ErrorAlert'
import { NumberInput } from '../../components/Fields'
import { fieldLabel } from '../../components/fieldStyles'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { paths } from '../../lib/constants'
import { formatTarget } from '../../lib/formatDuration'
import { DURATION_UNITS, MIN_TARGET_VALUE, orderError, TARGET_FIELDS, validValue, type TargetField } from '../../lib/projectSla'
import { useSaveProjectSla } from './queries'

const CALENDARS: readonly SlaCalendar[] = ['business_hours', 'always']
const AT_RISK: readonly AtRiskPercent[] = [50, 75, 90]

type SlaCardProps = {
  project: Project
  canEdit: boolean
  /** Settings › General is admin only; others get the note without a link. */
  canOpenSettings: boolean
  workingDayMinutes: number
  onEditStart: () => void
  onSaved: () => void
}

/**
 * The project's SLA (follow-up #1 C): a compact read-only view, and a form behind "Edit SLA".
 * Give it key={project.id} so switching projects discards unsaved changes.
 */
export function SlaCard({ project, canEdit, canOpenSettings, workingDayMinutes, onEditStart, onSaved }: SlaCardProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<ProjectSla | null>(null)
  const save = useSaveProjectSla(project.id)
  const invalid =
    draft !== null &&
    draft.targets.some((target) => !TARGET_FIELDS.every((f) => validValue(target[f])) || orderError(target, workingDayMinutes))

  return (
    <EditableCard
      title={t('projects.sla.title', { project: project.name })}
      description={t('projects.sla.description')}
      editLabel={t('projects.sla.edit')}
      saveLabel={t('projects.sla.save')}
      canEdit={canEdit}
      isEditing={draft !== null}
      isSaveDisabled={invalid}
      isPending={save.isPending}
      onEdit={() => {
        onEditStart()
        setDraft(structuredClone(project.sla))
      }}
      onCancel={() => {
        save.reset()
        setDraft(null)
      }}
      onSave={() => {
        if (!draft) return
        save.mutate(draft, {
          onSuccess: () => {
            setDraft(null)
            onSaved()
          },
        })
      }}
    >
      {draft ? (
        <>
          <SlaForm sla={draft} onChange={setDraft} workingDayMinutes={workingDayMinutes} canOpenSettings={canOpenSettings} />
          <ErrorAlert error={save.error} />
        </>
      ) : (
        <SlaSummary sla={project.sla} />
      )}
    </EditableCard>
  )
}

function SlaSummary({ sla }: { sla: ProjectSla }) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const pauses = [
    sla.pauseOnWaitingClient && t('projects.sla.pauses.waiting'),
    sla.pauseOnUat && t('projects.sla.pauses.uat'),
  ].filter(Boolean)
  const alerts = [
    sla.notifySupervisorOnBreach && t('projects.sla.alerts.supervisor'),
    sla.notifyPicOnP1Breach && t('projects.sla.alerts.pic'),
  ].filter(Boolean)
  const summary = [
    { term: t('projects.sla.pausesWhile'), value: pauses.length ? pauses.join(', ') : t('projects.sla.pauses.never') },
    { term: t('projects.sla.atRiskAt'), value: t('projects.sla.atRiskValue', { percent: sla.atRiskPercent }) },
    { term: t('projects.sla.breachAlerts'), value: alerts.length ? alerts.join(', ') : t('projects.sla.alerts.none') },
  ]
  return (
    <>
      <Table caption={t('projects.sla.caption')}>
        <thead>
          <tr>
            <Th>{t('projects.sla.cols.priority')}</Th>
            <Th>{t('projects.sla.cols.firstResponse')}</Th>
            <Th>{t('projects.sla.cols.resolve')}</Th>
            <Th>{t('projects.sla.cols.deliver')}</Th>
            <Th>{t('projects.sla.cols.clock')}</Th>
          </tr>
        </thead>
        <tbody>
          {sla.targets.map((target) => (
            <tr key={target.priority}>
              <RowHeader>
                <span className="flex items-center gap-2">
                  <PriorityBadge priority={target.priority} />
                  <span className="font-bold">{names[target.priority].name}</span>
                </span>
              </RowHeader>
              {TARGET_FIELDS.map((field) => (
                <Td key={field} className="font-bold">
                  {formatTarget(t, target[field])}
                </Td>
              ))}
              <Td className="text-ink-2">{t(`projects.sla.clock.${target.calendar}`)}</Td>
            </tr>
          ))}
        </tbody>
      </Table>
      <dl className="grid grid-cols-1 gap-x-4 gap-y-2.5 border-t border-border-subtle pt-3 text-sm sm:grid-cols-3">
        {summary.map((item) => (
          <div key={item.term} className="flex flex-col gap-0.5">
            <dt className="text-xs font-semibold text-muted">{item.term}</dt>
            <dd className="font-bold text-ink">{item.value}</dd>
          </div>
        ))}
      </dl>
    </>
  )
}

type SlaFormProps = {
  sla: ProjectSla
  onChange: (sla: ProjectSla) => void
  workingDayMinutes: number
  canOpenSettings: boolean
}

function SlaForm({ sla, onChange, workingDayMinutes, canOpenSettings }: SlaFormProps) {
  const { t } = useTranslation()
  const names = usePriorityNames()
  const updateTarget = (index: number, next: ProjectTarget) =>
    onChange({ ...sla, targets: sla.targets.map((target, i) => (i === index ? next : target)) })
  const set = <K extends keyof ProjectSla>(key: K, value: ProjectSla[K]) => onChange({ ...sla, [key]: value })

  return (
    <>
      {sla.targets.map((target, i) => (
        <PriorityTargets
          key={target.priority}
          target={target}
          name={names[target.priority].name}
          error={orderError(target, workingDayMinutes)}
          onChange={(next) => updateTarget(i, next)}
        />
      ))}
      <div className="grid grid-cols-1 gap-4 border-t border-border-subtle pt-3.5 sm:grid-cols-2">
        <div className="flex flex-col gap-2.5">
          <h3 className="text-xs font-extrabold tracking-caps text-muted uppercase">{t('projects.sla.pauseTitle')}</h3>
          <Checkbox
            isSelected={sla.pauseOnWaitingClient}
            onChange={(v) => set('pauseOnWaitingClient', v)}
            description={t('projects.sla.pauseWaitingHint')}
          >
            {t('projects.sla.pauses.waiting')}
          </Checkbox>
          <Checkbox isSelected={sla.pauseOnUat} onChange={(v) => set('pauseOnUat', v)} description={t('projects.sla.pauseUatHint')}>
            {t('projects.sla.pauses.uat')}
          </Checkbox>
          <p className="text-xs text-muted">
            {canOpenSettings ? (
              <Trans i18nKey="projects.sla.hoursNote" components={{ link: <Link to={paths.settingsGeneral} className="font-bold" /> }} />
            ) : (
              t('projects.sla.hoursNotePlain')
            )}
          </p>
        </div>
        <div className="flex flex-col gap-2.5">
          <h3 className="text-xs font-extrabold tracking-caps text-muted uppercase">{t('projects.sla.alertsTitle')}</h3>
          <Dropdown
            label={t('projects.sla.atRiskLabel')}
            options={AT_RISK.map((p) => ({ id: String(p), label: t('projects.sla.atRiskValue', { percent: p }) }))}
            value={String(sla.atRiskPercent)}
            onChange={(v) => set('atRiskPercent', Number(v) as AtRiskPercent)}
          />
          <Checkbox
            isSelected={sla.notifySupervisorOnBreach}
            onChange={(v) => set('notifySupervisorOnBreach', v)}
            description={t('projects.sla.notifySupervisorHint')}
          >
            {t('projects.sla.notifySupervisor')}
          </Checkbox>
          <Checkbox
            isSelected={sla.notifyPicOnP1Breach}
            onChange={(v) => set('notifyPicOnP1Breach', v)}
            description={t('projects.sla.notifyPicHint')}
          >
            {t('projects.sla.notifyPic')}
          </Checkbox>
        </div>
      </div>
    </>
  )
}

type PriorityTargetsProps = {
  target: ProjectTarget
  name: string
  error: 'resolve' | 'deliver' | null
  onChange: (target: ProjectTarget) => void
}

/** One priority's three targets and its clock, with the order rule shown inline. */
function PriorityTargets({ target, name, error, onChange }: PriorityTargetsProps) {
  const { t } = useTranslation()
  const headingId = useId()
  const errorId = useId()
  return (
    <div
      role="group"
      aria-labelledby={headingId}
      aria-describedby={error ? errorId : undefined}
      className="flex flex-col gap-3 rounded-xl border border-border px-4 py-3.5"
    >
      <p id={headingId} className="flex items-center gap-2.5">
        <PriorityBadge priority={target.priority} />
        <span className="text-md font-extrabold">{name}</span>
      </p>
      <div className="grid grid-cols-1 items-start gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {TARGET_FIELDS.map((field) => (
          <DurationField
            key={field}
            field={field}
            value={target[field]}
            onChange={(d) => onChange({ ...target, [field]: d })}
          />
        ))}
        <Dropdown
          label={t('projects.sla.fields.clock')}
          options={CALENDARS.map((c) => ({
            id: c,
            label: t(`projects.sla.clock.${c}`),
            description: t(`projects.sla.clockHint.${c}`),
          }))}
          value={target.calendar}
          onChange={(c) => onChange({ ...target, calendar: c })}
        />
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-xs font-bold text-chip-danger-fg">
          {t(`projects.sla.errors.${error}`)}
        </p>
      )}
    </div>
  )
}

function DurationField({ field, value, onChange }: { field: TargetField; value: Duration; onChange: (d: Duration) => void }) {
  const { t } = useTranslation()
  const labelId = useId()
  const label = t(`projects.sla.fields.${field}`)
  return (
    <div role="group" aria-labelledby={labelId} className="flex flex-col gap-1.5">
      <span id={labelId} className={fieldLabel('md')}>
        {label}
      </span>
      <div className="flex gap-1.5">
        <NumberInput
          label={t('projects.sla.fields.value')}
          hideLabel
          className="w-(--number-input-w) shrink-0"
          minValue={MIN_TARGET_VALUE}
          step={1}
          formatOptions={{ maximumFractionDigits: 0, useGrouping: false }}
          value={Number.isFinite(value.value) ? value.value : NaN}
          isInvalid={!validValue(value)}
          onChange={(v) => onChange({ ...value, value: Number.isFinite(v) ? Math.trunc(v) : 0 })}
        />
        <Dropdown
          label={t('projects.sla.fields.unit')}
          hideLabel
          className="min-w-0 flex-1"
          options={DURATION_UNITS.map((u) => ({ id: u, label: t(`projects.sla.units.${u}`) }))}
          value={value.unit}
          onChange={(unit) => onChange({ ...value, unit })}
        />
      </div>
    </div>
  )
}
