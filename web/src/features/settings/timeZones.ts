import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

/** Indonesia's three zones first (Settings mock), each with its local name; then every zone. */
const INDONESIAN_ZONES = [
  { id: 'Asia/Jakarta', key: 'wib' },
  { id: 'Asia/Makassar', key: 'wita' },
  { id: 'Asia/Jayapura', key: 'wit' },
] as const

/** The zone a new organization starts in, matching the API's default. */
export const DEFAULT_TIME_ZONE = INDONESIAN_ZONES[0].id

/** Time zone choices for a SearchSelect; current stays listed even if the browser lacks it. */
export function useTimeZoneOptions(current: string) {
  const { t } = useTranslation()
  return useMemo(() => {
    const known = new Set<string>(INDONESIAN_ZONES.map((z) => z.id))
    const rest = Intl.supportedValuesOf('timeZone').filter((z) => !known.has(z))
    if (!known.has(current) && !rest.includes(current)) rest.unshift(current)
    return [
      ...INDONESIAN_ZONES.map((z) => ({ id: z.id, label: t(`settings.general.zones.${z.key}`), searchText: z.id })),
      ...rest.map((z) => ({ id: z, label: z })),
    ]
  }, [current, t])
}
