import { ArrowRight } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { AutoApproveDays, DodBranch, DodConfig, Project } from '../../api/client'
import { Checkbox } from '../../components/Checkbox'
import { Dropdown } from '../../components/Dropdown'
import { EditableCard } from '../../components/EditableCard'
import { ErrorAlert } from '../../components/ErrorAlert'
import { cx } from '../../lib/text'
import { AUTO_APPROVE_DAYS, DOD_BRANCHES, doneMeans, flowSteps } from '../dod/labels'
import { useSaveProjectDod } from './queries'

type ProjectDod = { dod: DodConfig; allowTicketOverride: boolean }

type ProjectDodCardProps = {
  project: Project
  canEdit: boolean
  onEditStart: () => void
  onSaved: () => void
}

/**
 * The project's Definition of Done (HANDOFF §7, Projects mock): what has to happen before a
 * ticket resolves by itself, read compactly, and a form behind "Edit DoD". Give it
 * key={project.id} so switching projects discards unsaved changes.
 */
export function ProjectDodCard({ project, canEdit, onEditStart, onSaved }: ProjectDodCardProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<ProjectDod | null>(null)
  const save = useSaveProjectDod(project.id)

  return (
    <EditableCard
      tone="emphasis"
      title={t('projects.dod.title')}
      description={t('projects.dod.description')}
      editLabel={t('projects.dod.edit')}
      saveLabel={t('projects.dod.save')}
      canEdit={canEdit}
      isEditing={draft !== null}
      isPending={save.isPending}
      onEdit={() => {
        onEditStart()
        setDraft({ dod: structuredClone(project.dod), allowTicketOverride: project.allowTicketOverride })
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
          <DodForm value={draft} hasRepository={project.hasRepository} onChange={setDraft} />
          <ErrorAlert error={save.error} />
        </>
      ) : (
        <>
          <DodPreview dod={project.dod} />
          <p className="text-sm text-ink-2">
            {t(project.allowTicketOverride ? 'projects.dod.overrideOn' : 'projects.dod.overrideOff')}
          </p>
        </>
      )}
    </EditableCard>
  )
}

/** "Done means: …" and the status flow, as the mock draws them under the conditions. */
function DodPreview({ dod }: { dod: DodConfig }) {
  const { t, i18n } = useTranslation()
  const steps = flowSteps(t, dod)
  return (
    <>
      <p className="rounded-2xl bg-tint px-4 py-3 text-base leading-relaxed text-ink" aria-live="polite">
        <b>{t('dod.doneMeans')}</b> {doneMeans(t, dod, i18n.language)}
      </p>
      <ol aria-label={t('dod.flowLabel')} className="flex flex-wrap items-center gap-1.5">
        {steps.map((step, i) => {
          const last = i === steps.length - 1
          return (
            <li key={step} className="flex items-center gap-1.5">
              <span
                className={cx(
                  'rounded-full border px-3 py-1 text-xs font-bold',
                  last ? 'border-ink bg-ink text-on-primary' : 'border-border-strong bg-surface text-ink',
                )}
              >
                {step}
              </span>
              {!last && <ArrowRight aria-hidden="true" className="size-(--icon-sm) text-muted" />}
            </li>
          )
        })}
      </ol>
    </>
  )
}

type DodFormProps = {
  value: ProjectDod
  hasRepository: boolean
  onChange: (next: ProjectDod) => void
}

function DodForm({ value, hasRepository, onChange }: DodFormProps) {
  const { t } = useTranslation()
  const { dod } = value
  const set = (next: Partial<DodConfig>) => onChange({ ...value, dod: { ...dod, ...next } })
  const codeHint = (hint: string) => (hasRepository ? hint : `${hint} ${t('dod.needsRepo')}.`)

  return (
    <>
      <fieldset className="flex flex-col gap-2">
        <legend className="sr-only">{t('projects.dod.conditionsLabel')}</legend>
        <ConditionRow
          on={dod.merged.enabled}
          label={t('dod.items.merged.label')}
          hint={codeHint(t('dod.items.merged.hint'))}
          isDisabled={!hasRepository}
          onToggle={(on) => set({ merged: { ...dod.merged, enabled: on } })}
          control={
            <Dropdown<DodBranch>
              label={t('dod.branchLabel')}
              options={DOD_BRANCHES.map((b) => ({ id: b, label: t(`dod.branch.${b}.label`), description: t(`dod.branch.${b}.hint`) }))}
              value={dod.merged.branch}
              onChange={(branch) => set({ merged: { ...dod.merged, branch } })}
              isDisabled={!hasRepository}
            />
          }
        />
        <ConditionRow
          on={dod.ciPassed}
          label={t('dod.items.ci_passed.label')}
          hint={codeHint(t('dod.items.ci_passed.hint'))}
          isDisabled={!hasRepository}
          onToggle={(ciPassed) => set({ ciPassed })}
        />
        <ConditionRow
          on={dod.deployedProduction}
          label={t('dod.items.deployed.label')}
          hint={codeHint(t('dod.items.deployed.hint'))}
          isDisabled={!hasRepository}
          onToggle={(deployedProduction) => set({ deployedProduction })}
        />
        <ConditionRow
          on={dod.uat.enabled}
          label={t('dod.items.uat_approved.label')}
          hint={t('dod.items.uat_approved.hint')}
          onToggle={(on) => set({ uat: { ...dod.uat, enabled: on } })}
          control={
            <Dropdown<`${AutoApproveDays}`>
              label={t('dod.autoApproveLabel')}
              options={AUTO_APPROVE_DAYS.map((d) => ({
                id: `${d}`,
                label: t(`dod.autoApprove.${d}.label`),
                description: t(`dod.autoApprove.${d}.hint`, { defaultValue: '' }) || undefined,
              }))}
              value={`${dod.uat.autoApproveWorkingDays}`}
              onChange={(days) => set({ uat: { ...dod.uat, autoApproveWorkingDays: Number(days) as AutoApproveDays } })}
            />
          }
        />
        <ConditionRow
          on={dod.requesterConfirms}
          label={t('dod.items.requester_confirmed.label')}
          hint={t('dod.items.requester_confirmed.hint')}
          onToggle={(requesterConfirms) => set({ requesterConfirms })}
        />
      </fieldset>
      <DodPreview dod={dod} />
      <Checkbox
        isSelected={value.allowTicketOverride}
        onChange={(allowTicketOverride) => onChange({ ...value, allowTicketOverride })}
        description={t('projects.dod.allowOverride.hint')}
      >
        {t('projects.dod.allowOverride.label')}
      </Checkbox>
    </>
  )
}

type ConditionRowProps = {
  on: boolean
  label: string
  hint: string
  isDisabled?: boolean
  onToggle: (on: boolean) => void
  /** The condition's setting (branch, auto-approve), kept while the condition is off. */
  control?: ReactNode
}

/** One condition: a bordered row that turns primary when ticked (Projects mock). */
function ConditionRow({ on, label, hint, isDisabled, onToggle, control }: ConditionRowProps) {
  return (
    <div
      className={cx(
        'flex flex-col gap-2 rounded-xl border-2 px-3.5 py-2.5 transition-colors duration-(--duration-fast) ease-standard',
        'sm:flex-row sm:items-center sm:justify-between',
        on ? 'border-primary bg-primary-tint' : 'border-border bg-surface',
      )}
    >
      <Checkbox isSelected={on} isDisabled={isDisabled} onChange={onToggle} description={hint} className="flex-1">
        {label}
      </Checkbox>
      {control && <div className="w-full sm:w-(--dod-control-w) sm:shrink-0">{control}</div>}
    </div>
  )
}
