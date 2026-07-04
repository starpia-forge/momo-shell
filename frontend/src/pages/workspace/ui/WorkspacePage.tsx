import { useEffect, useLayoutEffect, useRef } from 'react'
import { disposeSession, useSessionStore } from '../../../entities/session'
import type { Host } from '../../../entities/host'
import { HostKeyPrompt } from '../../../features/session-connect'
import { HostSidebar } from '../../../widgets/host-sidebar'
import { StatusBar } from '../../../widgets/status-bar'
import { TabBar, createLocalTab, useTabStore, type Tab } from '../../../widgets/tab-bar'
import { WorkspaceLayout, findLeaf, leaves, useWorkspaceLayoutStore } from '../../../widgets/workspace-layout'
import './WorkspacePage.css'

export function WorkspacePage() {
  const tabs = useTabStore((s) => s.tabs)
  const activeId = useTabStore((s) => s.activeId)
  const trees = useWorkspaceLayoutStore((s) => s.trees)
  const focusedLeafMap = useWorkspaceLayoutStore((s) => s.focusedLeaf)
  const opened = useRef(false)

  useEffect(() => {
    if (opened.current) return
    opened.current = true
    void createLocalTab()
  }, [])

  const activeTab = tabs.find((t) => t.id === activeId)

  // Every tab needs exactly one layout tree; a new tab's initial tree is a
  // single leaf seeded from its own (id, sessionId) pair. Trees for removed
  // tabs are pruned here too, as a defensive backstop -- the normal close
  // path (handleCloseTab / handleTabBecameEmpty) already removes its own
  // tree directly. Runs as a layout effect so a freshly-added tab's tree
  // exists before paint, instead of flashing an empty pane for one frame.
  useLayoutEffect(() => {
    const store = useWorkspaceLayoutStore.getState()
    const tabIds = new Set(tabs.map((t) => t.id))
    for (const tab of tabs) {
      if (!store.trees[tab.id]) store.ensureTree(tab.id, tab.sessionId, tab.id)
    }
    for (const tid of Object.keys(store.trees)) {
      if (!tabIds.has(tid)) store.removeTree(tid)
    }
  }, [tabs])

  // Tab.sessionId tracks the active tab's *focused* pane -- StatusBar and
  // the tab-bar status dot both key off it. Reconnects (and, once splitting
  // exists, focus changes) repoint the focused leaf's session, so mirror
  // that back onto the tab here.
  useLayoutEffect(() => {
    if (!activeTab) return
    const tree = trees[activeTab.id]
    const focusedLeafId = focusedLeafMap[activeTab.id]
    if (!tree || !focusedLeafId) return
    const leaf = findLeaf(tree, focusedLeafId)
    if (leaf && leaf.sessionId !== activeTab.sessionId) {
      useTabStore.getState().replaceSession(activeTab.id, leaf.sessionId)
    }
  }, [activeTab, trees, focusedLeafMap])

  // HostSidebar (a widget) can't import the tab-bar widget directly under
  // FSD's same-layer rule, so the page bridges "host connected" -> "create
  // its tab" here.
  function handleHostConnect(host: Host, sessionId: string) {
    useTabStore.getState().addTab({
      id: sessionId,
      kind: 'ssh',
      hostId: host.id,
      sessionId,
      title: host.name,
      subtitle: host.address,
    })
  }

  // Same bridging reason: closing a tab means disposing every leaf session
  // in its workspace-layout tree, then removing the tab-bar entry -- neither
  // widget may import the other.
  function handleCloseTab(tab: Tab) {
    const tree = useWorkspaceLayoutStore.getState().trees[tab.id]
    const sessionIds = tree ? leaves(tree).map((l) => l.sessionId) : [tab.sessionId]
    const anyConnecting = sessionIds.some((id) => useSessionStore.getState().sessions[id]?.state === 'connecting')
    if (anyConnecting && !window.confirm('연결 중인 세션을 닫을까요?')) return
    sessionIds.forEach(disposeSession)
    useWorkspaceLayoutStore.getState().removeTree(tab.id)
    useTabStore.getState().removeTab(tab.id)
  }

  // WorkspaceLayout already disposed the last leaf's session and removed
  // its own (now-empty) tree before calling this -- only the tab itself
  // remains to be removed.
  function handleTabBecameEmpty(tabId: string) {
    useTabStore.getState().removeTab(tabId)
  }

  return (
    <div className="workspace">
      <TabBar onCloseTab={handleCloseTab} />
      <div className="workspace__body">
        <div className="workspace__sidebar">
          <HostSidebar onConnect={handleHostConnect} />
        </div>
        <div className="workspace__pane">
          {activeTab && <WorkspaceLayout key={activeTab.id} tabId={activeTab.id} onTabBecameEmpty={handleTabBecameEmpty} />}
        </div>
      </div>
      <StatusBar sessionId={activeTab?.sessionId ?? null} />
      <HostKeyPrompt />
    </div>
  )
}
