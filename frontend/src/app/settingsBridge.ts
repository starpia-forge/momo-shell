import { applyTerminalTheme, setFontSize, setScrollback } from '../entities/session'
import { useSettingsStore } from '../entities/settings'

function applyTheme(theme: 'dark' | 'light') {
  document.documentElement.dataset.theme = theme
  applyTerminalTheme()
}

// initSettingsBridge wires entities/settings' pure value store to its real
// side effects (CSS theme variable + every live xterm instance). Applies
// the current snapshot once at subscribe time too, so a StrictMode
// unmount/remount racing with load()'s resolution never leaves a setting
// unapplied.
export function initSettingsBridge(): () => void {
  const initial = useSettingsStore.getState()
  applyTheme(initial.theme)
  setFontSize(initial.fontSize)
  setScrollback(initial.scrollback)

  return useSettingsStore.subscribe((state, prev) => {
    if (state.theme !== prev.theme) applyTheme(state.theme)
    if (state.fontSize !== prev.fontSize) setFontSize(state.fontSize)
    if (state.scrollback !== prev.scrollback) setScrollback(state.scrollback)
  })
}
