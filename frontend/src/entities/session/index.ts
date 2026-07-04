export {
  openLocalSession,
  attach,
  detach,
  disposeSession,
  fitSession,
  getFontSize,
  setFontSize,
  resetFontSize,
} from './lib/terminal-registry'
export { useSessionStore } from './model/store'
export type { SessionMeta, SessionState } from './model/store'
export { TerminalPane } from './ui/TerminalPane'
