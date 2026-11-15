import { useTranslation } from 'react-i18next'

import type { BoardCardSla } from '../../api/client'
import { Pill } from '../../components/Badges'
import { SLA_TONE, slaChipText } from '../tickets/slaFormat'

/** A card's SLA state in words and colour (Main mock): "41m left", "Breached 12m", "Met SLA". */
export function SlaChip({ sla }: { sla: BoardCardSla }) {
  const { t } = useTranslation()
  const text = slaChipText(t, sla.clock, sla)
  if (!text) return null
  return <Pill tone={SLA_TONE[sla.phase]}>{text}</Pill>
}
