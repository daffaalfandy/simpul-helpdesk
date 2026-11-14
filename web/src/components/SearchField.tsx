import { Search, X } from 'lucide-react'
import { Button, Input, Label, SearchField as AriaSearchField } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { cx } from '../lib/text'

type SearchFieldProps = {
  /** Read by screen readers; the placeholder says what to type. */
  label: string
  placeholder: string
  value: string
  onChange: (value: string) => void
  className?: string
}

/** Search box with a leading icon and a clear button; Escape clears it too. */
export function SearchField({ label, placeholder, value, onChange, className }: SearchFieldProps) {
  const { t } = useTranslation()
  return (
    <AriaSearchField value={value} onChange={onChange} className={cx('group flex flex-col', className)}>
      <Label className="sr-only">{label}</Label>
      <div
        className={
          'flex min-h-(--control-h-md) items-center gap-2 rounded-lg border border-border-strong bg-surface pr-1 pl-3.5 ' +
          'transition-colors duration-(--duration-fast) ease-standard focus-within:border-primary'
        }
      >
        <Search aria-hidden="true" className="size-(--icon-md) shrink-0 text-muted" />
        <Input
          placeholder={placeholder}
          className={
            'min-w-0 flex-1 bg-transparent font-sans text-base text-ink outline-none placeholder:text-muted ' +
            '[&::-webkit-search-cancel-button]:appearance-none'
          }
        />
        <Button
          aria-label={t('common.clearSearch')}
          className={
            'hit-area inline-flex size-(--control-h-sm) shrink-0 items-center justify-center rounded-md text-muted ' +
            'hover:bg-surface-subtle hover:text-ink group-data-empty:invisible'
          }
        >
          <X aria-hidden="true" className="size-(--icon-md)" />
        </Button>
      </div>
    </AriaSearchField>
  )
}
