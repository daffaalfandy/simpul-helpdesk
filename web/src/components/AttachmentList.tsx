import { Paperclip } from 'lucide-react'

import { attachmentHref } from '../api/attachments'
import type { Attachment } from '../api/client'
import { formatFileSize } from '../lib/files'

/** Downloadable file chips. */
export function AttachmentList({ items }: { items: ReadonlyArray<Attachment> }) {
  if (items.length === 0) return null
  return (
    <ul className="flex flex-wrap gap-2">
      {items.map((a) => (
        <li key={a.id} className="max-w-full">
          <a
            href={attachmentHref(a.id)}
            download={a.filename}
            className="hit-area inline-flex min-h-(--control-h-sm) max-w-full items-center gap-1.5 rounded-md bg-tint px-2.5 text-sm font-semibold text-ink hover:bg-surface-subtle"
          >
            <Paperclip aria-hidden="true" className="size-(--icon-sm) shrink-0" />
            <span className="truncate">{a.filename}</span>
            <span className="shrink-0 font-normal text-muted">{formatFileSize(a.sizeBytes)}</span>
          </a>
        </li>
      ))}
    </ul>
  )
}
