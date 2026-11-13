import type { AttachmentPolicy } from '../api/client'
import { DATE_LOCALE } from '../i18n/locale'

const BYTES_PER_KILOBYTE = 1024
const BYTES_PER_MEGABYTE = BYTES_PER_KILOBYTE * BYTES_PER_KILOBYTE
const EXTENSION_SEPARATOR = '.'

export type FileProblem = 'tooLarge' | 'typeNotAllowed'

/** ".png" for "Screen Shot.PNG"; empty without an extension. */
export function fileExtension(name: string): string {
  const dot = name.lastIndexOf(EXTENSION_SEPARATOR)
  return dot > 0 ? name.slice(dot).toLowerCase() : ''
}

/** Checks a file against the API's upload policy before sending it. The server checks again. */
export function checkFile(file: File, policy: AttachmentPolicy): FileProblem | null {
  if (file.size > policy.maxBytes) return 'tooLarge'
  if (!policy.extensions.includes(fileExtension(file.name))) return 'typeNotAllowed'
  return null
}

const kilobytes = new Intl.NumberFormat(DATE_LOCALE, { style: 'unit', unit: 'kilobyte', maximumFractionDigits: 0 })
const megabytes = new Intl.NumberFormat(DATE_LOCALE, { style: 'unit', unit: 'megabyte', maximumFractionDigits: 1 })

/** "84 kB", "1.2 MB". */
export function formatFileSize(bytes: number): string {
  if (bytes < BYTES_PER_MEGABYTE) return kilobytes.format(Math.max(1, Math.round(bytes / BYTES_PER_KILOBYTE)))
  return megabytes.format(bytes / BYTES_PER_MEGABYTE)
}

/** Whole megabytes, for "Max 10 MB". */
export function megabytesOf(bytes: number): number {
  return Math.floor(bytes / BYTES_PER_MEGABYTE)
}
