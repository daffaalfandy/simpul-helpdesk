import { Check, ChevronDown } from 'lucide-react'
import type { Key, ReactNode } from 'react'
import { Button, FieldError, Label, ListBox, ListBoxItem, Popover, Select, SelectValue, Text } from 'react-aria-components'

import { cx } from '../lib/text'
import { fieldLabel } from './fieldStyles'

export type DropdownOption<K extends string> = { id: K; label: string; description?: string }

type DropdownProps<K extends string> = {
  label: ReactNode
  options: ReadonlyArray<DropdownOption<K>>
  value: K | null
  onChange: (value: K) => void
  size?: 'md' | 'lg'
  /** Keep the label for screen readers only (e.g. inside a table row). */
  hideLabel?: boolean
  description?: ReactNode
  /** Shown while nothing is chosen. Always pass a translated string. */
  placeholder?: string
  isDisabled?: boolean
  isRequired?: boolean
  isInvalid?: boolean
  errorMessage?: string
  className?: string
}

/**
 * Custom select per HANDOFF §14.3 (no native <select>). Keyboard: ↑/↓, Enter, Esc and
 * typeahead come from React Aria.
 */
export function Dropdown<K extends string>({
  label,
  options,
  value,
  onChange,
  size = 'md',
  hideLabel,
  description,
  placeholder,
  isDisabled,
  isRequired,
  isInvalid,
  errorMessage,
  className,
}: DropdownProps<K>) {
  return (
    <Select
      value={value}
      onChange={(key: Key | null) => key !== null && onChange(String(key) as K)}
      placeholder={placeholder}
      isDisabled={isDisabled}
      isRequired={isRequired}
      isInvalid={isInvalid}
      className={cx('group/select flex flex-col gap-1.5', className)}
    >
      <Label className={hideLabel ? 'sr-only' : fieldLabel(size)}>{label}</Label>
      <Button
        className={cx(
          'flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg border border-border-strong',
          'bg-surface px-3.5 text-left font-sans text-ink outline-none',
          'group-data-[open]/select:border-primary group-data-[open]/select:ring-1 group-data-[open]/select:ring-primary group-data-[open]/select:ring-inset',
          'group-data-[invalid]/select:border-danger',
          'data-disabled:cursor-not-allowed data-disabled:bg-surface-subtle data-disabled:text-muted',
          size === 'lg' ? 'min-h-(--control-h-lg) text-md' : 'min-h-(--control-h-md) text-base',
        )}
      >
        <SelectValue className="truncate font-bold data-placeholder:font-normal data-placeholder:text-muted">
          {({ isPlaceholder, selectedText, defaultChildren }) => (isPlaceholder ? defaultChildren : selectedText)}
        </SelectValue>
        <ChevronDown
          aria-hidden="true"
          className="size-(--icon-md) shrink-0 text-muted transition-transform duration-(--duration-base) group-data-[open]/select:rotate-180"
        />
      </Button>
      {description && (
        <Text slot="description" className="text-xs text-muted">
          {description}
        </Text>
      )}
      <FieldError className="text-xs font-semibold text-danger">{errorMessage}</FieldError>
      <Popover className="max-h-80 min-w-(--trigger-width) overflow-auto rounded-xl border border-border-strong bg-surface p-1.5 shadow-popover outline-none data-entering:animate-enter">
        <ListBox className="flex flex-col gap-0.5 outline-none">
          {options.map((o) => (
            <ListBoxItem
              key={o.id}
              id={o.id}
              textValue={o.label}
              className={cx(
                'group/option flex min-h-(--control-h-md) cursor-pointer items-center justify-between gap-3',
                'rounded-md px-3 py-2 text-ink outline-none data-focused:bg-surface-subtle data-selected:bg-primary-tint',
              )}
            >
              <span className="flex flex-col">
                <span className="font-bold">{o.label}</span>
                {o.description && <span className="text-xs text-muted">{o.description}</span>}
              </span>
              <Check aria-hidden="true" className="hidden size-(--icon-md) shrink-0 text-primary group-data-selected/option:block" />
            </ListBoxItem>
          ))}
        </ListBox>
      </Popover>
    </Select>
  )
}
