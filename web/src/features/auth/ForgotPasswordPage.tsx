import { Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Form } from 'react-aria-components'

import { useForgotPassword } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { paths } from '../../lib/constants'
import { AuthLayout, authCard } from './AuthLayout'

export function ForgotPasswordPage() {
  const { t } = useTranslation()
  const forgot = useForgotPassword()
  const [email, setEmail] = useState('')

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    forgot.mutate(email)
  }

  if (forgot.isSuccess) {
    return (
      <AuthLayout title={t('forgot.sentTitle')}>
        <div className={authCard}>
          {/* Same message whether or not the account exists. */}
          <Alert tone="success">{t('forgot.sent', { email })}</Alert>
          <Link to={paths.login} className="text-center text-sm font-bold">
            {t('common.backToSignIn')}
          </Link>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout title={t('forgot.title')} intro={t('forgot.intro')}>
      <Form onSubmit={onSubmit} className={authCard}>
        {forgot.isError && <Alert tone="error">{t('common.errorGeneric')}</Alert>}
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
        <Button type="submit" size="lg" isPending={forgot.isPending} className="w-full">
          {t('forgot.submit')}
        </Button>
        <Link to={paths.login} className="text-center text-sm font-bold">
          {t('common.backToSignIn')}
        </Link>
      </Form>
    </AuthLayout>
  )
}
