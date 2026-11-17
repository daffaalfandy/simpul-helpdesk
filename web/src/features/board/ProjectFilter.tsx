import { useSuspenseQuery } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { Button as AriaButton, Dialog, DialogTrigger, Popover } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { Button } from '../../components/Button'
import { Checkbox, CheckboxGroup } from '../../components/Checkbox'
import { ErrorAlert } from '../../components/ErrorAlert'
import { pillClasses } from '../../components/FilterPills'
import { POPOVER_OFFSET_PX } from '../../lib/constants'
import { cx } from '../../lib/text'
import { projectFilterQueryOptions, projectsQueryOptions, useSetProjectFilter } from '../projects/queries'
import { useSignedInUser } from '../shell/useSignedInUser'

/** Picking among fewer projects than this changes nothing, so the filter stays out of the way. */
const MIN_PROJECTS_TO_FILTER = 2

/**
 * The projects this user sees on the board, its numbers and ticket lists, saved for them in
 * this organization (owner, 17 Nov). Each tick saves; nothing ticked means every project, and
 * "Show all projects" clears the filter and closes the list.
 */
export function ProjectFilter() {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const { data: projects } = useSuspenseQuery(projectsQueryOptions)
  const { data: filter } = useSuspenseQuery(projectFilterQueryOptions)
  const save = useSetProjectFilter()
  const [isOpen, setOpen] = useState(false)
  const picked = filter.projectIds
  if (projects.length < MIN_PROJECTS_TO_FILTER && picked.length === 0) return null

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={setOpen}>
      <AriaButton className={pillClasses(picked.length > 0)}>
        {picked.length === 0
          ? t('board.projects.all')
          : t('board.projects.some', { picked: picked.length, total: projects.length })}
        <ChevronDown aria-hidden="true" className="size-(--icon-md)" />
      </AriaButton>
      <Popover
        placement="bottom start"
        offset={POPOVER_OFFSET_PX}
        className={cx(
          'w-(--project-filter-w) max-w-[calc(100vw-2*var(--space-4))] rounded-2xl border border-border-strong bg-surface',
          'shadow-popover outline-none data-entering:animate-enter',
        )}
      >
        <Dialog aria-label={t('board.projects.title')} className="flex flex-col gap-3 p-4 outline-none">
          <CheckboxGroup
            label={t('board.projects.title')}
            description={t('board.projects.hint', { org: me.organization?.name ?? '' })}
            value={picked}
            onChange={(ids) => save.mutate(ids)}
          >
            <div className="max-h-(--project-filter-max-h) overflow-y-auto">
              {projects.map((p) => (
                <Checkbox key={p.id} value={p.id} description={p.client.name}>
                  {p.name}
                </Checkbox>
              ))}
            </div>
          </CheckboxGroup>
          <ErrorAlert error={save.error} />
          {picked.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="self-start px-0"
              onPress={() => {
                save.mutate([])
                setOpen(false)
              }}
            >
              {t('board.projects.showAll')}
            </Button>
          )}
        </Dialog>
      </Popover>
    </DialogTrigger>
  )
}
