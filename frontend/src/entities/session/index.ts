export {
  openLocalSession,
  openSSHSession,
  attach,
  detach,
  disposeSession,
  fitSession,
  getFontSize,
  setFontSize,
  resetFontSize,
} from './lib/terminal-registry'
export { useSessionStore } from './model/store'
export type { SessionMeta, SessionState, SessionKind } from './model/store'
export { useHostKeyPromptStore } from './model/hostKeyPrompts'
export { TerminalPane } from './ui/TerminalPane'
