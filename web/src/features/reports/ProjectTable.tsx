import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { SlaReportProject } from '../../api/client'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { reportSpan } from './format'
import { TargetBar } from './TargetBar'

type ProjectTableProps = {
  projects: ReadonlyArray<SlaReportProject>
  target: number
  /** The project whose PDF is being made, if any. */
  downloading: string | null
  onDownload: (project: SlaReportProject) => void
}

/** One row per project with its client, and each project's own PDF to send to that client. */
export function ProjectTable({ projects, target, downloading, onDownload }: ProjectTableProps) {
  const { t } = useTranslation()
  return (
    <Table caption={t('reports.table.caption')} minWidth="min-w-(--report-table-min-w)">
      <thead>
        <tr>
          <Th>{t('reports.table.project')}</Th>
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
        {projects.map((p) => {
          const f = p.figures
          return (
            <tr key={p.id}>
              <RowHeader>
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  {p.name}
                  {!p.active && <Pill tone="neutral">{t('reports.hidden')}</Pill>}
                </span>
                <span className="block text-sm font-regular text-muted">{p.client.name}</span>
              </RowHeader>
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
                  aria-label={t('reports.downloadPdfFor', { project: p.name, client: p.client.name })}
                  isPending={downloading === p.id}
                  onPress={() => onDownload(p)}
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
