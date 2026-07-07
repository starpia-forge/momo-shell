import i18n from '../../../shared/i18n'
import { useSessionStore } from '../model/store'

/** Shared confirm-before-close check, used when closing a tab or a single pane. */
export function confirmSessionClose(sessionId: string): boolean {
  const session = useSessionStore.getState().sessions[sessionId]
  if (session?.state === 'connecting') return window.confirm(i18n.t('workspace.confirmCloseConnecting'))
  return true
}
