import { useTranslation } from 'react-i18next'

import { Card } from '../../../components/Card'
import { RowHeader, Table, Td, Th } from '../../../components/Table'

/**
 * The rows of the rules table. The mock's keyword, pull/merge request and reopen rows went when
 * Git stopped moving tickets (owner, 17 Nov).
 */
const RULES = ['link', 'branch', 'words'] as const

/** Rows whose example is described in words rather than quoted as a commit message. */
const PROSE_EXAMPLES: ReadonlySet<(typeof RULES)[number]> = new Set(['branch'])

/** "Commit message rules" (Integrations mock): what a commit does, with examples under the install's ticket prefix. */
export function CommitRules({ prefix, className }: { prefix: string; className?: string }) {
  const { t } = useTranslation()
  return (
    <Card
      as="h3"
      title={t('settings.integrations.rules.title')}
      description={t('settings.integrations.rules.description')}
      className={className}
    >
      <Table caption={t('settings.integrations.rules.title')} minWidth="min-w-(--rules-table-min-w)">
        <thead>
          <tr>
            <Th>{t('settings.integrations.rules.cols.contains')}</Th>
            <Th>{t('settings.integrations.rules.cols.example')}</Th>
            <Th>{t('settings.integrations.rules.cols.effect')}</Th>
          </tr>
        </thead>
        <tbody className="text-sm">
          {RULES.map((rule) => (
            <tr key={rule}>
              <RowHeader>{t(`settings.integrations.rules.${rule}.contains`)}</RowHeader>
              <Td>
                {PROSE_EXAMPLES.has(rule) ? (
                  t(`settings.integrations.rules.${rule}.example`)
                ) : (
                  <code className="font-mono text-sm">{t(`settings.integrations.rules.${rule}.example`, { prefix })}</code>
                )}
              </Td>
              <Td>{t(`settings.integrations.rules.${rule}.effect`)}</Td>
            </tr>
          ))}
        </tbody>
      </Table>
    </Card>
  )
}
