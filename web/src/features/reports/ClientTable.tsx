import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { SlaReportClient } from '../../api/client'
import { Button } from '../../components/Button'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { reportSpan } from './format'
import { TargetBar } from './TargetBar'

type ClientTableProps = {
  clients: ReadonlyArray<SlaReportClient>
  target: number
  /** The client whose PDF is being made, if any. */
  downloading: string | null
  onDownload: (client: SlaReportClient) => void
}

/** One row per client (Reports mock), with each client's own PDF to send them. */
export function ClientTable({ clients, target, downloading, onDownload }: ClientTableProps) {
  const { t } = useTranslation()
  return (
    <Table caption={t('reports.table.caption')} minWidth="min-w-(--report-table-min-w)">
      <thead>
        <tr>
          <Th>{t('reports.table.client')}</Th>
          <Th>{t('reports.table.tickets')}</Th>
          <Th>{t('reports.table.split')}</Th>
          <Th>{t('reports.table.avgFirstResponse')}</Th>
          <Th className="w-[30%]">{t('reports.table.within')}</Th>
          <Th>{t('reports.table.breaches')}</Th>
          <Th>
            <span className="sr-only">{t('reports.table.report')}</span>
          </Th>
        </tr>
      </thead>
      <tbody>
        {clients.map((c) => {
          const f = c.figures
          return (
            <tr key={c.id}>
              <RowHeader>{c.name}</RowHeader>
              <Td className="tabular-nums">{f.tickets}</Td>
              <Td className="text-ink-2 tabular-nums">
                {t('reports.table.splitValue', { incidents: f.incidents, requests: f.requests })}
              </Td>
              <Td className={f.avgFirstResponseMs == null ? 'text-muted' : 'tabular-nums'}>
                {f.avgFirstResponseMs == null ? t('common.none') : reportSpan(t, f.avgFirstResponseMs)}
              </Td>
              <Td>
                {f.withinSlaPercent == null ? (
                  <span className="text-muted">{t('reports.noTickets')}</span>
                ) : (
                  <TargetBar percent={f.withinSlaPercent} target={target} />
                )}
              </Td>
              <Td className="tabular-nums">{f.breaches}</Td>
              <Td className="text-right">
                {/* Pulled up so the 36px button centres on the row's text line. */}
                <Button
                  variant="outline"
                  size="sm"
                  className="-my-2"
                  icon={<Download aria-hidden="true" className="size-(--icon-md)" />}
                  aria-label={t('reports.downloadPdfFor', { client: c.name })}
                  isPending={downloading === c.id}
                  onPress={() => onDownload(c)}
                >
                  {t('reports.downloadPdf')}
                </Button>
              </Td>
            </tr>
          )
        })}
      </tbody>
    </Table>
  )
}
