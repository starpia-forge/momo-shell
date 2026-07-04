import { getFontSize, setFontSize, resetFontSize } from '../entities/session'

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
    }
  }

  window.addEventListener('keydown', handler, true)
  return () => window.removeEventListener('keydown', handler, true)
}
