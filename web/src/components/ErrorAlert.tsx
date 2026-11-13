import { useTranslation } from 'react-i18next'

import { errorMessage } from '../api/errors'
import { Alert } from './Alert'

/** An API error as a translated alert; nothing when there is no error. */
export function ErrorAlert({ error }: { error: unknown }) {
  const { t } = useTranslation()
  if (!error) return null
  return <Alert tone="error">{errorMessage(t, error)}</Alert>
}
