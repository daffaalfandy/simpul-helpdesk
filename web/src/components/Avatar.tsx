import { cx, initials } from '../lib/text'

const COLORS = ['bg-avatar-1', 'bg-avatar-2', 'bg-avatar-3', 'bg-avatar-4'] as const
const UNASSIGNED_MARK = '?'

/** Stable colour per person, derived from their id. */
function colorFor(seed: string): string {
  let sum = 0
  for (const ch of seed) sum += ch.charCodeAt(0)
  return COLORS[sum % COLORS.length] ?? COLORS[0]
}

type AvatarProps = {
  /** Nobody (e.g. an unassigned ticket) shows "?" on the neutral colour. */
  name?: string | null
  seed?: string
  size?: 'sm' | 'md'
  /** Accessible name, e.g. "Assignee: Rina Pratiwi". Without it the avatar is decorative. */
  label?: string
}

/** Initials avatar per HANDOFF §14.3. */
export function Avatar({ name, seed, size = 'md', label }: AvatarProps) {
  return (
    <span
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      className={cx(
        'inline-flex shrink-0 items-center justify-center rounded-full font-bold text-on-primary',
        name ? colorFor(seed ?? name) : 'bg-avatar-none',
        size === 'sm' ? 'size-(--avatar-sm) text-2xs' : 'size-(--avatar-md) text-xs',
      )}
    >
      {name ? initials(name) : UNASSIGNED_MARK}
    </span>
  )
}
