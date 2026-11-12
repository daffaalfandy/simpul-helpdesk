import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import en from './en.json'

export const DEFAULT_LANGUAGE = 'en'
export const DEFAULT_NAMESPACE = 'translation'

export const resources = { en: { [DEFAULT_NAMESPACE]: en } } as const

void i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LANGUAGE,
  fallbackLng: DEFAULT_LANGUAGE,
  defaultNS: DEFAULT_NAMESPACE,
  interpolation: { escapeValue: false }, // React already escapes
})

export default i18n
