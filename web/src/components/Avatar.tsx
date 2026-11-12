import { cx, initials } from '../lib/text'

const COLORS = ['bg-avatar-1', 'bg-avatar-2', 'bg-avatar-3', 'bg-avatar-4'] as const

/** Stable colour per person, derived from their id. */
function colorFor(seed: string): string {
  let sum = 0
  for (const ch of seed) sum += ch.charCodeAt(0)
  return COLORS[sum % COLORS.length] ?? COLORS[0]
}

type AvatarProps = { name: string; seed: string; size?: 'sm' | 'md' }

/** Initials avatar per HANDOFF §14.3. Decorative: the name is always shown next to it. */
export function Avatar({ name, seed, size = 'md' }: AvatarProps) {
  return (
    <span
      aria-hidden="true"
      className={cx(
        'inline-flex shrink-0 items-center justify-center rounded-full font-bold text-on-primary',
        colorFor(seed),
        size === 'sm' ? 'size-(--avatar-sm) text-2xs' : 'size-(--avatar-md) text-xs',
      )}
    >
      {initials(name)}
    </span>
  )
}
