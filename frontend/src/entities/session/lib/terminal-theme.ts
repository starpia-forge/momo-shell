import type { ITheme } from '@xterm/xterm'

function readVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/** Reads the current theme's runtime CSS variables (app/styles/main.css) and
 * builds the xterm ITheme from them, so a new terminal always matches the
 * app's current color scheme instead of a hardcoded value. Only the ANSI
 * colors the design calls out are overridden -- the rest keep xterm's
 * defaults. */
export function readTerminalTheme(): ITheme {
  return {
    background: readVar('--theme-termbg'),
    foreground: readVar('--theme-termtext'),
    cursor: readVar('--theme-termtext'),
    green: readVar('--theme-green'),
    yellow: readVar('--theme-amber'),
    red: readVar('--theme-red'),
    blue: readVar('--theme-blue'),
    magenta: readVar('--theme-purple'),
    brightBlack: readVar('--theme-termdim'),
  }
}

/** xterm renders via canvas/WebGL, which can't resolve `var(--font-mono)`
 * directly -- read the concrete font stack from CSS instead of duplicating
 * it as a second hardcoded literal. */
export function readTerminalFontFamily(): string {
  return readVar('--font-mono')
}

/** Converts a `#rgb`/`#rrggbb` CSS color into an rgba() string at the given
 * alpha -- xterm's search decorations take solid colors, not our
 * theme's alpha-tint utilities. */
export function hexToRgba(hex: string, alpha: number): string {
  const h = hex.replace('#', '')
  const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h
  const n = parseInt(full, 16)
  const r = (n >> 16) & 255
  const g = (n >> 8) & 255
  const b = n & 255
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export function readAccentColor(): string {
  return readVar('--theme-accent')
}
