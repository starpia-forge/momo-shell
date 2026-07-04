import { disposeSession, useSessionStore } from '../../../entities/session'
import { useTabStore } from '../model/store'

// Closing a tab always closes its session -- Phase 3's split panes are the
// only future case where a tab could outlive one of its sessions, and that
// doesn't exist yet.
export function closeTab(tabId: string): void {
  const session = useSessionStore.getState().sessions[tabId]
  if (session?.state === 'connecting' && !window.confirm('연결 중인 세션을 닫을까요?')) {
    return
  }
  disposeSession(tabId)
  useTabStore.getState().removeTab(tabId)
}
