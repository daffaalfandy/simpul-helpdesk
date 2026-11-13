import { CalendarDate, parseDate, parseTime, Time } from '@internationalized/date'
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Button as AriaButton } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { BusinessHours, Holiday, Weekday } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox, CheckboxGroup } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { DateInput, TimeInput } from '../../components/Fields'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { HOLIDAY_YEARS_AROUND, MINUTES_PER_DAY, MINUTES_PER_HOUR } from '../../lib/constants'
import { currentYear, formatLongDate } from '../../lib/dates'
import { closedDays, WEEK, weekdayRuns } from '../../lib/weekdays'
import {
  businessHoursQueryOptions,
  holidayPresetsQueryOptions,
  holidaysQueryOptions,
  useCreateHoliday,
  useDeleteHoliday,
  useLoadHolidayPreset,
  useUpdateBusinessHours,
} from './queries'
import { ErrorAlert, SectionHeader } from './shared'

/** The API writes the end of the day as "24:00"; a time field shows it as 00:00. */
const END_OF_DAY = '24:00'
const MIDNIGHT = new Time(0, 0)
/** "HH:MM" */
const CLOCK_LENGTH = END_OF_DAY.length

function clockText(time: Time): string {
  return time.toString().slice(0, CLOCK_LENGTH)
}

/** Minutes from midnight; a closing time of 00:00 means the end of the day. */
function minutesOf(time: Time, isClose: boolean): number {
  const m = time.hour * MINUTES_PER_HOUR + time.minute
  return isClose && m === 0 ? MINUTES_PER_DAY : m
}

export function GeneralPage() {
  const [flash, setFlash] = useState<string | null>(null)
  return (
    <>
      <SectionHeader section="general" />
      {flash && <Alert tone="success">{flash}</Alert>}
      <BusinessHoursCard onFlash={setFlash} />
      <HolidaysCard onFlash={setFlash} />
    </>
  )
}

type CardProps = { onFlash: (message: string | null) => void }

function BusinessHoursCard({ onFlash }: CardProps) {
  const { t } = useTranslation()
  const { data: hours } = useSuspenseQuery(businessHoursQueryOptions)
  const year = currentYear()
  const { data: holidays } = useQuery(holidaysQueryOptions(year))
  const [editing, setEditing] = useState(false)

  const dayLabel = (run: { first: Weekday; last: Weekday }) =>
    run.first === run.last
      ? t(`weekday.short.${run.first}`)
      : t('common.range', { from: t(`weekday.short.${run.first}`), to: t(`weekday.short.${run.last}`) })

  return (
    <Card
      as="h3"
      title={t('settings.hours.title')}
      description={t('settings.hours.helper')}
    >
      <dl className="grid grid-cols-2 gap-2.5 text-base">
        {weekdayRuns(hours.weekdays).map((run) => (
          <div key={run.first} className="contents">
            <dt className="font-semibold text-muted">{dayLabel(run)}</dt>
            <dd className="font-bold tabular-nums">{t('common.range', { from: hours.opens, to: hours.closes })}</dd>
          </div>
        ))}
        {weekdayRuns(closedDays(hours.weekdays)).map((run) => (
          <div key={run.first} className="contents">
            <dt className="font-semibold text-muted">{dayLabel(run)}</dt>
            <dd className="font-bold">{t('settings.hours.closed')}</dd>
          </div>
        ))}
        <dt className="font-semibold text-muted">{t('settings.hours.holidays')}</dt>
        <dd className="font-bold">{t('settings.hours.holidayCount', { count: holidays?.length ?? 0, year })}</dd>
      </dl>
      <p className="text-xs text-muted">{t('settings.hours.timezone', { timezone: hours.timezone })}</p>
      <Button
        variant="outline"
        className="self-start"
        onPress={() => {
          onFlash(null)
          setEditing(true)
        }}
      >
        {t('settings.hours.edit')}
      </Button>
      {editing && (
        <HoursDialog
          hours={hours}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false)
            onFlash(t('settings.hours.saved'))
          }}
        />
      )}
    </Card>
  )
}

