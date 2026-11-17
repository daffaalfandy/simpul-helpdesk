import { useSuspenseQuery } from '@tanstack/react-query'
import { KeyRound, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { CreatedPersonalToken, PersonalToken } from '../../api/client'
import { organizationQueryOptions } from '../../api/queries'
import { Alert } from '../../components/Alert'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { CopyField } from '../../components/CopyField'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { useTimestamp } from '../tickets/time'
import { tokensQueryOptions } from './queries'
import { CreateTokenDialog, RevokeTokenDialog, TokenCreatedDialog } from './TokenDialogs'
import { snippetsFor } from './snippets'

/** What the page is doing: one dialog at a time. */
type Action = { kind: 'create' } | { kind: 'created'; result: CreatedPersonalToken } | { kind: 'revoke'; token: PersonalToken }

/**
 * MCP access (owner, 17 Nov): staff make personal access tokens so an AI client (Claude Code,
 * Cursor, …) can read and work tickets as them, and copy the settings that client needs.
 */
export function McpAccessPage() {
  const { t } = useTranslation()
  const { data } = useSuspenseQuery(tokensQueryOptions)
  const { data: org } = useSuspenseQuery(organizationQueryOptions)
  const [action, setAction] = useState<Action | null>(null)
  const [flash, setFlash] = useState<string | null>(null)
  const start = (a: Action) => {
    setFlash(null)
    setAction(a)
  }
  const close = () => setAction(null)

  return (
    <div className="flex flex-col gap-4.5">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-extrabold tracking-title">{t('account.mcp.title')}</h1>
        <p className="max-w-3xl text-base text-muted">{t('account.mcp.intro', { org: org.name })}</p>
      </div>
      {flash && <Alert tone="success">{flash}</Alert>}
      <TokensCard tokens={data.items} orgName={org.name} onCreate={() => start({ kind: 'create' })} onRevoke={(token) => start({ kind: 'revoke', token })} />
      <ConnectCard mcpUrl={data.mcpUrl} />

      {action?.kind === 'create' && (
        <CreateTokenDialog orgName={org.name} onClose={close} onCreated={(result) => setAction({ kind: 'created', result })} />
      )}
      {action?.kind === 'created' && <TokenCreatedDialog mcpUrl={data.mcpUrl} result={action.result} onClose={close} />}
      {action?.kind === 'revoke' && (
        <RevokeTokenDialog
          token={action.token}
          onClose={close}
          onRevoked={() => {
            close()
            setFlash(t('account.mcp.revoked', { name: action.token.name }))
          }}
        />
      )}
    </div>
  )
}

type TokensCardProps = {
  tokens: PersonalToken[]
  orgName: string
  onCreate: () => void
  onRevoke: (token: PersonalToken) => void
}

function TokensCard({ tokens, orgName, onCreate, onRevoke }: TokensCardProps) {
  const { t } = useTranslation()
  const { stamp, full } = useTimestamp()
  // When the page opened: a token that expires while it is open still reads as live until a reload.
  const [now] = useState(() => Date.now())
  return (
    <Card
      title={t('account.mcp.tokensTitle')}
      labelledBy="mcp-tokens"
      description={t('account.mcp.tokensIntro', { org: orgName })}
      actions={
        <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={onCreate}>
          {t('account.mcp.create')}
        </Button>
      }
    >
      {tokens.length === 0 ? (
        <div className="flex items-start gap-3 rounded-xl bg-surface-subtle px-4 py-3.5">
          <KeyRound aria-hidden="true" className="mt-0.5 size-(--icon-md) shrink-0 text-muted" />
          <p className="text-sm text-ink-2">{t('account.mcp.empty')}</p>
        </div>
      ) : (
        <Table caption={t('account.mcp.tokensTitle')}>
          <thead>
            <tr>
              <Th>{t('account.mcp.cols.name')}</Th>
              <Th>{t('account.mcp.cols.access')}</Th>
              <Th>{t('account.mcp.cols.lastUsed')}</Th>
              <Th>{t('account.mcp.cols.expires')}</Th>
              <Th>
                <span className="sr-only">{t('account.mcp.cols.actions')}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {tokens.map((token) => {
              const expired = new Date(token.expiresAt).getTime() <= now
              return (
                <tr key={token.id}>
                  <RowHeader>
                    <span className="flex flex-col">
                      <span className="break-words">{token.name}</span>
                      <span className="font-mono text-sm font-regular text-muted">
                        {t('account.mcp.endsWith', { lastFour: token.lastFour })}
                      </span>
                    </span>
                  </RowHeader>
                  <Td>
                    <Pill tone={token.access === 'write' ? 'info' : 'neutral'}>{t(`account.mcp.access.${token.access}.short`)}</Pill>
                  </Td>
                  <Td className="text-sm text-ink-2">{token.lastUsedAt ? stamp(token.lastUsedAt) : t('account.mcp.neverUsed')}</Td>
                  <Td className="text-sm text-ink-2">
                    {expired ? <Pill tone="danger">{t('account.mcp.expired')}</Pill> : full(token.expiresAt)}
                  </Td>
                  <Td className="text-right">
                    <Button variant="outline" size="sm" onPress={() => onRevoke(token)} aria-label={t('account.mcp.revokeLabel', { name: token.name })}>
                      {t('account.mcp.revoke')}
                    </Button>
                  </Td>
                </tr>
              )
            })}
          </tbody>
        </Table>
      )}
    </Card>
  )
}

/** Where the client connects, with the settings to paste; the token is a placeholder here. */
function ConnectCard({ mcpUrl }: { mcpUrl: string }) {
  const { t } = useTranslation()
  const snippets = snippetsFor(mcpUrl, t('account.mcp.tokenPlaceholder'))
  return (
    <Card title={t('account.mcp.connectTitle')} labelledBy="mcp-connect" description={t('account.mcp.connectIntro')}>
      <dl className="flex flex-col gap-4">
        <CopyField label={t('account.mcp.address')} value={mcpUrl} />
        <CopyField label={t('account.mcp.claudeCode')} value={snippets.claudeCode} hint={t('account.mcp.claudeCodeHint')} />
        <CopyField label={t('account.mcp.otherClients')} value={snippets.json} hint={t('account.mcp.otherClientsHint')} block />
      </dl>
      <p className="text-sm text-muted">{t('account.mcp.actsAsYou')}</p>
    </Card>
  )
}
