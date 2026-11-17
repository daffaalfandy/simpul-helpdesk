import { useNavigate } from '@tanstack/react-router'
import { Button as AriaButton } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { useLogout, useSwitchOrganization } from '../../api/queries'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { OrgLogo } from '../../components/Logo'
import { Spinner } from '../../components/Spinner'
import { paths } from '../../lib/constants'
import { homePathFor } from '../../lib/roles'
import { safeRedirect } from '../../lib/text'
import { useSignedInUser } from '../shell/useSignedInUser'
import { AuthLayout, authCard } from './AuthLayout'

/**
 * After sign-in, people who work in several organizations choose where this tab works. One
 * press picks it: there is nothing else to fill in.
 */
export function OrganizationPicker({ redirectTo }: { redirectTo?: string }) {
  const { t } = useTranslation()
  const me = useSignedInUser()
  const navigate = useNavigate()
  const switchTo = useSwitchOrganization()
  const logout = useLogout()

  const pick = (organizationId: string) =>
    switchTo.mutate(organizationId, {
      onSuccess: (next) => void navigate({ href: safeRedirect(redirectTo) ?? homePathFor(next.role), replace: true }),
    })
  const signOut = () => logout.mutate(undefined, { onSettled: () => void navigate({ to: paths.login, replace: true }) })

  return (
    <AuthLayout title={t('organizations.pick.title')} intro={t('organizations.pick.intro', { count: me.organizations.length })}>
      <div className={authCard}>
        <ErrorAlert error={switchTo.error} />
        <ul aria-label={t('organizations.pick.label')} className="flex flex-col gap-2">
          {me.organizations.map((o) => (
            <li key={o.id}>
              <AriaButton
                onPress={() => pick(o.id)}
                aria-label={t('organizations.option', { org: o.name, role: t(`roles.${o.role}`) })}
                isDisabled={switchTo.isPending}
                className="flex w-full cursor-pointer items-center gap-3 rounded-xl border-2 border-border bg-surface p-3 text-left text-ink outline-none transition-colors duration-(--duration-fast) data-disabled:cursor-default data-focus-visible:shadow-(--focus-ring) data-hovered:border-border-strong data-pressed:border-primary data-pressed:bg-primary-tint"
              >
                <OrgLogo org={o} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-base font-bold">{o.name}</span>
                  <span className="text-sm text-muted">{t(`roles.${o.role}`)}</span>
                </span>
                {switchTo.isPending && switchTo.variables === o.id && <Spinner className="text-primary" />}
              </AriaButton>
            </li>
          ))}
        </ul>
      </div>
      <Button variant="ghost" className="self-center" onPress={signOut} isPending={logout.isPending}>
        {t('organizations.pick.signOut')}
      </Button>
    </AuthLayout>
  )
}
