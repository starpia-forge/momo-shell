import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import ko from './locales/ko.json'
import en from './locales/en.json'
import zh from './locales/zh.json'
import ja from './locales/ja.json'
import { resolveLanguage } from './resolve'

// navigator is unavailable under vitest's node test environment; fall back
// to 'en' so importing this module in tests never throws.
const navLang = typeof navigator !== 'undefined' ? navigator.language : 'en'

void i18n.use(initReactI18next).init({
  resources: {
    ko: { translation: ko },
    en: { translation: en },
    zh: { translation: zh },
    ja: { translation: ja },
  },
  lng: resolveLanguage('system', navLang),
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  returnNull: false,
})

export default i18n
export { resolveLanguage } from './resolve'
export type { ResolvedLanguage } from './resolve'
