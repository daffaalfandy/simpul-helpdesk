import { Switch as AriaSwitch, type SwitchProps as AriaSwitchProps } from 'react-aria-components'

import { cx } from '../lib/text'

type SwitchProps = Omit<AriaSwitchProps, 'children' | 'className'> & {
  /** Accessible name; the switch has no visible label of its own (it sits in a table cell). */
  'aria-label': string
  className?: string
}

/**
 * On/off switch (Notifications settings). Saves on change, so it never needs a submit button.
 * A disabled switch that is on reads as "always on": dimmed, with a not-allowed cursor.
 */
export function Switch({ className, ...props }: SwitchProps) {
  return (
    <AriaSwitch
      {...props}
      className={cx(
        'group relative inline-flex cursor-pointer items-center data-disabled:cursor-not-allowed',
        // 44px touch target around the 26px track.
        "before:absolute before:inset-[calc((var(--switch-h)-var(--control-h-md))/2)] before:content-['']",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cx(
          'flex h-(--switch-h) w-(--switch-w) items-center rounded-full bg-border-strong px-(--switch-inset)',
          'transition-colors duration-(--duration-base) ease-standard',
          'group-data-selected:bg-primary group-data-disabled:opacity-60',
          'group-data-hovered:brightness-95',
          'group-data-focus-visible:shadow-(--focus-ring)',
        )}
      >
        <span
          className={cx(
            'size-(--switch-knob) rounded-full bg-surface shadow-card transition-transform duration-(--duration-base) ease-standard',
            'group-data-selected:translate-x-[calc(var(--switch-w)-var(--switch-knob)-2*var(--switch-inset))]',
          )}
        />
      </span>
    </AriaSwitch>
  )
}
