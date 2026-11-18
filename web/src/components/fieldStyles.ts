export type FieldSize = 'md' | 'lg'

/**
 * Field label per the mock: compact forms (dialogs, settings, the agent app) use a small muted label;
 * the portal's large fields use a slightly larger label in ink.
 */
export function fieldLabel(size: FieldSize = 'md') {
  return size === 'lg' ? 'text-sm font-bold text-ink' : 'text-xs font-bold text-muted'
}

/** A field's label, control, description and error, stacked. */
export const fieldStack = 'flex flex-col gap-1.5'

/** The bordered box around a field's input (number fields, date segments). */
export const fieldBox =
  'flex min-h-(--control-h-md) w-full items-center rounded-lg border border-border-strong bg-surface px-3.5 ' +
  'font-sans text-base text-ink data-invalid:border-danger data-focus-within:border-primary'

/** One typed part of a date (day, month, year); the focused part is filled so it reads as the caret. */
export const dateSegment =
  'rounded-xs px-0.5 tabular-nums outline-none data-placeholder:text-muted data-focused:bg-primary data-focused:text-on-primary'

export const fieldDescription = 'text-xs text-muted'
export const fieldError = 'text-xs font-semibold text-danger'
