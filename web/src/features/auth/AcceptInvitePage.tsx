import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Form } from 'react-aria-components'

import { ApiError, ErrorCode } from '../../api/client'
import { useAcceptInvite } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { MAX_PASSWORD_LENGTH, MIN_PASSWORD_LENGTH, RESET_TOKEN_HASH_KEY } from '../../lib/constants'
import { AuthLayout, authCard } from './AuthLayout'
import { usePostSignIn } from './usePostSignIn'

/** The invite token travels in the URL fragment, as reset tokens do, so it never reaches logs. */
function readTokenFromHash(): string {
  return new URLSearchParams(window.location.hash.slice(1)).get(RESET_TOKEN_HASH_KEY) ?? ''
}

/** The invite email's link: choose a first password, then straight into the app or portal. */
export function AcceptInvitePage() {
  const { t } = useTranslation()
  const postSignIn = usePostSignIn()
  const accept = useAcceptInvite()
  const [token] = useState(readTokenFromHash)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')

  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search)
    }
  }, [])

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    accept.mutate({ token, password }, { onSuccess: (me) => postSignIn(me) })
  }

  if (!token) {
    return (
      <AuthLayout title={t('invite.title')}>
        <div className={authCard}>
          <Alert tone="error">{t('invite.missingToken')}</Alert>
        </div>
      </AuthLayout>
    )
  }

  const invalidToken = accept.error instanceof ApiError && accept.error.code === ErrorCode.InvalidToken

  return (
    <AuthLayout title={t('invite.title')} intro={t('invite.intro')}>
      <Form onSubmit={onSubmit} className={authCard}>
        {accept.error && <Alert tone="error">{invalidToken ? t('invite.invalidToken') : t('common.errorGeneric')}</Alert>}
        <TextInput
          name="password"
          type="password"
          autoComplete="new-password"
          isRequired
          minLength={MIN_PASSWORD_LENGTH}
          maxLength={MAX_PASSWORD_LENGTH}
          label={t('invite.password')}
          description={t('reset.hint', { min: MIN_PASSWORD_LENGTH })}
          value={password}
          onChange={setPassword}
          validate={(v) => (v.length < MIN_PASSWORD_LENGTH ? t('reset.errors.tooShort', { min: MIN_PASSWORD_LENGTH }) : null)}
        />
        <TextInput
          name="confirm"
          type="password"
          autoComplete="new-password"
          isRequired
          label={t('invite.confirm')}
          value={confirm}
          onChange={setConfirm}
          validate={(v) => (v !== password ? t('reset.errors.mismatch') : null)}
        />
        <Button type="submit" size="lg" isPending={accept.isPending} className="w-full">
          {t('invite.submit')}
        </Button>
      </Form>
    </AuthLayout>
  )
}
