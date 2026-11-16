import { useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleAlert, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { GitProvider, Repository, RepositoryWithSecret } from '../../../api/client'
import { Alert } from '../../../components/Alert'
import { Pill, type PillTone } from '../../../components/Badges'
import { Button } from '../../../components/Button'
import { Card } from '../../../components/Card'
import { useCodeLabels } from '../../code/labels'
import { useTimestamp } from '../../tickets/time'
import { SectionHeader } from '../shared'
import { CommitRules } from './CommitRules'
import { CommitTester } from './CommitTester'
import { ExternalLink } from '../../../components/ExternalLink'
import { integrationsQueryOptions } from './queries'
import { RecentEvents } from './RecentEvents'
import { AddRepositoryDialog, EditProjectsDialog, RemoveRepositoryDialog, RotateSecretDialog, SetupDialog } from './RepositoryDialogs'

const PROVIDERS: readonly GitProvider[] = ['github', 'gitlab']

/** Hosts whose name goes without saying next to a repository. */
const PUBLIC_HOSTS: Record<GitProvider, string> = { github: 'github.com', gitlab: 'gitlab.com' }

type ProviderStatus = 'notSetUp' | 'connected' | 'waiting' | 'failing'

const statusTones: Record<ProviderStatus, PillTone> = {
  notSetUp: 'neutral',
  connected: 'success',
  waiting: 'warning',
  failing: 'danger',
}

/** A provider card's status: the worst of its repositories'. */
function providerStatus(repos: Repository[]): ProviderStatus {
  if (repos.length === 0) return 'notSetUp'
  if (repos.some((r) => r.status === 'failing')) return 'failing'
  if (repos.some((r) => r.status === 'waiting')) return 'waiting'
  return 'connected'
}

/** What the page is doing with a repository: one dialog at a time. */
type Action =
  | { kind: 'add'; provider: GitProvider }
  | { kind: 'projects' | 'rotate' | 'remove'; repo: Repository }
  | { kind: 'setup'; result: RepositoryWithSecret; created: boolean }

/** Settings › GitHub / GitLab (Integrations mock): connected repositories, the commit message rules, the tester and recent events. */
export function IntegrationsPage() {
  const { t } = useTranslation()
  const { data } = useSuspenseQuery(integrationsQueryOptions)
  const [action, setAction] = useState<Action | null>(null)
  const [flash, setFlash] = useState<string | null>(null)

  const start = (a: Action) => {
    setFlash(null)
    setAction(a)
  }
  const close = () => setAction(null)
  const done = (message: string) => {
    setAction(null)
    setFlash(message)
  }

  return (
    <>
      <SectionHeader section="integrations" />
      {flash && <Alert tone="success">{flash}</Alert>}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--provider-card-min-w)),1fr))] gap-4">
        {PROVIDERS.map((p) => (
          <ProviderCard
            key={p}
            provider={p}
            repos={data.repositories.filter((r) => r.provider === p)}
            onAction={start}
          />
        ))}
      </div>
      <div className="flex flex-wrap items-start gap-4">
        <CommitRules prefix={data.ticketPrefix} className="min-w-0 flex-[999_1_var(--rules-card-basis)]" />
        <CommitTester prefix={data.ticketPrefix} className="min-w-0 flex-[1_1_var(--tester-card-basis)]" />
      </div>
      <RecentEvents />

      {action?.kind === 'add' && (
        <AddRepositoryDialog
          provider={action.provider}
          onClose={close}
          onCreated={(result) => setAction({ kind: 'setup', result, created: true })}
        />
      )}
      {action?.kind === 'projects' && (
        <EditProjectsDialog
          repo={action.repo}
          onClose={close}
          onDone={() => done(t('settings.integrations.projectsSaved', { name: action.repo.fullName }))}
        />
      )}
      {action?.kind === 'rotate' && (
        <RotateSecretDialog
          repo={action.repo}
          onClose={close}
          onRotated={(result) => setAction({ kind: 'setup', result, created: false })}
        />
      )}
      {action?.kind === 'remove' && (
        <RemoveRepositoryDialog
          repo={action.repo}
          onClose={close}
          onDone={() => done(t('settings.integrations.removed', { name: action.repo.fullName }))}
        />
      )}
      {action?.kind === 'setup' && (
        <SetupDialog
          result={action.result}
          created={action.created}
          onClose={() =>
            done(
              t(action.created ? 'settings.integrations.added' : 'settings.integrations.secretSaved', {
                name: action.result.repository.fullName,
              }),
            )
          }
        />
      )}
    </>
  )
}

