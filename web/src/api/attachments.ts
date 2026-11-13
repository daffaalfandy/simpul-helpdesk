import { api, API_BASE_PATH, type Attachment } from './client'
import { unwrap } from './unwrap'

/** The multipart field the API reads the file from. */
const ATTACHMENT_FORM_FIELD = 'file'

/** Uploads one file; it stays pending until a ticket or message names it in attachmentIds. */
export function uploadAttachment(file: File, signal?: AbortSignal): Promise<Attachment> {
  return unwrap(
    api.POST('/attachments', {
      // openapi-typescript types `format: binary` as string; the serializer sends the real File.
      body: { file: file as unknown as string },
      bodySerializer: () => {
        const form = new FormData()
        form.append(ATTACHMENT_FORM_FIELD, file, file.name)
        return form
      },
      signal,
    }),
  )
}

/** Download link for a file; the session cookie authorises it. */
export function attachmentHref(id: string): string {
  return `${API_BASE_PATH}/attachments/${encodeURIComponent(id)}`
}
