import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { meQueryOptions, useLogout } from '../../api/queries'
import { Button } from '../../components/Button'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths } from '../../lib/constants'
import { homePathFor } from '../../lib/roles'
import { AuthLayout, authCard } from './AuthLayout'

/**
 * Staff whose account belongs to no organization (never added, or taken off their last one).
 * They can only wait for an administrator, check again, or sign out.
 */
export function NoOrganizationPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const logout = useLogout()
  const check = useMutation({
    mutationFn: () => queryClient.fetchQuery({ ...meQueryOptions, staleTime: 0 }),
    onSuccess: (me) => {
      if (me?.organization) void navigate({ to: homePathFor(me.role), replace: true })
    },
  })
  const signOut = () => logout.mutate(undefined, { onSettled: () => void navigate({ to: paths.login, replace: true }) })
  const stillNone = check.isSuccess && !check.data?.organization

  return (
    <AuthLayout title={t('organizations.none.title')} intro={t('organizations.none.intro')}>
      <div className={authCard}>
        <ErrorAlert error={check.error} />
        {stillNone && (
          <p role="status" className="text-sm text-muted">
            {t('organizations.none.still')}
          </p>
        )}
        <Button size="lg" className="w-full" onPress={() => check.mutate()} isPending={check.isPending}>
          {t('organizations.none.check')}
        </Button>
        <Button variant="outline" size="lg" className="w-full" onPress={signOut} isPending={logout.isPending}>
          {t('organizations.none.signOut')}
        </Button>
      </div>
    </AuthLayout>
  )
}
