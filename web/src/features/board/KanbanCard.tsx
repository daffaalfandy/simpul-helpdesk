import { useRouter } from '@tanstack/react-router'
import { GridListItem } from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import type { BoardCard } from '../../api/client'
import { Avatar } from '../../components/Avatar'
import { PriorityBadge } from '../../components/Badges'
import { paths } from '../../lib/constants'

/** Kanban card per HANDOFF §14.3: the whole card opens the ticket. */
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
      className={
        'flex flex-col rounded-lg bg-surface px-3.25 py-3 text-ink shadow-card outline-none ' +
        'transition-shadow duration-(--duration-fast) ease-standard hover:shadow-raised data-focus-visible:shadow-(--focus-ring)'
      }
    >
      <div className="flex items-start justify-between gap-2 text-xs font-semibold text-muted">
        <span>{t('ticket.keyType', { key: card.key, type: t(`ticket.type.${card.type}`) })}</span>
        <PriorityBadge priority={card.priority} />
      </div>
      <p className="mt-1.5 text-base leading-snug font-bold">{card.subject}</p>
      <p className="mt-1 text-sm text-muted">{card.client.name}</p>
      <div className="mt-2.5 flex items-center justify-end">
        <Avatar
          name={card.assignee?.name}
          seed={card.assignee?.id}
          size="sm"
          label={t('ticket.assigneeLabel', { name: assignee })}
        />
      </div>
    </GridListItem>
  )
}