function ProviderCard({
  provider,
  repos,
  onAction,
}: {
  provider: GitProvider
  repos: Repository[]
  onAction: (a: Action) => void
}) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const status = providerStatus(repos)
  return (
    <Card
      as="h3"
      title={labels.provider(provider)}
      description={t(`settings.integrations.how.${provider}`)}
      actions={<Pill tone={statusTones[status]}>{t(`settings.integrations.providerStatus.${status}`)}</Pill>}
    >
      {repos.length === 0 ? (
        <p className="text-sm text-muted">{t('settings.integrations.noRepos', { provider: labels.provider(provider) })}</p>
      ) : (
        <ul className="flex flex-col divide-y divide-border">
          {repos.map((r) => (
            <RepositoryRow key={r.id} repo={r} onAction={onAction} />
          ))}
        </ul>
      )}
      <div>
        <Button
          variant="outline"
          size="sm"
          icon={<Plus aria-hidden="true" className="size-(--icon-sm)" />}
          onPress={() => onAction({ kind: 'add', provider })}
          aria-label={t('settings.integrations.addFor', { provider: labels.provider(provider) })}
        >
          {t('settings.integrations.add')}
        </Button>
      </div>
    </Card>
  )
}

function RepositoryRow({ repo, onAction }: { repo: Repository; onAction: (a: Action) => void }) {
  const { t } = useTranslation()
  const name = repo.host === PUBLIC_HOSTS[repo.provider] ? repo.fullName : `${repo.host}/${repo.fullName}`
  return (
    <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <ExternalLink href={repo.webUrl} className="font-bold break-all text-ink">
          {name}
        </ExternalLink>
        <DeliveryStatus repo={repo} />
      </div>
      <p className="text-sm text-ink-2">
        {repo.projects.length === 0 ? (
          <span className="text-muted">{t('settings.integrations.noProjects')}</span>
        ) : (
          <>
            <span className="sr-only">{t('settings.integrations.projectsLabel')} </span>
            {repo.projects.map((p, i) => (
              <span key={p.id}>
                {i > 0 && t('common.separator')}
                <Link
                  to="/projects/$projectId"
                  params={{ projectId: p.id }}
                  className="font-bold text-primary hover:text-primary-hover"
                >
                  {p.name}
                </Link>{' '}
                <span className="text-muted">{t('settings.integrations.projectClient', { client: p.client.name })}</span>
              </span>
            ))}
          </>
        )}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          onPress={() => onAction({ kind: 'projects', repo })}
          aria-label={t('settings.integrations.editProjectsLabel', { name: repo.fullName })}
        >
          {t('settings.integrations.editProjects')}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onPress={() => onAction({ kind: 'rotate', repo })}
          aria-label={t('settings.integrations.newSecretLabel', { name: repo.fullName })}
        >
          {t('settings.integrations.newSecret')}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onPress={() => onAction({ kind: 'remove', repo })}
          aria-label={t('settings.integrations.removeLabel', { name: repo.fullName })}
        >
          {t('settings.integrations.remove')}
        </Button>
      </div>
    </li>
  )
}

/** When the repository last sent a webhook, or why the latest one was refused. */
function DeliveryStatus({ repo }: { repo: Repository }) {
  const { t } = useTranslation()
  const { stamp } = useTimestamp()
  if (repo.status === 'failing' && repo.lastErrorAt && repo.lastError) {
    return (
      <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-chip-danger-fg">
        <CircleAlert aria-hidden="true" className="size-(--icon-sm) shrink-0" />
        {t('settings.integrations.refused', {
          time: stamp(repo.lastErrorAt),
          reason: t(`settings.integrations.errors.${repo.lastError}`),
        })}
      </span>
    )
  }
  if (repo.status === 'connected' && repo.lastDeliveryAt) {
    return <span className="text-sm text-muted">{t('settings.integrations.lastEvent', { time: stamp(repo.lastDeliveryAt) })}</span>
  }
  return <span className="text-sm text-muted">{t('settings.integrations.waiting')}</span>
}