function HoursDialog({ hours, onClose, onSaved }: { hours: BusinessHours; onClose: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const save = useUpdateBusinessHours()
  const [days, setDays] = useState<string[]>(hours.weekdays)
  const [opens, setOpens] = useState<Time | null>(parseTime(hours.opens))
  const [closes, setCloses] = useState<Time | null>(hours.closes === END_OF_DAY ? MIDNIGHT : parseTime(hours.closes))
  const [submitted, setSubmitted] = useState(false)

  const openMin = opens ? minutesOf(opens, false) : null
  const closeMin = closes ? minutesOf(closes, true) : null
  const rangeOk = openMin !== null && closeMin !== null && openMin < closeMin
  const length = rangeOk ? closeMin - openMin : null
  const lengthText =
    length === null
      ? t('common.none')
      : length % MINUTES_PER_HOUR === 0
        ? t('settings.hours.dayLength', { count: length / MINUTES_PER_HOUR })
        : t('settings.hours.dayLengthMinutes', {
            hours: Math.floor(length / MINUTES_PER_HOUR),
            minutes: length % MINUTES_PER_HOUR,
          })

  const submit = () => {
    setSubmitted(true)
    if (days.length === 0 || !rangeOk || !opens || !closes) return
    save.mutate(
      {
        weekdays: WEEK.filter((d) => days.includes(d)),
        opens: clockText(opens),
        closes: closeMin === MINUTES_PER_DAY ? END_OF_DAY : clockText(closes),
      },
      { onSuccess: onSaved },
    )
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.hours.dialogTitle')}
      subtitle={t('settings.hours.timezone', { timezone: hours.timezone })}
      submitLabel={t('common.save')}
      onSubmit={submit}
      isPending={save.isPending}
      error={<ErrorAlert error={save.error} />}
    >
      <CheckboxGroup
        label={t('settings.hours.days')}
        value={days}
        onChange={setDays}
        inline
        isInvalid={submitted && days.length === 0}
      >
        {WEEK.map((d) => (
          <Checkbox key={d} value={d}>
            {t(`weekday.long.${d}`)}
          </Checkbox>
        ))}
      </CheckboxGroup>
      {submitted && days.length === 0 && <p className="text-xs font-semibold text-danger">{t('settings.hours.noDays')}</p>}
      <div className="grid gap-4 sm:grid-cols-2">
        <TimeInput label={t('settings.hours.opens')} value={opens} onChange={setOpens} isRequired />
        <TimeInput label={t('settings.hours.closes')} value={closes} onChange={setCloses} isRequired />
      </div>
      {submitted && !rangeOk && (
        <p className="text-xs font-semibold text-danger">{t('settings.hours.closesBeforeOpens')}</p>
      )}
      <p className="rounded-lg bg-surface-subtle px-4 py-3 text-sm text-ink-2">
        {t('settings.hours.workingDayNote', { length: lengthText })}
      </p>
    </FormDialog>
  )
}

function HolidaysCard({ onFlash }: CardProps) {
  const { t } = useTranslation()
  const thisYear = currentYear()
  const [year, setYear] = useState(thisYear)
  const { data: presets } = useSuspenseQuery(holidayPresetsQueryOptions)
  const holidays = useQuery({ ...holidaysQueryOptions(year), placeholderData: (prev) => prev })
  const load = useLoadHolidayPreset()
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState<Holiday | null>(null)
  const remove = useDeleteHoliday()

  const years = [
    ...new Set([
      ...Array.from({ length: 2 * HOLIDAY_YEARS_AROUND + 1 }, (_, i) => thisYear - HOLIDAY_YEARS_AROUND + i),
      ...presets.map((p) => p.year),
    ]),
  ].sort((a, b) => a - b)
  const preset = presets.find((p) => p.year === year)
  const list = holidays.data ?? []

  return (
    <Card
      as="h3"
      title={t('settings.holidays.title')}
      description={t('settings.holidays.helper')}
      actions={
        <div className="flex flex-wrap items-end gap-3">
          <Dropdown
            label={t('settings.holidays.year')}
            hideLabel
            options={years.map((y) => ({ id: String(y), label: String(y) }))}
            value={String(year)}
            onChange={(y) => setYear(Number(y))}
            className="min-w-28"
          />
          <Button
            variant="outline"
            icon={<Plus aria-hidden="true" className="size-(--icon-md)" />}
            onPress={() => {
              onFlash(null)
              setAdding(true)
            }}
          >
            {t('settings.holidays.add')}
          </Button>
        </div>
      }
    >
      {preset && preset.missingCount > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-primary-tint-2 bg-primary-tint px-4 py-3">
          <p className="text-sm text-ink">{t('settings.holidays.preset', { count: preset.missingCount, year })}</p>
          <Button
            size="sm"
            isPending={load.isPending}
            onPress={() =>
              load.mutate(year, {
                onSuccess: (r) => onFlash(t('settings.holidays.loaded', { count: r.addedCount, year })),
              })
            }
          >
            {t('settings.holidays.loadPreset', { count: preset.missingCount })}
          </Button>
        </div>
      )}
      <ErrorAlert error={load.error} />
      {list.length === 0 ? (
        <p className="text-base text-muted">{t('settings.holidays.empty', { year })}</p>
      ) : (
        <Table caption={t('settings.holidays.caption', { year })} minWidth="min-w-0">
          <thead>
            <tr>
              <Th>{t('settings.holidays.colDate')}</Th>
              <Th>{t('settings.holidays.colName')}</Th>
              <Th className="text-right">
                <span className="sr-only">{t('settings.holidays.colActions')}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {list.map((h) => (
              <tr key={h.id}>
                <RowHeader className="font-semibold whitespace-nowrap tabular-nums">{formatLongDate(h.date)}</RowHeader>
                <Td>{h.name}</Td>
                <Td className="text-right">
                  <AriaButton
                    aria-label={t('settings.holidays.removeLabel', { name: h.name })}
                    onPress={() => {
                      onFlash(null)
                      setRemoving(h)
                    }}
                    className="hit-area inline-flex size-(--control-h-sm) cursor-pointer items-center justify-center rounded-md text-muted data-hovered:bg-danger-tint data-hovered:text-danger"
                  >
                    <Trash2 aria-hidden="true" className="size-(--icon-md)" />
                  </AriaButton>
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {adding && (
        <HolidayDialog
          year={year}
          onClose={() => setAdding(false)}
          onAdded={(h) => {
            setAdding(false)
            setYear(parseDate(h.date).year)
            onFlash(t('settings.holidays.added', { name: h.name }))
          }}
        />
      )}
      <ConfirmDialog
        isOpen={removing !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRemoving(null)
            remove.reset()
          }
        }}
        tone="danger"
        title={t('settings.holidays.removeTitle')}
        confirmLabel={t('settings.holidays.remove')}
        isPending={remove.isPending}
        error={<ErrorAlert error={remove.error} />}
        onConfirm={() => {
          if (!removing) return
          remove.mutate(removing.id, {
            onSuccess: () => {
              onFlash(t('settings.holidays.removed', { name: removing.name }))
              setRemoving(null)
            },
          })
        }}
      >
        {removing && t('settings.holidays.removeBody', { name: removing.name, date: formatLongDate(removing.date) })}
      </ConfirmDialog>
    </Card>
  )
}

function HolidayDialog({ year, onClose, onAdded }: { year: number; onClose: () => void; onAdded: (h: Holiday) => void }) {
  const { t } = useTranslation()
  const create = useCreateHoliday()
  const [date, setDate] = useState<CalendarDate | null>(null)
  const [name, setName] = useState('')
  const [submitted, setSubmitted] = useState(false)

  const submit = () => {
    setSubmitted(true)
    if (!date || name.trim() === '') return
    create.mutate({ date: date.toString(), name: name.trim() }, { onSuccess: onAdded })
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t('settings.holidays.dialogTitle')}
      submitLabel={t('settings.holidays.add')}
      onSubmit={submit}
      isPending={create.isPending}
      error={<ErrorAlert error={create.error} />}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <DateInput
          label={t('settings.holidays.date')}
          value={date}
          onChange={setDate}
          placeholderValue={new CalendarDate(year, 1, 1)}
          isRequired
          isInvalid={submitted && !date}
          errorMessage={t('settings.holidays.dateRequired')}
        />
        <TextInput
          label={t('settings.holidays.name')}
          value={name}
          onChange={setName}
          isRequired
          isInvalid={submitted && name.trim() === ''}
          errorMessage={t('settings.holidays.nameRequired')}
        />
      </div>
    </FormDialog>
  )
}
