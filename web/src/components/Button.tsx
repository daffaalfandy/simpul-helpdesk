import type { ReactNode } from 'react'
import { Button as AriaButton, type ButtonProps as AriaButtonProps } from 'react-aria-components'

import { cx } from '../lib/text'
import { Spinner } from './Spinner'

export type ButtonVariant = 'primary' | 'outline' | 'success' | 'danger' | 'dark' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'lg'

const base =
  'inline-flex items-center justify-center gap-2 font-sans font-bold whitespace-nowrap select-none ' +
  'transition-colors duration-(--duration-fast) ease-standard cursor-pointer ' +
  'disabled:cursor-not-allowed data-disabled:cursor-not-allowed'

const variants: Record<ButtonVariant, string> = {
  primary:
    'bg-primary text-on-primary hover:bg-primary-hover data-hovered:bg-primary-hover data-pressed:bg-primary-hover',
  outline:
    'bg-surface text-ink border border-border-strong hover:bg-surface-subtle data-hovered:bg-surface-subtle',
  success: 'bg-success text-on-primary hover:bg-success-ink data-hovered:bg-success-ink',
  danger: 'bg-danger text-on-primary',
  dark: 'bg-ink text-on-primary hover:bg-ink-2 data-hovered:bg-ink-2',
  ghost: 'bg-transparent text-primary hover:text-primary-hover data-hovered:text-primary-hover',
}

const disabled = 'data-disabled:bg-disabled data-disabled:text-on-primary data-disabled:border-transparent'

const sizes: Record<ButtonSize, string> = {
  sm: 'hit-area min-h-(--control-h-sm) px-3 rounded-md text-sm',
  md: 'min-h-(--control-h-md) px-4 rounded-lg text-base',
  lg: 'min-h-(--control-h-lg) px-5 rounded-lg text-md font-extrabold',
}

/** Class names for anything that should look like a button (e.g. links). */
export function buttonClasses(variant: ButtonVariant = 'primary', size: ButtonSize = 'md', className?: string) {
  return cx(base, variants[variant], sizes[size], className)
}

type ButtonProps = Omit<AriaButtonProps, 'children' | 'className'> & {
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: ReactNode
  className?: string
  children: ReactNode
}

/** Button per HANDOFF §14.3. While isPending, a spinner replaces the icon and presses are ignored. */
export function Button({ variant = 'primary', size = 'md', icon, className, children, isPending, ...props }: ButtonProps) {
  return (
    <AriaButton {...props} isPending={isPending} className={cx(buttonClasses(variant, size, className), disabled)}>
      {isPending ? <Spinner /> : icon}
      {children}
    </AriaButton>
  )
}
