import { useEffect, useRef } from 'react'
import { TerminalPane, openSSHSession, disposeSession } from '../../../entities/session'
import type { Host } from '../../../entities/host'
import { HostKeyPrompt } from '../../../features/session-connect'
import { HostSidebar } from '../../../widgets/host-sidebar'
import { StatusBar } from '../../../widgets/status-bar'
import { TabBar, createLocalTab, useTabStore, type Tab } from '../../../widgets/tab-bar'
import './WorkspacePage.css'

export function WorkspacePage() {
  const tabs = useTabStore((s) => s.tabs)
  const activeId = useTabStore((s) => s.activeId)
  const opened = useRef(false)

  useEffect(() => {
    if (opened.current) return
    opened.current = true
    void createLocalTab()
  }, [])

  const activeTab = tabs.find((t) => t.id === activeId)

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

  // Same bridging reason: TerminalPane (an entity) fires onReconnect, and
  // repointing the tab at the new session is tab-bar's store to own.
  async function handleReconnect(tab: Tab) {
    if (!tab.hostId) return
    disposeSession(tab.sessionId)
    const newSessionId = await openSSHSession(tab.hostId, 80, 24)
    useTabStore.getState().replaceSession(tab.id, newSessionId)
  }

  return (
    <div className="workspace">
      <TabBar />
      <div className="workspace__body">
        <div className="workspace__sidebar">
          <HostSidebar onConnect={handleHostConnect} />
        </div>
        <div className="workspace__pane">
          {activeTab && (
            // key=sessionId: a reconnect repoints this same tab at a new
            // session id without remounting the *tab*, but the pane must
            // still get a fresh mount (fresh container, fresh attach) rather
            // than updating in place.
            <TerminalPane
              key={activeTab.sessionId}
              sessionId={activeTab.sessionId}
              onReconnect={activeTab.kind === 'ssh' ? () => void handleReconnect(activeTab) : undefined}
            />
          )}
        </div>
      </div>
      <StatusBar sessionId={activeTab?.sessionId ?? null} />
      <HostKeyPrompt />
    </div>
  )
}
