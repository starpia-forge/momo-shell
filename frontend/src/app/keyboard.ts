import { getFontSize, setFontSize, resetFontSize } from '../entities/session'
import { useTabStore } from '../widgets/tab-bar'

const FONT_STEP = 1

// Capture phase so this runs before xterm's own keydown handling -- otherwise
// Ctrl/Cmd+= would already have been forwarded to the shell as a keystroke by
// the time this listener saw it.
export function registerGlobalShortcuts(): () => void {
  const handler = (e: KeyboardEvent) => {
    if (!(e.ctrlKey || e.metaKey)) return

    if (e.key === '=' || e.key === '+') {
      e.preventDefault()
      e.stopPropagation()
      setFontSize(getFontSize() + FONT_STEP)
    } else if (e.key === '-') {
      e.preventDefault()
      e.stopPropagation()
      setFontSize(getFontSize() - FONT_STEP)
    } else if (e.key === '0') {
      e.preventDefault()
      e.stopPropagation()
      resetFontSize()
    } else if (e.key.toLowerCase() === 't') {
      e.preventDefault()
      e.stopPropagation()
      useTabStore.getState().openNewTabPopover()
    } else if (e.key >= '1' && e.key <= '9') {
      e.preventDefault()
      e.stopPropagation()
      useTabStore.getState().activateByIndex(Number(e.key) - 1)
    }
  }

  window.addEventListener('keydown', handler, true)
  return () => window.removeEventListener('keydown', handler, true)
}
