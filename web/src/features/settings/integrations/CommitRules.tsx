import { useTranslation } from 'react-i18next'

import { Card } from '../../../components/Card'
import { RowHeader, Table, Td, Th } from '../../../components/Table'

/** The rows of the rules table, in the mock's order. */
const RULES = ['link', 'fix', 'opened', 'merged', 'reopen'] as const

/** "Commit message rules" (Integrations mock): what a message or event does, with examples under the install's ticket prefix. */
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
                {rule === 'merged' ? (
                  t('settings.integrations.rules.merged.example')
                ) : (
                  <code className="font-mono text-sm">{t(`settings.integrations.rules.${rule}.example`, { prefix })}</code>
                )}
              </Td>
              <Td>{t(`settings.integrations.rules.${rule}.effect`)}</Td>
            </tr>
          ))}
        </tbody>
      </Table>
      <p className="text-sm text-muted">{t('settings.integrations.rules.runs')}</p>
    </Card>
  )
}
