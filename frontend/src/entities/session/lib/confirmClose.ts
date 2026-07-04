import { useSessionStore } from '../model/store'

/** Shared confirm-before-close check, used when closing a tab or a single pane. */
export function confirmSessionClose(sessionId: string): boolean {
  const session = useSessionStore.getState().sessions[sessionId]
  if (session?.state === 'connecting') return window.confirm('연결 중인 세션을 닫을까요?')
  return true
}
