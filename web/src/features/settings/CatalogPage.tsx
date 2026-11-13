import { useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { CatalogItem } from '../../api/client'
import { Alert } from '../../components/Alert'
import { Pill } from '../../components/Badges'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Checkbox } from '../../components/Checkbox'
import { ConfirmDialog, FormDialog } from '../../components/Dialog'
import { NumberInput } from '../../components/Fields'
import { RowHeader, Table, Td, Th } from '../../components/Table'
import { TextInput } from '../../components/TextInput'
import { catalogItemsQueryOptions, useDeleteCatalogItem, useSaveCatalogItem } from './queries'
import { ErrorAlert, SectionHeader } from './shared'

/** New entries go after the existing ones. */
const SORT_STEP = 10

/**
 * Settings › Service desk › Catalogue: what clients can ask for in a service request. Delivery
 * times come from each project's SLA (follow-up #1).
 */
export function CatalogPage() {
  const { t } = useTranslation()
  const { data: items } = useSuspenseQuery(catalogItemsQueryOptions)
  const [flash, setFlash] = useState<string | null>(null)
  // undefined: closed; null: adding a new item.
  const [editing, setEditing] = useState<CatalogItem | null | undefined>(undefined)
  const open = (item: CatalogItem | null) => {
    setFlash(null)
    setEditing(item)
  }
  const nextSort = items.reduce((max, x) => Math.max(max, x.sortOrder), 0) + SORT_STEP

  return (
    <>
      <SectionHeader section="catalog" />
      {flash && <Alert tone="success">{flash}</Alert>}
      <Card
        as="h3"
        title={t('settings.catalog.items.title')}
        description={t('settings.catalog.items.description')}
        actions={
          <Button icon={<Plus aria-hidden="true" className="size-(--icon-md)" />} onPress={() => open(null)}>
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
                <Th>{t('settings.catalog.cols.description')}</Th>
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
                  <RowHeader>{i.name}</RowHeader>
                  <Td className="text-muted">{i.description ?? t('common.none')}</Td>
                  <Td>
                    <ShownPill active={i.active} />
                  </Td>
                  <Td className="text-right tabular-nums">{i.ticketCount}</Td>
                  <Td className="text-right">
                    <EditButton name={i.name} onPress={() => open(i)} />
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
      {editing !== undefined && (
        <EntryDialog
          entry={editing}
          nextSortOrder={nextSort}
          onClose={() => setEditing(undefined)}
          onDone={(message) => {
            setEditing(undefined)
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
  entry: CatalogItem | null
  nextSortOrder: number
  onClose: () => void
  onDone: (message: string) => void
}

function EntryDialog({ entry, nextSortOrder, onClose, onDone }: EntryDialogProps) {
  const { t } = useTranslation()
  const save = useSaveCatalogItem()
  const remove = useDeleteCatalogItem()
  const [name, setName] = useState(entry?.name ?? '')
  const [description, setDescription] = useState(entry?.description ?? '')
  const [sortOrder, setSortOrder] = useState(entry?.sortOrder ?? nextSortOrder)
  const [active, setActive] = useState(entry?.active ?? true)
  const [submitted, setSubmitted] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const nameMissing = name.trim() === ''

  const submit = () => {
    setSubmitted(true)
    if (nameMissing) return
    const input = { name: name.trim(), description: description.trim() || null, sortOrder, active }
    save.mutate(
      { id: entry?.id, input },
      { onSuccess: (saved) => onDone(t(entry ? 'settings.catalog.saved' : 'settings.catalog.created', { name: saved.name })) },
    )
  }

  return (
    <>
      <FormDialog
        isOpen={!confirmingDelete}
        onOpenChange={(isOpen) => !isOpen && onClose()}
        title={entry ? t('settings.catalog.items.dialogEdit', { name: entry.name }) : t('settings.catalog.items.dialogNew')}
        submitLabel={entry ? t('common.save') : t('settings.catalog.items.add')}
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
