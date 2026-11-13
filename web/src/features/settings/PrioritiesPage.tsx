import { useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Clock } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Priority, PriorityLevel } from '../../api/client'
import { priorityLevelsQueryOptions } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { PriorityBadge, priorityClasses } from '../../components/Badges'
import { buttonClasses } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { UnsavedBar } from '../../components/UnsavedBar'
import { paths } from '../../lib/constants'
import { cx } from '../../lib/text'
import { useSavePriorityLevels } from './queries'
import { ErrorAlert, SectionHeader } from './shared'

// Field limits, matching the API (api/internal/priorities).
const MAX_NAME_LENGTH = 40
const MAX_DESCRIPTION_LENGTH = 200

type Field = 'name' | 'description'

const isBlank = (s: string) => s.trim() === ''

/** Settings › Priorities (follow-up #1): global priority names, and the impact × urgency guidance. */
export function PrioritiesPage() {
  const { t } = useTranslation()
  const { data: saved } = useSuspenseQuery(priorityLevelsQueryOptions)
  const [draft, setDraft] = useState<PriorityLevel[]>(saved)
  const [showErrors, setShowErrors] = useState(false)
  const [flash, setFlash] = useState(false)
  const save = useSavePriorityLevels()
  const namesId = useId()
  const dirty = draft.some((l, i) => l.name !== saved[i]?.name || l.description !== saved[i]?.description)
  const invalid = draft.some((l) => isBlank(l.name) || isBlank(l.description))

  const edit = (code: Priority, field: Field, value: string) => {
    setFlash(false)
    setDraft((levels) => levels.map((l) => (l.code === code ? { ...l, [field]: value } : l)))
  }
  const discard = () => {
    setDraft(saved)
    setShowErrors(false)
    save.reset()
  }
  const submit = () => {
    setShowErrors(true)
    if (invalid) return
    save.mutate(draft, {
      onSuccess: (levels) => {
        setDraft(levels)
        setShowErrors(false)
        setFlash(true)
      },
    })
  }

  return (
    <>
      <SectionHeader section="priorities" />
      {flash && <Alert tone="success">{t('settings.priorities.saved')}</Alert>}
      <div className="flex flex-wrap items-center justify-between gap-3.5 rounded-2xl border border-primary-tint-2 bg-primary-tint px-4.5 py-3.5">
        <p className="flex items-center gap-2.5 text-base font-semibold text-chip-info-fg">
          <Clock aria-hidden="true" className="size-(--icon-lg) shrink-0" />
          {t('settings.priorities.banner')}
        </p>
        <Link to={paths.projects} className={buttonClasses('primary')}>
          {t('settings.priorities.setSla')}
        </Link>
      </div>
      <div className="flex flex-wrap items-start gap-4">
        <section
          aria-labelledby={namesId}
          className="flex min-w-0 flex-[999_1_520px] flex-col gap-3 rounded-3xl border border-border bg-surface px-5 py-4.5 shadow-card"
        >
          <div className="flex flex-col gap-1">
            <h3 id={namesId} className="text-md font-extrabold">
              {t('settings.priorities.namesTitle')}
            </h3>
            <p className="text-sm text-muted">{t('settings.priorities.namesHelper')}</p>
          </div>
          {draft.map((level) => {
            const forPriority = <span className="sr-only"> {t('settings.priorities.forPriority', { priority: level.code })}</span>
            return (
              <div key={level.code} className="flex flex-wrap items-end gap-3 border-t border-border-subtle pt-3">
                <PriorityBadge priority={level.code} size="lg" />
                <TextInput
                  className="flex-[1_1_150px]"
                  label={
                    <>
                      {t('settings.priorities.name')}
                      {forPriority}
                    </>
                  }
                  value={level.name}
                  maxLength={MAX_NAME_LENGTH}
                  onChange={(v) => edit(level.code, 'name', v)}
                  isInvalid={showErrors && isBlank(level.name)}
                  errorMessage={t('settings.priorities.required')}
                />
                <TextInput
                  className="flex-[3_1_260px]"
                  label={
                    <>
                      {t('settings.priorities.description')}
                      {forPriority}
                    </>
                  }
                  value={level.description}
                  maxLength={MAX_DESCRIPTION_LENGTH}
                  onChange={(v) => edit(level.code, 'description', v)}
                  isInvalid={showErrors && isBlank(level.description)}
                  errorMessage={t('settings.priorities.required')}
                />
              </div>
            )
          })}
        </section>
        <PriorityMatrix />
      </div>
      <ErrorAlert error={save.error} />
      {dirty && (
        <UnsavedBar message={t('settings.priorities.unsaved')} onDiscard={discard} onSave={submit} isPending={save.isPending} />
      )}
    </>
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

/** Read-only guidance for agents (Settings mock): a grid of coloured priority blocks. */
function PriorityMatrix() {
  const { t } = useTranslation()
  const titleId = useId()
  return (
    <section
      aria-labelledby={titleId}
      className="flex min-w-0 flex-[1_1_320px] flex-col gap-1 rounded-3xl border border-border bg-surface px-5 py-4.5 shadow-card"
    >
      <h3 id={titleId} className="text-md font-extrabold">
        {t('settings.priorities.matrix.title')}
      </h3>
      <p className="mb-2.5 text-sm text-muted">{t('settings.priorities.matrix.helper')}</p>
      <table className="w-full border-separate border-spacing-1.5 text-xs">
        <caption className="sr-only">{t('settings.priorities.matrix.caption')}</caption>
        <thead>
          <tr>
            <th scope="col" className="sr-only">
              {t('settings.priorities.matrix.impact')}
            </th>
            {URGENCIES.map((u) => (
              <th key={u} scope="col" className="px-0.5 py-1.5 text-center font-bold text-muted">
                {t(`settings.priorities.matrix.urgencies.${u}`)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {IMPACTS.map((impact) => (
            <tr key={impact}>
              <th scope="row" className="py-2.5 text-left font-bold text-muted">
                {t(`settings.priorities.matrix.impacts.${impact}`)}
              </th>
              {MATRIX[impact].map((p, i) => (
                <td key={URGENCIES[i]} className={cx('rounded-sm py-2.5 text-center font-extrabold', priorityClasses[p])}>
                  {p}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
