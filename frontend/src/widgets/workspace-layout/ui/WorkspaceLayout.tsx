import { Fragment, useRef, useState, type DragEvent } from 'react'
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels'
import { disposeSession, openSSHSession, TerminalPane, useSessionStore } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { AltDragOverlay, computeDropZone, dragSourceProps, DropZoneOverlay } from '../../../features/pane-dnd'
import { SearchOverlay, useTerminalSearchStore } from '../../../features/terminal-search'
import { decodePaneDrag, isPaneDrag, type DropZone } from '../../../shared/lib/paneDnd'
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
  const searchOpen = useTerminalSearchStore((s) => s.openForLeafId === leaf.id)
  const [menuPos, setMenuPos] = useState<{ x: number; y: number } | null>(null)
  const [dropZone, setDropZone] = useState<DropZone | null>(null)
  const isSSH = session?.kind === 'ssh'
  const host = isSSH && session?.hostId ? hosts[session.hostId] : undefined
  const title = isSSH ? (host?.name ?? '연결 중...') : '로컬 쉘'
  const subtitle = isSSH ? (host?.address ?? '') : (session?.shell ?? '')
  const dragPayload = { tabId, leafId: leaf.id, sessionId: leaf.sessionId }

  function focusThis() {
    useWorkspaceLayoutStore.getState().setFocus(tabId, leaf.id)
  }

  function handleDragOver(e: DragEvent) {
    if (!isPaneDrag(e.dataTransfer)) return
    e.preventDefault()
    const rect = e.currentTarget.getBoundingClientRect()
    const relX = (e.clientX - rect.left) / rect.width
    const relY = (e.clientY - rect.top) / rect.height
    setDropZone(computeDropZone(relX, relY))
  }

  function handleDragLeave(e: DragEvent) {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDropZone(null)
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault()
    const zone = dropZone
    setDropZone(null)
    if (!zone) return
    const payload = decodePaneDrag(e.dataTransfer)
    if (!payload || payload.leafId === leaf.id) return

    const layout = useWorkspaceLayoutStore.getState()
    if (payload.tabId === tabId) {
      layout.moveLeafInTab(tabId, payload.leafId, leaf.id, zone, crypto.randomUUID())
      return
    }

    // Cross-tab: this pane arrived here via the tab bar's hover-to-activate
    // (dragging over a tab for 500ms switches to it, letting the drag
    // continue onto one of its panes). Detach the leaf from its source
    // tab's tree (without touching the session itself) and insert it here.
    const removed = layout.closeLeaf(payload.tabId, payload.leafId)
    if (!removed) return
    layout.insertLeafAtZone(tabId, leaf.id, zone, payload.leafId, removed.sessionId, crypto.randomUUID())
    if (removed.becameEmpty) onTabBecameEmpty(payload.tabId)
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
    <div
      className={`pane-view ${isFocused ? 'pane-view--focused' : ''}`}
      onClick={focusThis}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <div className="pane-view__header" onContextMenu={openContextMenu} {...dragSourceProps(dragPayload)}>
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
        <AltDragOverlay payload={dragPayload} />
        {searchOpen && (
          <SearchOverlay sessionId={leaf.sessionId} onClose={() => useTerminalSearchStore.getState().close()} />
        )}
      </div>
      <DropZoneOverlay zone={dropZone} />
      {menuPos && <ContextMenu x={menuPos.x} y={menuPos.y} items={contextMenuItems} onClose={() => setMenuPos(null)} />}
    </div>
  )
}
