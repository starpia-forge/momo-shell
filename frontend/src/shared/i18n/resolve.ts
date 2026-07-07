export type ResolvedLanguage = 'ko' | 'en' | 'zh' | 'ja'

const RESOLVED_LANGUAGES: readonly ResolvedLanguage[] = ['ko', 'en', 'zh', 'ja']

// resolveLanguage maps a stored language setting ('system' or an explicit
// code) plus the OS/browser locale to one of our four bundled languages,
// falling back to English when the system locale isn't one we ship.
export function resolveLanguage(setting: string, navigatorLang: string): ResolvedLanguage {
  if (setting !== 'system') {
    const explicit = RESOLVED_LANGUAGES.find((l) => l === setting)
    if (explicit) return explicit
  }
  const prefix = navigatorLang.toLowerCase().split('-')[0]
  return RESOLVED_LANGUAGES.find((l) => l === prefix) ?? 'en'
}
