export type FieldSize = 'md' | 'lg'

/**
 * Field label per the mock: compact forms (dialogs, settings, the agent app) use a small muted label;
 * the portal's large fields use a slightly larger label in ink.
 */
export function fieldLabel(size: FieldSize = 'md') {
  return size === 'lg' ? 'text-sm font-bold text-ink' : 'text-xs font-bold text-muted'
}
