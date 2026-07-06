import type { ITheme } from '@xterm/xterm'

/** Reads the current theme's runtime CSS variables (app/styles/main.css) and
 * builds the xterm ITheme from them, so a new terminal always matches the
 * app's current color scheme instead of a hardcoded value. */
export function readTerminalTheme(): ITheme {
  const style = getComputedStyle(document.documentElement)
  return {
    background: style.getPropertyValue('--theme-canvas').trim(),
    foreground: style.getPropertyValue('--theme-fg').trim(),
    cursor: style.getPropertyValue('--theme-fg').trim(),
  }
}
