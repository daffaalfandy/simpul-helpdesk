import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { Form } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { setCurrentOrganization } from '../../api/organization'
import { meQueryOptions } from '../../api/queries'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths } from '../../lib/constants'
import {
  cleanOrganizationDraft,
  isOrganizationDraftValid,
  newOrganizationDraft,
  OrganizationFields,
} from '../settings/OrganizationFields'
import { useCreateOrganization } from '../settings/queries'
import { AuthLayout, authCard } from './AuthLayout'

/**
 * First-run setup, step 2: the administrator names the first organization. The app opens in it,
 * at Settings › General, to add the logo and footer note.
 */
export function OrganizationSetupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const create = useCreateOrganization()
  const [draft, setDraft] = useState(newOrganizationDraft)
  const [tried, setTried] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setTried(true)
    if (!isOrganizationDraftValid(draft)) return
    create.mutate(cleanOrganizationDraft(draft), {
      onSuccess: async (created) => {
        setCurrentOrganization(created.id)
        await queryClient.fetchQuery({ ...meQueryOptions, staleTime: 0 })
        void navigate({ to: paths.settingsGeneral, replace: true })
      },
    })
  }

  return (
    <AuthLayout title={t('setup.organizationStep.title')} intro={t('setup.organizationStep.intro')}>
      <Form onSubmit={onSubmit} className={authCard}>
        <ErrorAlert error={create.error} />
        <OrganizationFields draft={draft} onChange={setDraft} showErrors={tried} />
        <Button type="submit" size="lg" isPending={create.isPending} className="w-full">
          {t('setup.organizationStep.submit')}
        </Button>
      </Form>
    </AuthLayout>
  )
}
