import type { TFunction } from 'i18next'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Form } from 'react-aria-components'

import { ApiError, ErrorCode } from '../../api/client'
import { organizationQueryOptions, useLogin } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { paths, SECONDS_PER_MINUTE } from '../../lib/constants'
import { homePathFor } from '../../lib/roles'
import { safeRedirect } from '../../lib/text'
import { AuthLayout, authCard } from './AuthLayout'

export function LoginPage({ redirectTo }: { redirectTo?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: org } = useQuery(organizationQueryOptions)
  const login = useLogin()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    login.mutate(
      { email, password },
      {
        onSuccess: (me) => void navigate({ href: safeRedirect(redirectTo) ?? homePathFor(me.role), replace: true }),
      },
    )
  }

  return (
    <AuthLayout title={t('login.title', { org: org?.name ?? '' })}>
      <Form onSubmit={onSubmit} className={authCard}>
        {login.error && <Alert tone="error">{loginErrorMessage(login.error, t)}</Alert>}
        <TextInput
          name="email"
          type="email"
          autoComplete="username"
          isRequired
          label={t('login.email')}
          placeholder={t('login.emailPlaceholder')}
          value={email}
          onChange={setEmail}
          errorMessage={(v) => (v.validationDetails.valueMissing ? t('login.errors.emailRequired') : t('login.errors.emailInvalid'))}
        />
        <TextInput
          name="password"
          type="password"
          autoComplete="current-password"
          isRequired
          label={t('login.password')}
          labelAction={
            <Link to={paths.forgotPassword} className="text-sm font-bold">
              {t('login.forgot')}
            </Link>
          }
          value={password}
          onChange={setPassword}
          errorMessage={t('login.errors.passwordRequired')}
        />
        <Button type="submit" size="lg" isPending={login.isPending} className="w-full">
          {t('login.submit')}
        </Button>
        <p className="text-center text-xs leading-normal text-muted">{t('login.helper')}</p>
      </Form>
      <p className="text-center text-sm text-muted">{t('login.noAccount')}</p>
    </AuthLayout>
  )
}

function loginErrorMessage(error: Error, t: TFunction): string {
  if (error instanceof ApiError) {
    if (error.code === ErrorCode.InvalidCredentials) return t('login.errors.invalidCredentials')
    if (error.code === ErrorCode.RateLimited) {
      const minutes = Math.max(1, Math.ceil((error.retryAfterSeconds ?? SECONDS_PER_MINUTE) / SECONDS_PER_MINUTE))
      return t('login.errors.rateLimited', { count: minutes })
    }
  }
  return t('common.errorGeneric')
}
