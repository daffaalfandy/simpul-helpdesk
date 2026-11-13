import { useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { CatalogItem, System } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { Dropdown } from '../../components/Dropdown'
import { NumberInput } from '../../components/Fields'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { fromParts, toParts, unitsFor, type DurationUnit } from '../../lib/duration'
import { formatDuration } from '../../lib/formatDuration'
import {
  businessHoursQueryOptions,
  catalogItemsQueryOptions,
  systemsQueryOptions,
  useDeleteCatalogItem,
  useDeleteSystem,
  useSaveCatalogItem,
  useSaveSystem,
} from './queries'
import { ErrorAlert, SectionHeader } from './shared'

/** Catalogue delivery times are working time (they feed business-hours SLA targets). */
const CALENDAR = 'business_hours'
/** New entries go after the existing ones. */
const SORT_STEP = 10
const MIN_DELIVERY = 1

type Kind = 'system' | 'item'
type Editing = { kind: 'system'; entry: System | null } | { kind: 'item'; entry: CatalogItem | null }

/** Settings › Service desk › Systems & catalogue: the report form's two lists. */
export function CatalogPage() {
  const { t } = useTranslation()
  const { data: systems } = useSuspenseQuery(systemsQueryOptions)
  const { data: items } = useSuspenseQuery(catalogItemsQueryOptions)
  const { data: hours } = useSuspenseQuery(businessHoursQueryOptions)
  const [flash, setFlash] = useState<string | null>(null)
  const [editing, setEditing] = useState<Editing | null>(null)
  const open = (e: Editing) => {
    setFlash(null)
    setEditing(e)
  }
  const nextSort = (list: ReadonlyArray<{ sortOrder: number }>) =>
    list.reduce((max, x) => Math.max(max, x.sortOrder), 0) + SORT_STEP

  return (
    <>
      <SectionHeader section="catalog" />
      {flash && <Alert tone="success">{flash}</Alert>}
      <Card
        as="h3"
        title={t('settings.catalog.systems.title')}
        description={t('settings.catalog.systems.description')}
        actions={
          <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => open({ kind: 'system', entry: null })}>
            {t('settings.catalog.systems.add')}
          </Button>
        }
      >
        {systems.length === 0 ? (
          <p className="text-sm text-muted">{t('settings.catalog.systems.empty')}</p>
        ) : (
          <Table caption={t('settings.catalog.systems.caption')}>
            <thead>
              <tr>
                <Th>{t('settings.catalog.cols.name')}</Th>
                <Th>{t('settings.catalog.cols.description')}</Th>
                <Th>{t('settings.catalog.cols.shown')}</Th>
                <Th className="text-right">{t('settings.catalog.cols.tickets')}</Th>
                <Th>
                  <span className="sr-only">{t('settings.catalog.cols.actions')}</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {systems.map((s) => (
                <tr key={s.id}>
                  <RowHeader>{s.name}</RowHeader>
                  <Td className="text-muted">{s.description ?? t('common.none')}</Td>
                  <Td>
                    <ShownPill active={s.active} />
                  </Td>
                  <Td className="text-right tabular-nums">{s.ticketCount}</Td>
                  <Td className="text-right">
                    <EditButton name={s.name} onPress={() => open({ kind: 'system', entry: s })} />
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
      <Card
        as="h3"
        title={t('settings.catalog.items.title')}
        description={t('settings.catalog.items.description')}
        actions={
          <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => open({ kind: 'item', entry: null })}>
            {t('settings.catalog.items.add')}
          </Button>
        }
      >
        {items.length === 0 ? (
          <p className="text-sm text-muted">{t('settings.catalog.items.empty')}</p>
        ) : (
          <Table caption={t('settings.catalog.items.caption')}>
            <thead>
              <tr>
                <Th>{t('settings.catalog.cols.name')}</Th>
                <Th>{t('settings.catalog.cols.delivery')}</Th>
                <Th>{t('settings.catalog.cols.shown')}</Th>
                <Th className="text-right">{t('settings.catalog.cols.tickets')}</Th>
                <Th>
                  <span className="sr-only">{t('settings.catalog.cols.actions')}</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {items.map((i) => (
                <tr key={i.id}>
                  <RowHeader>
                    <span className="flex flex-col">
                      <span>{i.name}</span>
                      {i.description && <span className="text-sm font-normal text-muted">{i.description}</span>}
                    </span>
                  </RowHeader>
                  <Td className="whitespace-nowrap">{formatDuration(t, i.deliveryMinutes, CALENDAR, hours.workingDayMinutes)}</Td>
                  <Td>
                    <ShownPill active={i.active} />
                  </Td>
                  <Td className="text-right tabular-nums">{i.ticketCount}</Td>
                  <Td className="text-right">
                    <EditButton name={i.name} onPress={() => open({ kind: 'item', entry: i })} />
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
      {editing && (
        <EntryDialog
          editing={editing}
          nextSortOrder={nextSort(editing.kind === 'system' ? systems : items)}
          workingDayMinutes={hours.workingDayMinutes}
          onClose={() => setEditing(null)}
          onDone={(message) => {
            setEditing(null)
            setFlash(message)
          }}
        />
      )}
    </>
  )
}

function ShownPill({ active }: { active: boolean }) {
  const { t } = useTranslation()
  return (
    <Pill tone={active ? 'success' : 'neutral'}>{active ? t('settings.catalog.shownYes') : t('settings.catalog.shownNo')}</Pill>
  )
}

function EditButton({ name, onPress }: { name: string; onPress: () => void }) {
  const { t } = useTranslation()
  return (
    <Button variant="outline" size="sm" onPress={onPress} aria-label={t('settings.catalog.editLabel', { name })}>
      {t('common.edit')}
    </Button>
  )
}

type EntryDialogProps = {
  editing: Editing
  nextSortOrder: number
  workingDayMinutes: number
  onClose: () => void
  onDone: (message: string) => void
}

function EntryDialog({ editing, nextSortOrder, workingDayMinutes, onClose, onDone }: EntryDialogProps) {
  const { t } = useTranslation()
  const kind: Kind = editing.kind
  const entry = editing.entry
  const saveSystem = useSaveSystem()
  const saveItem = useSaveCatalogItem()
  const deleteSystem = useDeleteSystem()
  const deleteItem = useDeleteCatalogItem()
  const save = kind === 'system' ? saveSystem : saveItem
  const remove = kind === 'system' ? deleteSystem : deleteItem

  const initialParts = toParts(
    editing.kind === 'item' && editing.entry ? editing.entry.deliveryMinutes : workingDayMinutes,
    CALENDAR,
    workingDayMinutes,
  )
  const [name, setName] = useState(entry?.name ?? '')
  const [description, setDescription] = useState(entry?.description ?? '')
  const [sortOrder, setSortOrder] = useState(entry?.sortOrder ?? nextSortOrder)
  const [active, setActive] = useState(entry?.active ?? true)
  const [amount, setAmount] = useState(initialParts.value)
  const [unit, setUnit] = useState<DurationUnit>(initialParts.unit)
  const [submitted, setSubmitted] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const nameMissing = name.trim() === ''
  const deliveryInvalid = kind === 'item' && !(amount >= MIN_DELIVERY)
  const titleKey = kind === 'system' ? 'systems' : 'items'

  const submit = () => {
    setSubmitted(true)
    if (nameMissing || deliveryInvalid) return
    const base = { name: name.trim(), description: description.trim() || null, sortOrder, active }
    const done = (saved: { name: string }) =>
      onDone(t(entry ? 'settings.catalog.saved' : 'settings.catalog.created', { name: saved.name }))
    if (kind === 'system') {
      saveSystem.mutate({ id: entry?.id, input: base }, { onSuccess: done })
    } else {
      const deliveryMinutes = fromParts({ value: amount, unit }, workingDayMinutes)
      saveItem.mutate({ id: entry?.id, input: { ...base, deliveryMinutes } }, { onSuccess: done })
    }
  }

  return (
    <>
      <FormDialog
        isOpen={!confirmingDelete}
        onOpenChange={(isOpen) => !isOpen && onClose()}
        title={
          entry
            ? t(`settings.catalog.${titleKey}.dialogEdit`, { name: entry.name })
            : t(`settings.catalog.${titleKey}.dialogNew`)
        }
        submitLabel={entry ? t('common.save') : t(`settings.catalog.${titleKey}.add`)}
        onSubmit={submit}
        isPending={save.isPending}
        error={<ErrorAlert error={save.error} />}
        footerStart={
          entry &&
          (entry.ticketCount > 0 ? (
            <span className="max-w-xs text-xs text-muted">{t('settings.catalog.inUse')}</span>
          ) : (
            <Button variant="danger" onPress={() => setConfirmingDelete(true)}>
              {t('settings.catalog.delete')}
            </Button>
          ))
        }
      >
        <TextInput
          label={t('settings.catalog.name')}
          value={name}
          onChange={setName}
          isRequired
          isInvalid={submitted && nameMissing}
          errorMessage={t('settings.catalog.nameRequired')}
        />
        <TextInput label={t('settings.catalog.description')} value={description} onChange={setDescription} />
        {kind === 'item' && (
          <div className="grid gap-4 sm:grid-cols-2">
            <NumberInput
              label={t('settings.catalog.items.deliveryTime')}
              value={amount}
              onChange={setAmount}
              minValue={MIN_DELIVERY}
              isInvalid={submitted && deliveryInvalid}
              errorMessage={t('settings.catalog.items.positive')}
            />
            <Dropdown
              label={t('settings.catalog.items.deliveryUnit')}
              options={unitsFor(CALENDAR).map((u) => ({ id: u, label: t(`duration.unit.${u}`) }))}
              value={unit}
              onChange={setUnit}
            />
          </div>
        )}
        <NumberInput
          label={t('settings.catalog.order')}
          description={t('settings.catalog.orderHelp')}
          value={sortOrder}
          onChange={setSortOrder}
          minValue={0}
        />
        <Checkbox isSelected={active} onChange={setActive}>
          {t('settings.catalog.shown')}
        </Checkbox>
      </FormDialog>
      {entry && (
        <ConfirmDialog
          isOpen={confirmingDelete}
          onOpenChange={(isOpen) => {
            if (!isOpen) {
              setConfirmingDelete(false)
              remove.reset()
            }
          }}
          tone="danger"
          title={t('settings.catalog.deleteTitle', { name: entry.name })}
          confirmLabel={t('settings.catalog.delete')}
          isPending={remove.isPending}
          error={<ErrorAlert error={remove.error} />}
          onConfirm={() =>
            remove.mutate(entry.id, { onSuccess: () => onDone(t('settings.catalog.deleted', { name: entry.name })) })
          }
        >
          {t('settings.catalog.deleteBody')}
        </ConfirmDialog>
      )}
    </>
  )
}
