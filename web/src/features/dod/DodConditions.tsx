import { ArrowRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { AutoApproveDays, DodBranch, DodConfig } from '../../api/client'
import { Checkbox } from '../../components/Checkbox'
import { Dropdown } from '../../components/Dropdown'
import { cx } from '../../lib/text'
import { AUTO_APPROVE_DAYS, DOD_BRANCHES, doneMeans, flowSteps } from './labels'

/** "Done means: …" and the status flow, as the Projects mock draws them under the conditions. */
export function DodPreview({ dod }: { dod: DodConfig }) {
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

type DodConditionsProps = {
  dod: DodConfig
  /** Merge, CI and deploy are read from a repository; without one they stay disabled. */
  hasRepository: boolean
  onChange: (next: DodConfig) => void
  legend: string
}

/** The five conditions as rows that turn blue when ticked, with their settings (Projects mock). */
export function DodConditions({ dod, hasRepository, onChange, legend }: DodConditionsProps) {
  const { t } = useTranslation()
  const set = (next: Partial<DodConfig>) => onChange({ ...dod, ...next })
  const codeHint = (hint: string) => (hasRepository ? hint : `${hint} ${t('dod.needsRepo')}.`)

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="sr-only">{legend}</legend>
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
