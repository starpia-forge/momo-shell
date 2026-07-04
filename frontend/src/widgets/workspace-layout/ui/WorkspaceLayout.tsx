import { Fragment, useRef, useState } from 'react'
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels'
import { disposeSession, openSSHSession, TerminalPane, useSessionStore } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { ContextMenu, type ContextMenuItem } from '../../../shared/ui'
import { closeLeafOrEscalate } from '../lib/closeLeaf'
import { splitFocused } from '../lib/splitFocused'
import { useWorkspaceLayoutStore } from '../model/store'
import type { LeafNode, PaneNode } from '../model/tree'
import './WorkspaceLayout.css'

const DOUBLE_CLICK_MS = 300

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
  const version = useWorkspaceLayoutStore((s) => s.splitVersion[node.id] ?? 0)

  if (node.type === 'leaf') {
    return <PaneView tabId={tabId} leaf={node} onTabBecameEmpty={onTabBecameEmpty} />
  }

  const direction = node.direction === 'row' ? 'horizontal' : 'vertical'
  return (
    <PanelGroup key={version} direction={direction} onLayout={(sizes) => setSizes(tabId, node.id, sizes.map((s) => s / 100))}>
      {node.children.map((child, i) => (
        <Fragment key={child.id}>
          {i > 0 && <EqualizeHandle onEqualize={() => useWorkspaceLayoutStore.getState().equalize(tabId, node.id)} />}
          <Panel defaultSize={node.sizes[i] * 100} minSize={15}>
            <LayoutNode tabId={tabId} node={child} onTabBecameEmpty={onTabBecameEmpty} />
          </Panel>
        </Fragment>
      ))}
    </PanelGroup>
  )
}

function EqualizeHandle({ onEqualize }: { onEqualize: () => void }) {
  const lastClickRef = useRef(0)

  function handleClick() {
    const now = Date.now()
    if (now - lastClickRef.current < DOUBLE_CLICK_MS) {
      onEqualize()
      lastClickRef.current = 0
    } else {
      lastClickRef.current = now
    }
  }

  return <PanelResizeHandle className="workspace-layout__handle" onClick={handleClick} />
}

interface PaneViewProps {
  tabId: string
  leaf: LeafNode
  onTabBecameEmpty: (tabId: string) => void
}

function PaneView({ tabId, leaf, onTabBecameEmpty }: PaneViewProps) {
  const session = useSessionStore((s) => s.sessions[leaf.sessionId])
  const hosts = useHostStore((s) => s.hosts)
  const isFocused = useWorkspaceLayoutStore((s) => s.focusedLeaf[tabId] === leaf.id)
  const [menuPos, setMenuPos] = useState<{ x: number; y: number } | null>(null)
  const isSSH = session?.kind === 'ssh'
  const host = isSSH && session?.hostId ? hosts[session.hostId] : undefined
  const title = isSSH ? (host?.name ?? '연결 중...') : '로컬 쉘'
  const subtitle = isSSH ? (host?.address ?? '') : (session?.shell ?? '')

  function focusThis() {
    useWorkspaceLayoutStore.getState().setFocus(tabId, leaf.id)
  }

  async function handleReconnect() {
    if (!isSSH || !session?.hostId) return
    disposeSession(leaf.sessionId)
    const newSessionId = await openSSHSession(session.hostId, 80, 24)
    useWorkspaceLayoutStore.getState().replaceLeafSession(tabId, leaf.id, newSessionId)
  }

  function handleClose() {
    closeLeafOrEscalate(tabId, leaf.id, onTabBecameEmpty)
  }

  function openContextMenu(e: React.MouseEvent) {
    e.preventDefault()
    focusThis()
    setMenuPos({ x: e.clientX, y: e.clientY })
  }

  const contextMenuItems: ContextMenuItem[] = [
    { label: '오른쪽에 분할', onClick: () => void splitFocused(tabId, 'row') },
    { label: '아래에 분할', onClick: () => void splitFocused(tabId, 'column') },
    { label: '닫기', danger: true, onClick: handleClose },
  ]

  return (
    <div className={`pane-view ${isFocused ? 'pane-view--focused' : ''}`} onClick={focusThis}>
      <div className="pane-view__header" onContextMenu={openContextMenu}>
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
      {menuPos && <ContextMenu x={menuPos.x} y={menuPos.y} items={contextMenuItems} onClose={() => setMenuPos(null)} />}
    </div>
  )
}
