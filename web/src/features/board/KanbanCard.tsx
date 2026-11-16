import { useRouter } from '@tanstack/react-router'
import { GitBranch, GripVertical } from 'lucide-react'
import { Button, GridListItem } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { BoardCard, BoardCardCode } from '../../api/client'
import { Avatar } from '../../components/Avatar'
import { Pill, PriorityBadge } from '../../components/Badges'
import { paths } from '../../lib/constants'
import { useCodeLabels } from '../code/labels'
import { SlaChip } from './SlaChip'

/**
 * Kanban card per HANDOFF §14.3: the whole card opens the ticket and drags with a pointer; the
 * grip starts a drag from the keyboard or a screen reader.
 */
export function KanbanCard({ card }: { card: BoardCard }) {
  const { t } = useTranslation()
  const router = useRouter()
  const href = router.buildLocation({ to: paths.ticket, params: { key: card.key } }).href
  const assignee = card.assignee?.name ?? t('ticket.unassigned')
  return (
    <GridListItem
      id={card.key}
      textValue={`${card.key} ${card.subject}`}
      href={href}
      // A view transition moves the card from its old column to its new one.
      style={{ viewTransitionName: `card-${card.id}` }}
      className={
        'flex cursor-grab flex-col rounded-lg bg-surface px-3.25 py-3 text-ink shadow-card outline-none ' +
        'transition-[box-shadow,opacity] duration-(--duration-fast) ease-standard hover:shadow-raised ' +
        'data-focus-visible:shadow-(--focus-ring) data-dragging:cursor-grabbing data-dragging:opacity-45 ' +
        'data-dragging:outline-2 data-dragging:outline-primary data-dragging:outline-dashed ' +
        'data-drop-target:outline-2 data-drop-target:outline-primary'
      }
    >
      <div className="flex items-start justify-between gap-2 text-xs font-semibold text-muted">
        <span>{t('ticket.keyType', { key: card.key, type: t(`ticket.type.${card.type}`) })}</span>
        <PriorityBadge priority={card.priority} />
      </div>
      <p className="mt-1.5 text-base leading-snug font-bold">{card.subject}</p>
      <p className="mt-1 text-sm text-muted">{card.client.name}</p>
      {card.code && <CodeChip code={card.code} />}
      <div className="mt-2.5 flex items-center justify-between">
        <Button
          slot="drag"
          aria-label={t('board.moveCard', { key: card.key })}
          className="hit-area -my-1.5 -ml-2 inline-flex size-(--control-h-sm) shrink-0 cursor-grab items-center justify-center rounded-md text-muted hover:bg-surface-subtle hover:text-ink"
        >
          <GripVertical aria-hidden="true" className="size-(--icon-md)" />
        </Button>
        <span className="flex min-w-0 flex-wrap items-center justify-end gap-2">
          {card.uat && (
            <Pill tone="paused">
              {card.uat.days != null
                ? t('agent.board.uatChip', { day: card.uat.day, days: card.uat.days })
                : t('agent.board.uatChipNoLimit', { day: card.uat.day })}
            </Pill>
          )}
          {card.sla && !(card.uat && card.sla.phase === 'paused') && <SlaChip sla={card.sla} />}
          <Avatar
            name={card.assignee?.name}
            seed={card.assignee?.id}
            size="sm"
            label={t('ticket.assigneeLabel', { name: assignee })}
          />
        </span>
      </div>
    </GridListItem>
  )
}

/** The code that reached the ticket (Main mock): its latest pull/merge request, or its commits. */
function CodeChip({ code }: { code: BoardCardCode }) {
  const { t } = useTranslation()
  const labels = useCodeLabels()
  const r = code.request
  let text: string
  if (r) {
    const request = labels.request(code.provider, r.number)
    if (r.state === 'merged') {
      text = code.deployed ? t('agent.board.code.mergedDeployed', { request }) : t('agent.board.code.merged', { request, target: r.target })
    } else if (r.state === 'closed') {
      text = t('agent.board.code.closed', { request })
    } else {
      text = t('agent.board.code.open', { request, target: r.target })
    }
  } else if (code.commits > 0) {
    text = t('agent.board.code.commits', { count: code.commits, branch: code.branch })
  } else {
    text = t('agent.board.code.deployed')
  }
  return (
    <p className="mt-2 flex items-center gap-1.5 rounded-md bg-chip-code-bg px-2 py-1.25 text-xs font-semibold text-chip-code-fg">
      <GitBranch aria-hidden="true" className="size-(--icon-sm) shrink-0" />
      <span className="min-w-0 truncate">{text}</span>
    </p>
  )
}
