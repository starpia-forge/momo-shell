export {
  openLocalSession,
  openSSHSession,
  openSSHDirectSession,
  attach,
  detach,
  disposeSession,
  fitSession,
  getFontSize,
  setFontSize,
  resetFontSize,
  setScrollback,
  applyTerminalTheme,
  searchSession,
  clearSearchSession,
  focusSession,
} from './lib/terminal-registry'
export { useSessionStore } from './model/store'
export type { SessionMeta, SessionState, SessionKind } from './model/store'
export { useHostKeyPromptStore } from './model/hostKeyPrompts'
export { confirmSessionClose } from './lib/confirmClose'
export { TerminalPane } from './ui/TerminalPane'
