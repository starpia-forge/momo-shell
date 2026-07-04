import { disposeSession, useSessionStore } from '../../../entities/session'
import { useTabStore, type Tab } from '../model/store'

// Closing a tab always closes its session -- Phase 3's split panes are the
// only future case where a tab could outlive one of its sessions, and that
// doesn't exist yet.
export function closeTab(tab: Tab): void {
  const session = useSessionStore.getState().sessions[tab.sessionId]
  if (session?.state === 'connecting' && !window.confirm('연결 중인 세션을 닫을까요?')) {
    return
  }
  disposeSession(tab.sessionId)
  useTabStore.getState().removeTab(tab.id)
}
