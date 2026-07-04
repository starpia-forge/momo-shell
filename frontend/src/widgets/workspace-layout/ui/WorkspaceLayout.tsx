import { Fragment } from 'react'
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels'
import { confirmSessionClose, disposeSession, openSSHSession, TerminalPane, useSessionStore } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { useWorkspaceLayoutStore } from '../model/store'
import type { LeafNode, PaneNode } from '../model/tree'
import './WorkspaceLayout.css'

interface WorkspaceLayoutProps {
  tabId: string
  /** The leaf tree became empty (its last pane closed) -- the page owns tab removal. */
  onTabBecameEmpty: (tabId: string) => void
}

export function WorkspaceLayout({ tabId, onTabBecameEmpty }: WorkspaceLayoutProps) {
  const tree = useWorkspaceLayoutStore((s) => s.trees[tabId])
  if (!tree) return null
  return (
    <div className="workspace-layout">
      <LayoutNode tabId={tabId} node={tree} onTabBecameEmpty={onTabBecameEmpty} />
    </div>
  )
}

interface LayoutNodeProps {
  tabId: string
  node: PaneNode
  onTabBecameEmpty: (tabId: string) => void
}

function LayoutNode({ tabId, node, onTabBecameEmpty }: LayoutNodeProps) {
  const setSizes = useWorkspaceLayoutStore((s) => s.setSizes)

  if (node.type === 'leaf') {
    return <PaneView tabId={tabId} leaf={node} onTabBecameEmpty={onTabBecameEmpty} />
  }

  const direction = node.direction === 'row' ? 'horizontal' : 'vertical'
  return (
    <PanelGroup direction={direction} onLayout={(sizes) => setSizes(tabId, node.id, sizes.map((s) => s / 100))}>
      {node.children.map((child, i) => (
        <Fragment key={child.id}>
          {i > 0 && <PanelResizeHandle className="workspace-layout__handle" />}
          <Panel defaultSize={node.sizes[i] * 100} minSize={15}>
            <LayoutNode tabId={tabId} node={child} onTabBecameEmpty={onTabBecameEmpty} />
          </Panel>
        </Fragment>
      ))}
    </PanelGroup>
  )
}

interface PaneViewProps {
  tabId: string
  leaf: LeafNode
  onTabBecameEmpty: (tabId: string) => void
}

function PaneView({ tabId, leaf, onTabBecameEmpty }: PaneViewProps) {
  const session = useSessionStore((s) => s.sessions[leaf.sessionId])
  const hosts = useHostStore((s) => s.hosts)
  const isSSH = session?.kind === 'ssh'
  const host = isSSH && session?.hostId ? hosts[session.hostId] : undefined
  const title = isSSH ? (host?.name ?? '연결 중...') : '로컬 쉘'
  const subtitle = isSSH ? (host?.address ?? '') : (session?.shell ?? '')

  async function handleReconnect() {
    if (!isSSH || !session?.hostId) return
    disposeSession(leaf.sessionId)
    const newSessionId = await openSSHSession(session.hostId, 80, 24)
    useWorkspaceLayoutStore.getState().replaceLeafSession(tabId, leaf.id, newSessionId)
  }

  function handleClose() {
    if (!confirmSessionClose(leaf.sessionId)) return
    disposeSession(leaf.sessionId)
    const result = useWorkspaceLayoutStore.getState().closeLeaf(tabId, leaf.id)
    if (result?.becameEmpty) onTabBecameEmpty(tabId)
  }

  return (
    <div className="pane-view" onClick={() => useWorkspaceLayoutStore.getState().setFocus(tabId, leaf.id)}>
      <div className="pane-view__header">
        <div className="pane-view__text">
          <span className="pane-view__title">{title}</span>
          {subtitle && <span className="pane-view__subtitle">{subtitle}</span>}
        </div>
        <button className="pane-view__close" onClick={handleClose} aria-label={`${title} 닫기`}>
          ×
        </button>
      </div>
      <div className="pane-view__body">
        <TerminalPane key={leaf.sessionId} sessionId={leaf.sessionId} onReconnect={isSSH ? () => void handleReconnect() : undefined} />
      </div>
    </div>
  )
}
