import type { TFunction } from 'i18next'

import en from '../i18n/en.json'
import { ApiError } from './client'

type ErrorCodeKey = keyof typeof en.errors

function isErrorCode(code: string): code is ErrorCodeKey {
  return Object.hasOwn(en.errors, code)
}

/** A translated message for an API error, or the generic one. */
export function errorMessage(t: TFunction, error: unknown): string {
  if (error instanceof ApiError && isErrorCode(error.code)) return t(`errors.${error.code}`)
  return t('common.errorGeneric')
}
