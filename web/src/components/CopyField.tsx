import { Check, Copy } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cx } from '../lib/text'
import { Button } from './Button'

/** How long "Copied" shows after a copy. */
const COPIED_MS = 2000

type CopyFieldProps = {
  label: string
  value: string
  hint?: string
  /** A multi-line value (a config file) keeps its lines; a single value wraps anywhere. */
  block?: boolean
}

/**
 * A value to paste somewhere else (a webhook secret, a token, a command), with a Copy button.
 * Renders a <dt>/<dd> pair, so place it inside a <dl>.
 */
export function CopyField({ label, value, hint, block }: CopyFieldProps) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(timer.current), [])
  const copy = async () => {
    await navigator.clipboard.writeText(value)
    setCopied(true)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setCopied(false), COPIED_MS)
  }
  const Value = block ? 'pre' : 'code'
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-sm font-bold text-ink">{label}</dt>
      <dd className="flex flex-wrap items-center gap-2">
        <Value
          className={cx(
            'min-w-0 flex-1 rounded-md bg-surface-subtle px-3 py-2 font-mono text-sm text-ink',
            block ? 'overflow-x-auto whitespace-pre' : 'break-all',
          )}
        >
          {value}
        </Value>
        <Button
          variant="outline"
          size="sm"
          icon={copied ? <Check aria-hidden="true" className="size-(--icon-sm)" /> : <Copy aria-hidden="true" className="size-(--icon-sm)" />}
          onPress={() => void copy()}
          aria-label={t('common.copyLabel', { label })}
        >
          {copied ? t('common.copied') : t('common.copy')}
        </Button>
        {hint && <span className="w-full text-xs text-muted">{hint}</span>}
      </dd>
    </div>
  )
}
