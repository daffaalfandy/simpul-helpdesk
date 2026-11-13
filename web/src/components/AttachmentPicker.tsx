import { CircleAlert, Paperclip, RotateCw, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { FileTrigger } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { formatFileSize } from '../lib/files'
import { cx } from '../lib/text'
import { Button } from './Button'
import { Spinner } from './Spinner'

export type UploadStatus = 'uploading' | 'done' | 'failed' | 'rejected'

/** A file picked for a message: uploading, uploaded, failed (can retry) or rejected before upload. */
export type UploadItem = { localId: number; name: string; size: number; status: UploadStatus; error?: string }

type AttachmentPickerProps = {
  items: ReadonlyArray<UploadItem>
  onSelect: (files: File[]) => void
  onRemove: (localId: number) => void
  onRetry: (localId: number) => void
  /** Allowed extensions, e.g. [".png", ".pdf"]. */
  accept: ReadonlyArray<string>
  buttonLabel: string
  hint?: ReactNode
  variant?: 'dashed' | 'outline'
  isDisabled?: boolean
}

/** File picker with one chip per file (HANDOFF §4: attach a screenshot or file). */
export function AttachmentPicker({
  items,
  onSelect,
  onRemove,
  onRetry,
  accept,
  buttonLabel,
  hint,
  variant = 'outline',
  isDisabled,
}: AttachmentPickerProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3">
        <FileTrigger
          acceptedFileTypes={[...accept]}
          allowsMultiple
          onSelect={(list) => list && onSelect(Array.from(list))}
        >
          <Button
            variant="outline"
            size="sm"
            isDisabled={isDisabled}
            className={cx(variant === 'dashed' && 'border-dashed')}
            icon={<Paperclip aria-hidden="true" className="size-(--icon-md)" />}
          >
            {buttonLabel}
          </Button>
        </FileTrigger>
        {hint && <span className="text-xs text-muted">{hint}</span>}
      </div>
      {items.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {items.map((item) => (
            <li
              key={item.localId}
              className={cx(
                'flex flex-wrap items-center gap-2 rounded-md px-2.5 py-1.5 text-sm',
                item.status === 'failed' || item.status === 'rejected' ? 'bg-danger-tint' : 'bg-tint',
              )}
            >
              {item.status === 'uploading' && <Spinner className="size-(--icon-sm)" />}
              {item.status === 'done' && <Paperclip aria-hidden="true" className="size-(--icon-sm) shrink-0" />}
              {(item.status === 'failed' || item.status === 'rejected') && (
                <CircleAlert aria-hidden="true" className="size-(--icon-sm) shrink-0 text-danger" />
              )}
              <span className="min-w-0 truncate font-semibold">{item.name}</span>
              <span className="text-muted">{formatFileSize(item.size)}</span>
              {item.status === 'uploading' && <span className="text-muted">{t('attach.uploading')}</span>}
              {item.error && (
                <span role="alert" className="font-semibold text-ink">
                  {item.error}
                </span>
              )}
              <span className="ml-auto flex gap-2">
                {item.status === 'failed' && (
                  <Button variant="ghost" size="sm" onPress={() => onRetry(item.localId)} aria-label={t('attach.retryLabel', { name: item.name })}>
                    <RotateCw aria-hidden="true" className="size-(--icon-sm)" />
                  </Button>
                )}
                <Button variant="ghost" size="sm" onPress={() => onRemove(item.localId)} aria-label={t('attach.removeLabel', { name: item.name })}>
                  <X aria-hidden="true" className="size-(--icon-sm)" />
                </Button>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
