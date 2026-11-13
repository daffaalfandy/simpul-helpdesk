import { useTimestamp } from './time'

/** A <time> showing a short stamp, with the full date and time on hover. */
export function Timestamp({ iso }: { iso: string }) {
  const { stamp, full } = useTimestamp()
  return (
    <time dateTime={iso} title={full(iso)}>
      {stamp(iso)}
    </time>
  )
}
