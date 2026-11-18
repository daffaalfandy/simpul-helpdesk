import { ArrowRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { AutoApproveDays, DodConfig } from '../../api/client'
import { Checkbox } from '../../components/Checkbox'
import { Dropdown } from '../../components/Dropdown'
import { TextInput } from '../../components/TextInput'
import { MAX_BRANCH_LENGTH } from '../../lib/constants'
import { cx } from '../../lib/text'
import { AUTO_APPROVE_DAYS, doneMeans, flowSteps } from './labels'

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
  onChange: (next: DodConfig) => void
  legend: string
  /** The project reads commits from a repository; without one the code condition can't be on. */
  hasRepository: boolean
  /** The API refused the branch name: the field shows it. */
  branchInvalid?: boolean
}

/**
 * The conditions as rows that turn blue when ticked, with their settings (Projects mock): the
 * code reaching a branch (10b, owner 18 Nov: Git ticks it, never moves the ticket), then the
 * client's checks.
 */
export function DodConditions({ dod, onChange, legend, hasRepository, branchInvalid }: DodConditionsProps) {
  const { t } = useTranslation()
  const set = (next: Partial<DodConfig>) => onChange({ ...dod, ...next })

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="sr-only">{legend}</legend>
      <ConditionRow
        on={dod.codeOnBranch.enabled}
        isDisabled={!hasRepository}
        label={t('dod.items.code_on_branch.label')}
        hint={t(hasRepository ? 'dod.items.code_on_branch.hint' : 'dod.items.code_on_branch.noRepository')}
        onToggle={(on) => set({ codeOnBranch: { ...dod.codeOnBranch, enabled: on } })}
        control={
          <TextInput
            label={t('dod.branchLabel')}
            placeholder={t('dod.branchPlaceholder')}
            value={dod.codeOnBranch.branch}
            onChange={(branch) => set({ codeOnBranch: { ...dod.codeOnBranch, branch } })}
            isDisabled={!hasRepository}
            isInvalid={branchInvalid}
            maxLength={MAX_BRANCH_LENGTH}
            autoComplete="off"
            spellCheck="false"
          />
        }
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
  hint: ReactNode
  onToggle: (on: boolean) => void
  /** The condition's setting (branch, auto-approve), kept while the condition is off. */
  control?: ReactNode
  /** The condition can't be turned on here (no repository); the hint says why. */
  isDisabled?: boolean
}

/** One condition: a bordered row that turns primary when ticked (Projects mock). */
function ConditionRow({ on, label, hint, onToggle, control, isDisabled }: ConditionRowProps) {
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
