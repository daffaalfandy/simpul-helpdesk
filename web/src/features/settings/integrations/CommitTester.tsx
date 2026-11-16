import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Input, Label, TextField } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { CommitRefAction, CommitTestRef } from '../../../api/client'
import { Card } from '../../../components/Card'
import { SEARCH_DEBOUNCE_MS } from '../../../lib/constants'
import { cx } from '../../../lib/text'
import { errorMessage } from '../shared'
import { commitTestQueryOptions } from './queries'

/** The sample messages under the tester (Integrations mock), in order. */
const SAMPLES = ['link', 'fix', 'reopen', 'none'] as const

/** Result groups, strongest first. */
const ACTIONS: readonly CommitRefAction[] = ['reopen', 'fix', 'link']

/** Matches the API's message limit (CommitTestInput). */
const MAX_MESSAGE_LENGTH = 5000

/** "Try a commit message" (Integrations mock): reads a message the way the connector does, as you type. */
export function CommitTester({ prefix, className }: { prefix: string; className?: string }) {
  const { t } = useTranslation()
  const [typed, setTyped] = useState(() => t('settings.integrations.tester.initial', { prefix }))
  const [message, setMessage] = useState(typed)
  useEffect(() => {
    const timer = setTimeout(() => setMessage(typed), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [typed])
  const test = useQuery({ ...commitTestQueryOptions(message), enabled: message.trim() !== '', placeholderData: keepPreviousData })

  return (
    <Card as="h3" tone="dark" title={t('settings.integrations.tester.title')} className={className}>
      <TextField value={typed} onChange={setTyped} maxLength={MAX_MESSAGE_LENGTH} className="flex flex-col gap-1.5">
        <Label className="text-sm font-bold text-on-dark-muted">{t('settings.integrations.tester.label')}</Label>
        <Input
          spellCheck={false}
          className="min-h-(--control-h-md) w-full rounded-lg border-0 bg-surface px-3.5 font-mono text-base text-ink"
        />
      </TextField>
      <div role="status" className="flex flex-col gap-3 rounded-lg bg-ink-2 px-3.5 py-3">
        {message.trim() === '' ? (
          <Result title={t('settings.integrations.tester.emptyTitle')} text={t('settings.integrations.tester.empty')} />
        ) : test.isError ? (
          <Result title={t('common.errorTitle')} text={errorMessage(t, test.error)} />
        ) : test.data ? (
          <Results refs={test.data.refs} prefix={prefix} />
        ) : (
          <Result title={t('settings.integrations.tester.reading')} />
        )}
      </div>
      <div className="flex flex-wrap gap-1.5">
        {SAMPLES.map((s) => {
          const sample = t(`settings.integrations.tester.samples.${s}`, { prefix })
          return (
            <button
              key={s}
              type="button"
              onClick={() => setTyped(sample)}
              className={cx(
                'hit-area min-h-(--control-h-sm) cursor-pointer rounded-md border border-avatar-none px-2.5 font-mono text-xs text-on-primary',
                'hover:bg-ink-2',
              )}
            >
              {sample}
            </button>
          )
        })}
      </div>
    </Card>
  )
}

function Results({ refs, prefix }: { refs: CommitTestRef[]; prefix: string }) {
  const { t } = useTranslation()
  if (refs.length === 0) {
    return <Result title={t('settings.integrations.tester.noneTitle')} text={t('settings.integrations.tester.none', { prefix })} />
  }
  const missing = refs.filter((r) => !r.ticketExists).map((r) => r.key)
  return (
    <>
      {ACTIONS.map((action) => {
        const keys = refs.filter((r) => r.action === action).map((r) => r.key)
        if (keys.length === 0) return null
        const list = keys.join(t('settings.integrations.tester.keySeparator'))
        return (
          <Result
            key={action}
            title={t(`settings.integrations.tester.${action}.title`, { keys: list })}
            text={t(`settings.integrations.tester.${action}.text`, { keys: list })}
          />
        )
      })}
      {missing.length > 0 && (
        <p className="text-sm text-on-dark-muted">
          {t('settings.integrations.tester.missing', {
            count: missing.length,
            keys: missing.join(t('settings.integrations.tester.keySeparator')),
          })}
        </p>
      )}
    </>
  )
}

function Result({ title, text }: { title: string; text?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-bold text-on-dark-accent">{title}</span>
      {text && <span className="text-base leading-relaxed">{text}</span>}
    </div>
  )
}
