import { Check, ChevronDown, Plus } from 'lucide-react'
import { useState, type Key, type ReactNode } from 'react'
import {
  Button,
  ComboBox,
  FieldError,
  Input,
  Label,
  ListBox,
  ListBoxItem,
  Popover,
  Text,
  useFilter,
} from 'react-aria-components'

import { cx } from '../lib/text'

export type SearchOption<K extends string> = { id: K; label: string; description?: string; searchText?: string }

type SearchSelectProps<K extends string> = {
  label: ReactNode
  options: ReadonlyArray<SearchOption<K>>
  value: K | null
  onChange: (value: K | null) => void
  placeholder?: string
  description?: ReactNode
  /** Shown when nothing matches. */
  emptyText: string
  isDisabled?: boolean
  isRequired?: boolean
  isInvalid?: boolean
  errorMessage?: string
  /** A last row that acts on the typed text, e.g. 'Add "Siti" as a new requester'. */
  action?: { label: (query: string) => string; onAction: (query: string) => void }
}

const ACTION_KEY = '__search-select-action__'

type Row = { id: string; label: string; description?: string; isAction?: boolean }

/**
 * Searchable dropdown (React Aria ComboBox) for long lists such as a client's requesters.
 * Remount it (key) to clear it from outside.
 */
export function SearchSelect<K extends string>({
  label,
  options,
  value,
  onChange,
  placeholder,
  description,
  emptyText,
  isDisabled,
  isRequired,
  isInvalid,
  errorMessage,
  action,
}: SearchSelectProps<K>) {
  const selected = options.find((o) => o.id === value)
  const [query, setQuery] = useState(selected?.label ?? '')
  const { contains } = useFilter({ sensitivity: 'base' })
  const typed = query.trim()
  const showAll = typed === '' || typed === selected?.label
  const rows: Row[] = options.filter((o) => showAll || contains(o.searchText ?? o.label, typed))
  if (action && typed !== '' && !showAll) rows.push({ id: ACTION_KEY, label: action.label(typed), isAction: true })

  const select = (key: Key | null) => {
    if (key === ACTION_KEY) {
      action?.onAction(typed)
      return
    }
    onChange(key === null ? null : (String(key) as K))
  }

  return (
    <ComboBox
      items={rows}
      inputValue={query}
      onInputChange={setQuery}
      selectedKey={value}
      onSelectionChange={select}
      menuTrigger="focus"
      allowsEmptyCollection
      isDisabled={isDisabled}
      isRequired={isRequired}
      isInvalid={isInvalid}
      className="group/combo flex flex-col gap-1.5"
    >
      <Label className="text-sm font-bold text-ink">{label}</Label>
      <div className="relative">
        <Input
          placeholder={placeholder}
          className={cx(
            'min-h-(--control-h-md) w-full rounded-lg border border-border-strong bg-surface py-0 pr-10 pl-3.5 font-sans text-base text-ink',
            'placeholder:text-muted data-invalid:border-danger data-disabled:bg-surface-subtle',
          )}
        />
        <Button className="absolute inset-y-0 right-0 flex w-10 cursor-pointer items-center justify-center text-muted">
          <ChevronDown aria-hidden="true" className="size-(--icon-md)" />
        </Button>
      </div>
      {description && (
        <Text slot="description" className="text-xs text-muted">
          {description}
        </Text>
      )}
      <FieldError className="text-xs font-semibold text-danger">{errorMessage}</FieldError>
      <Popover className="max-h-80 min-w-(--trigger-width) overflow-auto rounded-xl border border-border-strong bg-surface p-1.5 shadow-popover outline-none">
        <ListBox
          className="flex flex-col gap-0.5 outline-none"
          renderEmptyState={() => <p className="px-3 py-2 text-sm text-muted">{emptyText}</p>}
        >
          {(row: Row) => (
            <ListBoxItem
              id={row.id}
              textValue={row.label}
              className={cx(
                'group/option flex min-h-(--control-h-md) cursor-pointer items-center justify-between gap-3',
                'rounded-md px-3 py-2 text-ink outline-none data-focused:bg-surface-subtle data-selected:bg-primary-tint',
                row.isAction && 'border-t border-border font-bold text-primary',
              )}
            >
              <span className="flex items-center gap-2">
                {row.isAction && <Plus aria-hidden="true" className="size-(--icon-md)" />}
                <span className="flex flex-col">
                  <span className={row.isAction ? '' : 'font-bold'}>{row.label}</span>
                  {row.description && <span className="text-xs text-muted">{row.description}</span>}
                </span>
              </span>
              <Check aria-hidden="true" className="hidden size-(--icon-md) shrink-0 text-primary group-data-selected/option:block" />
            </ListBoxItem>
          )}
        </ListBox>
      </Popover>
    </ComboBox>
  )
}
