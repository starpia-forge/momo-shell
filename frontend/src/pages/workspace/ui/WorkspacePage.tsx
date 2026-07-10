import { useLayoutEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { disposeSession, useSessionStore } from '../../../entities/session'
import { useHostStore, type Host } from '../../../entities/host'
import { DestinationBar } from '../../../features/file-upload'
import { MCPApprovalDialog } from '../../../features/mcp-approval'
import { DelegationBadge, DelegationPanel } from '../../../features/mcp-control'
import { MCPPairApprovalDialog } from '../../../features/mcp-pairing'
import { PairApprovalDialog } from '../../../features/peer-pairing'
import { HostKeyPrompt } from '../../../features/session-connect'
import type { PaneDragPayload } from '../../../shared/lib/paneDnd'
import { HomePage } from '../../home'
import { SettingsPage } from '../../settings'
import { SftpPage } from '../../sftp'
import { AuditPanel } from '../../../widgets/audit-panel'
import { FileBrowserPanel } from '../../../widgets/file-browser'
import { HistoryPanel } from '../../../widgets/history-panel'
import { HostSidebar } from '../../../widgets/host-sidebar'
import { RightDock } from '../../../widgets/right-dock'
import { SharePanel } from '../../../widgets/share-panel'
import { StatusBar } from '../../../widgets/status-bar'
import { TabBar, useTabStore, type Tab } from '../../../widgets/tab-bar'
import { TransferCenter, TransferBadge } from '../../../widgets/transfer-center'
import { WorkspaceLayout, findLeaf, leaves, useWorkspaceLayoutStore } from '../../../widgets/workspace-layout'

export function WorkspacePage() {
  const { t } = useTranslation()
  const tabs = useTabStore((s) => s.tabs)
  const activeId = useTabStore((s) => s.activeId)
  const screen = useTabStore((s) => s.screen)
  const trees = useWorkspaceLayoutStore((s) => s.trees)
  const focusedLeafMap = useWorkspaceLayoutStore((s) => s.focusedLeaf)

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

  // A shared host connected via CreateSSHDirect has no saved Host row (no
  // hostId), so the tab title/subtitle come from the shared host's own
  // name/address instead.
  function handleSharedConnect(name: string, address: string, sessionId: string) {
    useTabStore.getState().addTab({
      id: sessionId,
      kind: 'ssh',
      sessionId,
      title: name,
      subtitle: address,
    })
  }

  // Same bridging reason: closing a tab means disposing every leaf session
  // in its workspace-layout tree, then removing the tab-bar entry -- neither
  // widget may import the other.
  function handleCloseTab(tab: Tab) {
    const tree = useWorkspaceLayoutStore.getState().trees[tab.id]
    const sessionIds = tree ? leaves(tree).map((l) => l.sessionId) : [tab.sessionId]
    const anyConnecting = sessionIds.some((id) => useSessionStore.getState().sessions[id]?.state === 'connecting')
    if (anyConnecting && !window.confirm(t('workspace.confirmCloseConnecting'))) return
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

  // Dropping a dragged pane onto a tab: hovering a tab already activates it
  // (TabBar's own timer) so the drag can continue onto one of its panes --
  // an actual drop on the tab item itself just finalizes that activation.
  // Dropping on empty tab-bar space detaches the pane into a brand new tab.
  function handlePaneDrop(payload: PaneDragPayload, targetTabId: string | null) {
    if (targetTabId !== null) {
      useTabStore.getState().setActive(targetTabId)
      return
    }

    const sourceTree = useWorkspaceLayoutStore.getState().trees[payload.tabId]
    if (sourceTree && leaves(sourceTree).length <= 1) return // already alone in its tab

    const removed = useWorkspaceLayoutStore.getState().closeLeaf(payload.tabId, payload.leafId)
    if (!removed) return

    const session = useSessionStore.getState().sessions[payload.sessionId]
    const host = session?.hostId ? useHostStore.getState().hosts[session.hostId] : undefined
    useTabStore.getState().addTab({
      id: payload.sessionId,
      kind: session?.kind === 'ssh' ? 'ssh' : 'local',
      hostId: session?.hostId,
      sessionId: payload.sessionId,
      title: session?.kind === 'ssh' ? (host?.name ?? t('common.connecting')) : t('tabBar.localShell'),
      subtitle: session?.kind === 'ssh' ? (host?.address ?? '') : (session?.shell ?? ''),
    })
    if (removed.becameEmpty) useTabStore.getState().removeTab(payload.tabId)
  }

  return (
    <div className="workspace flex flex-col h-screen w-screen bg-canvas text-fg">
      <TabBar
        onCloseTab={handleCloseTab}
        onPaneDrop={handlePaneDrop}
        trailing={
          <>
            <TransferBadge />
            <DelegationBadge />
          </>
        }
      />
      <div className="flex-1 flex min-h-0">
        {screen === 'settings' ? (
          <SettingsPage />
        ) : screen === 'sftp' ? (
          <SftpPage />
        ) : screen === 'home' ? (
          <HomePage onConnect={handleHostConnect} onConnectShared={handleSharedConnect} />
        ) : (
          <>
            <div className="flex-none w-59">
              <HostSidebar
                onConnect={handleHostConnect}
                onConnectShared={handleSharedConnect}
                activeHostId={activeTab?.kind === 'ssh' ? activeTab.hostId : undefined}
              />
            </div>
            <div className="flex-1 min-w-0 min-h-0 p-1 overflow-hidden">
              {activeTab && <WorkspaceLayout key={activeTab.id} tabId={activeTab.id} onTabBecameEmpty={handleTabBecameEmpty} />}
            </div>
            <RightDock
              historyPanel={
                <HistoryPanel
                  focusedSessionId={activeTab?.sessionId ?? null}
                  currentHostId={activeTab ? (activeTab.kind === 'ssh' ? (activeTab.hostId ?? '') : 'local') : ''}
                />
              }
              filesPanel={<FileBrowserPanel sessionId={activeTab?.sessionId ?? null} isSSH={activeTab?.kind === 'ssh'} />}
              sharePanel={<SharePanel />}
              auditPanel={<AuditPanel />}
            />
          </>
        )}
      </div>
      <StatusBar sessionId={activeTab?.sessionId ?? null} />
      <TransferCenter />
      <DelegationPanel />
      <HostKeyPrompt />
      <PairApprovalDialog />
      <MCPPairApprovalDialog />
      <MCPApprovalDialog />
      <DestinationBar />
    </div>
  )
}
