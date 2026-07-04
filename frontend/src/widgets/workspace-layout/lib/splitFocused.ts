import { openLocalSession, openSSHSession, useSessionStore } from '../../../entities/session'
import { useWorkspaceLayoutStore } from '../model/store'
import { findLeaf, type SplitDirection } from '../model/tree'

/**
 * Splits the tab's currently-focused pane, opening a fresh session of the
 * same kind (same host for SSH, a new local shell for local) and inserting
 * it after the focused leaf in the given direction.
 */
export async function splitFocused(tabId: string, direction: SplitDirection): Promise<void> {
  const layout = useWorkspaceLayoutStore.getState()
  const tree = layout.trees[tabId]
  const focusedLeafId = layout.focusedLeaf[tabId]
  if (!tree || !focusedLeafId) return
  const focusedLeaf = findLeaf(tree, focusedLeafId)
  if (!focusedLeaf) return

  const session = useSessionStore.getState().sessions[focusedLeaf.sessionId]
  const newSessionId =
    session?.kind === 'ssh' && session.hostId
      ? await openSSHSession(session.hostId, 80, 24)
      : await openLocalSession({ cols: 80, rows: 24 })

  useWorkspaceLayoutStore
    .getState()
    .splitLeaf(tabId, focusedLeafId, direction, crypto.randomUUID(), newSessionId, crypto.randomUUID())
}
