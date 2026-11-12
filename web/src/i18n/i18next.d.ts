import 'i18next'

import type en from './en.json'

// Typed translation keys: t('login.title') is checked at compile time.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: typeof en }
  }
}
