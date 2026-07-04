import { useEffect, useRef } from 'react'
import { TerminalPane } from '../../../entities/session'
import type { Host } from '../../../entities/host'
import { HostKeyPrompt } from '../../../features/session-connect'
import { HostSidebar } from '../../../widgets/host-sidebar'
import { StatusBar } from '../../../widgets/status-bar'
import { TabBar, createLocalTab, useTabStore } from '../../../widgets/tab-bar'
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

  return (
    <div className="workspace">
      <TabBar />
      <div className="workspace__body">
        <div className="workspace__sidebar">
          <HostSidebar onConnect={handleHostConnect} />
        </div>
        <div className="workspace__pane">{activeTab && <TerminalPane sessionId={activeTab.sessionId} />}</div>
      </div>
      <StatusBar sessionId={activeTab?.sessionId ?? null} />
      <HostKeyPrompt />
    </div>
  )
}
