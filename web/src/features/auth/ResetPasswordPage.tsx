import { Link } from '@tanstack/react-router'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Form } from 'react-aria-components'

import { ApiError, ErrorCode } from '../../api/client'
import { useResetPassword } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { MAX_PASSWORD_LENGTH, MIN_PASSWORD_LENGTH, paths, RESET_TOKEN_HASH_KEY } from '../../lib/constants'
import { AuthLayout, authCard } from './AuthLayout'

/** The reset token travels in the URL fragment so it never reaches server logs. */
function readTokenFromHash(): string {
  return new URLSearchParams(window.location.hash.slice(1)).get(RESET_TOKEN_HASH_KEY) ?? ''
}

export function ResetPasswordPage() {
  const { t } = useTranslation()
  const reset = useResetPassword()
  const [token] = useState(readTokenFromHash)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')

  // Remove the token from the address bar and history once read.
  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search)
    }
  }, [])

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    reset.mutate({ token, password })
  }

  const requestNewLink = (
    <Link to={paths.forgotPassword} className="font-bold">
      {t('reset.errors.requestNew')}
    </Link>
  )

  if (reset.isSuccess) {
    return (
      <AuthLayout title={t('reset.doneTitle')}>
        <div className={authCard}>
          <Alert tone="success">{t('reset.done')}</Alert>
          <Link to={paths.login} className="text-center text-sm font-bold">
            {t('common.backToSignIn')}
          </Link>
        </div>
      </AuthLayout>
    )
  }

  if (!token) {
    return (
      <AuthLayout title={t('reset.title')}>
        <div className={authCard}>
          <Alert tone="error">
            {t('reset.errors.missingToken')} {requestNewLink}
          </Alert>
        </div>
      </AuthLayout>
    )
  }

  const invalidToken = reset.error instanceof ApiError && reset.error.code === ErrorCode.InvalidToken

  return (
    <AuthLayout title={t('reset.title')}>
      <Form onSubmit={onSubmit} className={authCard}>
        {reset.error &&
          (invalidToken ? (
            <Alert tone="error">
              {t('reset.errors.invalidToken')} {requestNewLink}
            </Alert>
          ) : (
            <Alert tone="error">{t('common.errorGeneric')}</Alert>
          ))}
        <TextInput
          name="password"
          type="password"
          autoComplete="new-password"
          isRequired
          minLength={MIN_PASSWORD_LENGTH}
          maxLength={MAX_PASSWORD_LENGTH}
          label={t('reset.password')}
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
          label={t('reset.confirm')}
          value={confirm}
          onChange={setConfirm}
          validate={(v) => (v !== password ? t('reset.errors.mismatch') : null)}
        />
        <Button type="submit" size="lg" isPending={reset.isPending} className="w-full">
          {t('reset.submit')}
        </Button>
      </Form>
    </AuthLayout>
  )
}
