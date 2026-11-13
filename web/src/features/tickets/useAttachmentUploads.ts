import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { uploadAttachment } from '../../api/attachments'
import type { AttachmentPolicy } from '../../api/client'
import { errorMessage } from '../../api/errors'
import type { UploadItem } from '../../components/AttachmentPicker'
import { checkFile, megabytesOf } from '../../lib/files'

type Entry = UploadItem & { file: File; attachmentId?: string }

/**
 * Files picked for one message. Each uploads as soon as it is picked; files the policy
 * rejects never reach the network. Uploads still running when the form unmounts are
 * cancelled; the server purges unlinked uploads after a day.
 */
export function useAttachmentUploads(policy: AttachmentPolicy | undefined) {
  const { t } = useTranslation()
  const [entries, setEntries] = useState<Entry[]>([])
  const controllers = useRef(new Map<number, AbortController>())
  const nextId = useRef(0)

  useEffect(() => {
    const running = controllers.current
    return () => running.forEach((c) => c.abort())
  }, [])

  const update = (localId: number, patch: Partial<Entry>) =>
    setEntries((list) => list.map((e) => (e.localId === localId ? { ...e, ...patch } : e)))

  const start = (localId: number, file: File) => {
    const controller = new AbortController()
    controllers.current.set(localId, controller)
    uploadAttachment(file, controller.signal)
      .then((a) => update(localId, { status: 'done', attachmentId: a.id, error: undefined }))
      .catch((e: unknown) => {
        if (!controller.signal.aborted) update(localId, { status: 'failed', error: errorMessage(t, e) })
      })
      .finally(() => controllers.current.delete(localId))
  }

  const add = (files: File[]) => {
    const max = policy?.maxPerMessage ?? Number.POSITIVE_INFINITY
    let kept = entries.filter((e) => e.status !== 'rejected').length
    const added: Entry[] = []
    for (const file of files) {
      const localId = ++nextId.current
      const base = { localId, name: file.name, size: file.size, file }
      const problem = policy ? checkFile(file, policy) : null
      if (problem) {
        const mb = policy ? megabytesOf(policy.maxBytes) : 0
        added.push({ ...base, status: 'rejected', error: t(`attach.${problem}`, { max: mb }) })
      } else if (kept >= max) {
        added.push({ ...base, status: 'rejected', error: t('attach.tooMany', { max }) })
      } else {
        kept++
        added.push({ ...base, status: 'uploading' })
        start(localId, file)
      }
    }
    setEntries((list) => [...list, ...added])
  }

  const remove = (localId: number) => {
    controllers.current.get(localId)?.abort()
    setEntries((list) => list.filter((e) => e.localId !== localId))
  }

  const retry = (localId: number) => {
    const entry = entries.find((e) => e.localId === localId)
    if (!entry) return
    update(localId, { status: 'uploading', error: undefined })
    start(localId, entry.file)
  }

  const reset = () => {
    controllers.current.forEach((c) => c.abort())
    setEntries([])
  }

  const attachmentIds = entries.flatMap((e) => (e.status === 'done' && e.attachmentId ? [e.attachmentId] : []))
  /** Uploading or failed files hold the form back until retried or removed. */
  const isBlocking = entries.some((e) => e.status === 'uploading' || e.status === 'failed')
  return { items: entries as UploadItem[], add, remove, retry, reset, attachmentIds, isBlocking }
}
