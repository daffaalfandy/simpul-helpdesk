import { useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Form } from 'react-aria-components'

import { errorMessage } from '../../api/errors'
import { useCompleteSetup } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { TextInput } from '../../components/TextInput'
import { MAX_PASSWORD_LENGTH, MIN_PASSWORD_LENGTH, paths } from '../../lib/constants'
import { AuthLayout, authCard } from './AuthLayout'

/** Limit matching the API (users name). */
const MAX_NAME_LENGTH = 200

/**
 * First-run setup, step 1: the first administrator. It works once; the new administrator then
 * creates the first organization (owner, 17 Nov: an install always has both).
 */
export function SetupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setup = useCompleteSetup()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setup.mutate(
      { name: name.trim(), email: email.trim(), password },
      { onSuccess: () => void navigate({ to: paths.setupOrganization, replace: true }) },
    )
  }

  return (
    <AuthLayout title={t('setup.title')} intro={t('setup.intro')}>
      <Form onSubmit={onSubmit} className={authCard}>
        {setup.error && <Alert tone="error">{errorMessage(t, setup.error)}</Alert>}
        <TextInput
          name="name"
          autoComplete="name"
          isRequired
          maxLength={MAX_NAME_LENGTH}
          label={t('setup.name')}
          value={name}
          onChange={setName}
          errorMessage={t('setup.nameRequired')}
        />
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
        <Button type="submit" size="lg" isPending={setup.isPending} className="w-full">
          {t('setup.submit')}
        </Button>
      </Form>
    </AuthLayout>
  )
}
