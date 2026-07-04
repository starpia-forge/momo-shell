import { confirmSessionClose, disposeSession } from '../../../entities/session'
import { useWorkspaceLayoutStore } from '../model/store'
import { findLeaf } from '../model/tree'

/**
 * Closes one leaf (confirming first if its session is still connecting),
 * escalating to `onTabBecameEmpty` if it was the tab's last pane. Shared by
 * PaneView's own close button and the Ctrl/Cmd+Shift+W shortcut so both
 * paths behave identically.
 */
export function closeLeafOrEscalate(tabId: string, leafId: string, onTabBecameEmpty: (tabId: string) => void): void {
  const tree = useWorkspaceLayoutStore.getState().trees[tabId]
  const leaf = tree ? findLeaf(tree, leafId) : null
  if (!leaf) return
  if (!confirmSessionClose(leaf.sessionId)) return

  disposeSession(leaf.sessionId)
  const result = useWorkspaceLayoutStore.getState().closeLeaf(tabId, leafId)
  if (result?.becameEmpty) onTabBecameEmpty(tabId)
}
