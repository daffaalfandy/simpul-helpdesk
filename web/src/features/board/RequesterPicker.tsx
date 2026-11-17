import { useTranslation } from 'react-i18next'

import type { Requester } from '../../api/client'
import { SearchSelect } from '../../components/SearchSelect'

type RequesterPickerProps = {
  clientId: string | null
  requesters: ReadonlyArray<Requester>
  isLoading: boolean
  value: string | null
  onChange: (id: string | null) => void
  isInvalid: boolean
}

/**
 * Pick one of the client's requesters. Only an administrator adds requesters, on the client in
 * Settings (owner, 17 Nov), so the empty list says where to ask.
 */
export function RequesterPicker({ clientId, requesters, isLoading, value, onChange, isInvalid }: RequesterPickerProps) {
  const { t } = useTranslation()
  return (
    <SearchSelect
      key={clientId ?? ''}
      label={t('agent.logTicket.requester')}
      placeholder={t('agent.logTicket.requesterPlaceholder')}
      description={clientId ? undefined : t('agent.logTicket.chooseClientFirst')}
      emptyText={isLoading ? t('agent.logTicket.loadingRequesters') : t('agent.logTicket.noRequesters')}
      options={requesters.map((r) => ({ id: r.id, label: r.name, description: r.email, searchText: `${r.name} ${r.email}` }))}
      value={value}
      onChange={onChange}
      isDisabled={!clientId}
      isRequired
      isInvalid={isInvalid}
      errorMessage={t('agent.logTicket.errors.requester')}
    />
  )
}
