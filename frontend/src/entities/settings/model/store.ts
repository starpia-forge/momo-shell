import { create } from 'zustand'
import { getAllSettings, setSetting, type Accent, type Theme } from '../../../shared/api/settings'

const MIN_FONT_SIZE = 8
const MAX_FONT_SIZE = 32
const DEFAULT_FONT_SIZE = 14

const MIN_SCROLLBACK = 1000
const MAX_SCROLLBACK = 100000

const FONT_SIZE_PERSIST_DELAY_MS = 500

interface SettingsStore {
  theme: Theme
  accent: Accent
  fontSize: number
  scrollback: number
  loaded: boolean
  load: () => Promise<void>
  setTheme: (theme: Theme) => Promise<void>
  setAccent: (accent: Accent) => Promise<void>
  setScrollback: (lines: number) => Promise<void>
  setFontSize: (size: number) => void
  resetFontSize: () => void
}

let fontSizePersistTimer: ReturnType<typeof setTimeout> | undefined

export const useSettingsStore = create<SettingsStore>((set, get) => ({
  theme: 'dark',
  accent: 'pink',
  fontSize: DEFAULT_FONT_SIZE,
  scrollback: 10000,
  loaded: false,
  load: async () => {
    const settings = await getAllSettings()
    set({ ...settings, loaded: true })
  },
  setTheme: async (theme) => {
    const previous = get().theme
    set({ theme })
    try {
      await setSetting('appearance.theme', theme)
    } catch (err) {
      set({ theme: previous })
      throw err
    }
  },
  setAccent: async (accent) => {
    const previous = get().accent
    set({ accent })
    try {
      await setSetting('appearance.accent', accent)
    } catch (err) {
      set({ accent: previous })
      throw err
    }
  },
  setScrollback: async (lines) => {
    const clamped = Math.min(MAX_SCROLLBACK, Math.max(MIN_SCROLLBACK, lines))
    const previous = get().scrollback
    set({ scrollback: clamped })
    try {
      await setSetting('terminal.scrollback', String(clamped))
    } catch (err) {
      set({ scrollback: previous })
      throw err
    }
  },
  setFontSize: (size) => {
    const clamped = Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, size))
    set({ fontSize: clamped })
    if (fontSizePersistTimer) clearTimeout(fontSizePersistTimer)
    fontSizePersistTimer = setTimeout(() => {
      void setSetting('terminal.fontSize', String(clamped)).catch((err) => console.error(err))
    }, FONT_SIZE_PERSIST_DELAY_MS)
  },
  resetFontSize: () => {
    get().setFontSize(DEFAULT_FONT_SIZE)
  },
}))
