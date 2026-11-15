import { useQuery } from '@tanstack/react-query'
import { UserPlus, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button as AriaButton } from 'react-aria-components'

import type { Ticket } from '../../api/client'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { FormDialog } from '../../components/Dialog'
import { ErrorAlert } from '../../components/ErrorAlert'
import { SearchSelect } from '../../components/SearchSelect'
import { SEARCH_DEBOUNCE_MS } from '../../lib/constants'
import { useAddWatcher, useRemoveWatcher, watcherCandidatesQueryOptions, watchersQueryOptions } from '../tickets/queries'

/**
 * "People notified" on the portal ticket: the requester and the colleagues they added. Anyone
 * from the client can add or remove a colleague while the ticket is open.
 */
export function PeopleNotifiedCard({ ticket, meId }: { ticket: Ticket; meId: string }) {
  const { t } = useTranslation()
  const watchers = useQuery(watchersQueryOptions(ticket.key))
  const remove = useRemoveWatcher(ticket.key)
  const [adding, setAdding] = useState(false)
  const open = ticket.status !== 'closed'
  const requester =
    ticket.requester.id === meId ? t('portal.ticket.you', { name: ticket.requester.name }) : ticket.requester.name

  return (
    <Card as="h2" variant="compact" title={t('portal.ticket.peopleNotified')}>
      <ul className="flex flex-col gap-1.5 text-sm">
        <li className="flex min-h-(--control-h-sm) items-center">{requester}</li>
        {watchers.data?.map((w) => (
          <li key={w.id} className="flex min-h-(--control-h-sm) items-center justify-between gap-2">
            <span className="flex min-w-0 flex-col">
              <span>{w.id === meId ? t('portal.ticket.you', { name: w.name }) : w.name}</span>
              {w.jobTitle && <span className="text-xs text-muted">{w.jobTitle}</span>}
            </span>
            {open && (
              <AriaButton
                aria-label={t('portal.ticket.removeColleague', { name: w.name })}
                isDisabled={remove.isPending}
                onPress={() => remove.mutate(w.id)}
                className="hit-area inline-flex size-(--control-h-sm) shrink-0 cursor-pointer items-center justify-center rounded-md text-muted hover:bg-surface-subtle hover:text-ink data-disabled:cursor-not-allowed"
              >
                <X aria-hidden="true" className="size-(--icon-md)" />
              </AriaButton>
            )}
          </li>
        ))}
      </ul>
      {watchers.isError && <p className="text-sm text-muted">{t('portal.ticket.watchersError')}</p>}
      <ErrorAlert error={remove.error} />
      {open && (
        <Button
          variant="outline"
          icon={<UserPlus aria-hidden="true" className="size-(--icon-md)" />}
          onPress={() => setAdding(true)}
          className="self-start"
        >
          {t('portal.ticket.addColleague')}
        </Button>
      )}
      {adding && <AddColleagueDialog ticketKey={ticket.key} onClose={() => setAdding(false)} />}
    </Card>
  )
}

function AddColleagueDialog({ ticketKey, onClose }: { ticketKey: string; onClose: () => void }) {
  const { t } = useTranslation()
  const [typed, setTyped] = useState('')
  const [query, setQuery] = useState('')
  const [chosen, setChosen] = useState<string | null>(null)
  const [missing, setMissing] = useState(false)
  const candidates = useQuery(watcherCandidatesQueryOptions(ticketKey, query))
  const add = useAddWatcher(ticketKey)

  // The server narrows the list by name once typing pauses.
  useEffect(() => {
    const timer = setTimeout(() => setQuery(typed.trim()), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [typed])

  const submit = () => {
    if (!chosen) {
      setMissing(true)
      return
    }
    add.mutate(chosen, { onSuccess: onClose })
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(isOpen) => !isOpen && onClose()}
      title={t('portal.ticket.addColleague')}
      subtitle={t('portal.ticket.addColleagueSubtitle')}
      submitLabel={t('portal.ticket.add')}
      onSubmit={submit}
      isPending={add.isPending}
      error={<ErrorAlert error={add.error ?? candidates.error} />}
    >
      <SearchSelect
        label={t('portal.ticket.colleague')}
        placeholder={t('portal.ticket.colleaguePlaceholder')}
        emptyText={t('portal.ticket.noColleagues')}
        options={(candidates.data ?? []).map((c) => ({ id: c.id, label: c.name, description: c.jobTitle ?? undefined }))}
        value={chosen}
        onChange={(id) => {
          setChosen(id)
          setMissing(false)
        }}
        onQueryChange={setTyped}
        isRequired
        isInvalid={missing}
        errorMessage={t('portal.ticket.chooseColleague')}
      />
    </FormDialog>
  )
}
