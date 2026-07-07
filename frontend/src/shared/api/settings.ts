import { GetAll, Set, AppInfo } from '../../../wailsjs/go/wails/SettingsService'

export type Theme = 'dark' | 'light'
export type Accent = 'pink' | 'orange' | 'purple' | 'blue'

const ACCENTS: Accent[] = ['pink', 'orange', 'purple', 'blue']

export interface AppSettings {
  theme: Theme
  accent: Accent
  fontSize: number
  scrollback: number
}

const DEFAULTS: AppSettings = { theme: 'dark', accent: 'pink', fontSize: 14, scrollback: 10000 }

// parseAppSettings tolerates a raw KV map missing keys or holding malformed
// values (e.g. a hand-edited DB row) by falling back to defaults per field.
export function parseAppSettings(raw: Record<string, string>): AppSettings {
  const theme = raw['appearance.theme'] === 'light' ? 'light' : DEFAULTS.theme
  const accent = ACCENTS.includes(raw['appearance.accent'] as Accent) ? (raw['appearance.accent'] as Accent) : DEFAULTS.accent
  const fontSize = Number(raw['terminal.fontSize'])
  const scrollback = Number(raw['terminal.scrollback'])
  return {
    theme,
    accent,
    fontSize: Number.isFinite(fontSize) ? fontSize : DEFAULTS.fontSize,
    scrollback: Number.isFinite(scrollback) ? scrollback : DEFAULTS.scrollback,
  }
}

export async function getAllSettings(): Promise<AppSettings> {
  return parseAppSettings(await GetAll())
}

export async function setSetting(key: string, value: string): Promise<void> {
  await Set(key, value)
}

export async function getAppInfo(): Promise<{ version: string }> {
  return AppInfo()
}
