import { getFontSize, setFontSize, resetFontSize } from '../entities/session'
import { useHistoryPanelStore } from '../widgets/history-panel'
import { useTabStore } from '../widgets/tab-bar'
import { closeLeafOrEscalate, splitFocused, useWorkspaceLayoutStore, type ArrowDirection } from '../widgets/workspace-layout'

const FONT_STEP = 1
const ARROW_DIRECTIONS: Record<string, ArrowDirection> = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
}

// Capture phase so this runs before xterm's own keydown handling -- otherwise
// Ctrl/Cmd+= would already have been forwarded to the shell as a keystroke by
// the time this listener saw it.
export function registerGlobalShortcuts(): () => void {
  const handler = (e: KeyboardEvent) => {
    if (e.altKey && !e.ctrlKey && !e.metaKey) {
      const direction = ARROW_DIRECTIONS[e.key]
      if (!direction) return
      e.preventDefault()
      e.stopPropagation()
      const tabId = useTabStore.getState().activeId
      if (tabId) useWorkspaceLayoutStore.getState().moveFocus(tabId, direction)
      return
    }

    if (!(e.ctrlKey || e.metaKey)) return

    if (e.key === '=' || e.key === '+') {
      e.preventDefault()
      e.stopPropagation()
      setFontSize(getFontSize() + FONT_STEP)
    } else if (e.key === '-') {
      e.preventDefault()
      e.stopPropagation()
      setFontSize(getFontSize() - FONT_STEP)
    } else if (e.key === '0') {
      e.preventDefault()
      e.stopPropagation()
      resetFontSize()
    } else if (e.shiftKey && e.key.toLowerCase() === 'd') {
      e.preventDefault()
      e.stopPropagation()
      const tabId = useTabStore.getState().activeId
      if (tabId) void splitFocused(tabId, 'row')
    } else if (e.shiftKey && e.key.toLowerCase() === 'e') {
      e.preventDefault()
      e.stopPropagation()
      const tabId = useTabStore.getState().activeId
      if (tabId) void splitFocused(tabId, 'column')
    } else if (e.shiftKey && e.key.toLowerCase() === 'w') {
      e.preventDefault()
      e.stopPropagation()
      const tabId = useTabStore.getState().activeId
      const leafId = tabId ? useWorkspaceLayoutStore.getState().focusedLeaf[tabId] : undefined
      if (tabId && leafId) closeLeafOrEscalate(tabId, leafId, () => useTabStore.getState().removeTab(tabId))
    } else if (e.key.toLowerCase() === 't') {
      e.preventDefault()
      e.stopPropagation()
      useTabStore.getState().openNewTabPopover()
    } else if (e.key.toLowerCase() === 'h') {
      e.preventDefault()
      e.stopPropagation()
      useHistoryPanelStore.getState().toggle()
    } else if (e.key >= '1' && e.key <= '9') {
      e.preventDefault()
      e.stopPropagation()
      useTabStore.getState().activateByIndex(Number(e.key) - 1)
    }
  }

  window.addEventListener('keydown', handler, true)
  return () => window.removeEventListener('keydown', handler, true)
}
