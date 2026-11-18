import { useMutation, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { SlaReportProject } from '../../api/client'
import { downloadSlaExcel, downloadSlaPdf, slaReportQueryOptions } from '../../api/reports'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorAlert } from '../../components/ErrorAlert'
import { paths } from '../../lib/constants'
import { can } from '../../lib/roles'
import { useSignedInUser } from '../shell/useSignedInUser'
import { formatPercent, reportRangeLabel } from './format'
import { KpiTiles } from './KpiTiles'
import { MonthRangePicker } from './MonthRangePicker'
import type { MonthKey, MonthRange, ReportSearch } from './months'
import { ProjectTable } from './ProjectTable'

type Download = { kind: 'excel' } | { kind: 'pdf'; project: SlaReportProject }

type SlaReportPageProps = {
  /** The months in the URL; the current month when left out. */
  search: ReportSearch
  onRangeChange: (range: MonthRange, current: MonthKey) => void
}

/** SLA report (Reports mock): every project over a range of months against the contract target, with exports. */
export function SlaReportPage({ search, onRangeChange }: SlaReportPageProps) {
  const { t } = useTranslation()
  const { data: report } = useSuspenseQuery(slaReportQueryOptions(search))
  // The months the API answered for, so the files match what is on screen.
  const range: MonthRange = { from: report.from, to: report.to }
  const download = useMutation({
    mutationFn: (d: Download) => (d.kind === 'excel' ? downloadSlaExcel(range) : downloadSlaPdf(d.project.id, range)),
  })
  const pending = download.isPending ? download.variables : undefined
  const target = formatPercent(report.targetPercent)
  const me = useSignedInUser()
  const canAddProjects = can.manageProjects(me.role)

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl leading-tight font-extrabold tracking-title">{t('reports.title')}</h1>
          <p className="text-base text-muted">{t('reports.subtitle')}</p>
        </div>
        <div className="flex flex-wrap items-end gap-2">
          <MonthRangePicker
            value={range}
            current={report.currentMonth}
            oldest={report.oldestMonth}
            first={report.firstMonth}
            onChange={(r) => onRangeChange(r, report.currentMonth)}
          />
          <Button variant="outline" isPending={pending?.kind === 'excel'} onPress={() => download.mutate({ kind: 'excel' })}>
            {t('reports.downloadExcel')}
          </Button>
        </div>
      </div>
      <ErrorAlert error={download.error} />
      <KpiTiles report={report} />
      {report.projects.length === 0 ? (
        <EmptyState title={t('reports.empty.title')}>
          {t('reports.empty.body')}
          {canAddProjects && (
            <>
              {' '}
              <Link to={paths.projects} className="font-bold">
                {t('reports.empty.link')}
              </Link>
            </>
          )}
        </EmptyState>
      ) : (
        <Card
          title={t('reports.byProject', { period: reportRangeLabel(t, report.from, report.to, report.toDate) })}
          actions={<span className="text-sm text-muted">{t('reports.target', { target })}</span>}
        >
          <ProjectTable
            projects={report.projects}
            target={report.targetPercent}
            downloading={pending?.kind === 'pdf' ? pending.project.id : null}
            onDownload={(project) => download.mutate({ kind: 'pdf', project })}
          />
          <div className="flex flex-col gap-1 text-sm text-muted">
            <p>{t('reports.footnote.tick', { target })}</p>
            {report.waitingPause && <p>{t(`reports.footnote.waiting.${report.waitingPause}`)}</p>}
            <p>{t('reports.footnote.firstResponse')}</p>
          </div>
        </Card>
      )}
    </>
  )
}
