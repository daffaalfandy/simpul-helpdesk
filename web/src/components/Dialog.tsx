import { CircleAlert, CircleCheck, Info, X, type LucideIcon } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Dialog as AriaDialog, Button as AriaButton, Form, Heading, Modal, ModalOverlay } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { cx } from '../lib/text'
import { Button } from './Button'
import { TextArea } from './TextArea'

const overlay =
  'fixed inset-0 z-50 flex justify-center overflow-y-auto bg-overlay px-5 py-10 ' +
  'data-entering:animate-fade-in data-exiting:animate-fade-in data-exiting:[animation-direction:reverse]'
const panel =
  'w-full rounded-dialog bg-surface p-6 shadow-dialog outline-none ' +
  'data-entering:animate-dialog-in data-exiting:animate-dialog-in data-exiting:[animation-direction:reverse]'

type FormDialogProps = {
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  subtitle?: ReactNode
  /** Submit label; the button shows a spinner while isPending. */
  submitLabel: ReactNode
  onSubmit: () => void
  isPending?: boolean
  /** Error banner or other content above the footer. */
  error?: ReactNode
  /** Extra footer content on the left, e.g. a delete button. */
  footerStart?: ReactNode
  children: ReactNode
}

/** Form dialog per HANDOFF §14.3 (max 640px, top-aligned, scrolls). Esc and the X close it. */
export function FormDialog({
  isOpen,
  onOpenChange,
  title,
  subtitle,
  submitLabel,
  onSubmit,
  isPending,
  error,
  footerStart,
  children,
}: FormDialogProps) {
  const { t } = useTranslation()
  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSubmit()
  }
  return (
    // Not dismissed by clicking outside, so a stray click can't throw away a filled-in form. Esc still closes.
    <ModalOverlay isOpen={isOpen} onOpenChange={onOpenChange} className={cx(overlay, 'items-start')}>
      <Modal className={cx(panel, 'max-w-dialog-form')}>
        <AriaDialog className="outline-none">
          {({ close }) => (
            <Form onSubmit={submit} validationBehavior="aria" className="flex flex-col gap-5">
              <div className="flex items-start justify-between gap-4">
                <div className="flex flex-col gap-1">
                  <Heading slot="title" className="text-xl font-extrabold tracking-title">
                    {title}
                  </Heading>
                  {subtitle && <p className="text-sm text-muted">{subtitle}</p>}
                </div>
                <AriaButton
                  aria-label={t('common.close')}
                  onPress={close}
                  className="inline-flex size-(--control-h-md) shrink-0 cursor-pointer items-center justify-center rounded-md text-muted data-hovered:bg-surface-subtle"
                >
                  <X aria-hidden="true" className="size-(--icon-lg)" />
                </AriaButton>
              </div>
              {children}
              {error}
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border-subtle pt-4">
                <div>{footerStart}</div>
                <div className="flex flex-wrap gap-3">
                  <Button variant="outline" onPress={close}>
                    {t('common.cancel')}
                  </Button>
                  <Button type="submit" isPending={isPending}>
                    {submitLabel}
                  </Button>
                </div>
              </div>
            </Form>
          )}
        </AriaDialog>
      </Modal>
    </ModalOverlay>
  )
}

export type ConfirmTone = 'primary' | 'success' | 'danger'

const tones: Record<ConfirmTone, { icon: LucideIcon; circle: string; button: 'primary' | 'success' | 'danger' }> = {
  primary: { icon: Info, circle: 'bg-primary-tint text-primary', button: 'primary' },
  success: { icon: CircleCheck, circle: 'bg-success-tint text-success', button: 'success' },
  danger: { icon: CircleAlert, circle: 'bg-danger-tint text-danger', button: 'danger' },
}

type ConfirmDialogProps = {
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  tone?: ConfirmTone
  title: ReactNode
  children?: ReactNode
  confirmLabel: ReactNode
  /** Called with the reason when reasonLabel is set. */
  onConfirm: (reason: string) => void
  isPending?: boolean
  /** Asks for a required reason; confirm stays disabled until it is filled. */
  reasonLabel?: string
  error?: ReactNode
}

/** Confirmation dialog per HANDOFF §14.3: tinted icon, title, body, Cancel + coloured confirm. */
export function ConfirmDialog({
  isOpen,
  onOpenChange,
  tone = 'primary',
  title,
  children,
  confirmLabel,
  onConfirm,
  isPending,
  reasonLabel,
  error,
}: ConfirmDialogProps) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const { icon: Icon, circle, button } = tones[tone]
  const missingReason = reasonLabel !== undefined && reason.trim() === ''
  return (
    <ModalOverlay
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) setReason('')
        onOpenChange(open)
      }}
      isDismissable
      className={cx(overlay, 'items-center')}
    >
      <Modal className={cx(panel, 'max-w-dialog-confirm')}>
        <AriaDialog role="alertdialog" className="flex flex-col gap-4 outline-none">
          {({ close }) => (
            <>
              <span className={cx('inline-flex size-12 items-center justify-center rounded-full', circle)}>
                <Icon aria-hidden="true" className="size-(--icon-lg)" />
              </span>
              <Heading slot="title" className="text-xl font-extrabold tracking-title">
                {title}
              </Heading>
              {children && <div className="text-base text-ink-2">{children}</div>}
              {reasonLabel !== undefined && (
                <TextArea label={reasonLabel} value={reason} onChange={setReason} isRequired />
              )}
              {error}
              <div className="flex flex-wrap justify-end gap-3">
                <Button variant="outline" onPress={close}>
                  {t('common.cancel')}
                </Button>
                <Button
                  variant={button}
                  isDisabled={missingReason}
                  isPending={isPending}
                  onPress={() => onConfirm(reason.trim())}
                >
                  {confirmLabel}
                </Button>
              </div>
            </>
          )}
        </AriaDialog>
      </Modal>
    </ModalOverlay>
  )
}
