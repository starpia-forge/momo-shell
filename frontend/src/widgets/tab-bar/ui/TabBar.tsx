import { useEffect, useRef, useState, type DragEvent, type MouseEvent as ReactMouseEvent } from 'react'
import { useTabStore, type Tab } from '../model/store'
import { useSessionStore, type SessionState } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { cn } from '../../../shared/lib/cn'
import { decodePaneDrag, isPaneDrag, type PaneDragPayload } from '../../../shared/lib/paneDnd'
import { createLocalTab, createSSHTab } from '../lib/createTab'

const HOVER_ACTIVATE_MS = 500

const DOT: Record<SessionState | 'idle', string> = {
  running: 'bg-success',
  connecting: 'bg-warning animate-pulse-dot',
  error: 'bg-danger',
  starting: 'bg-line',
  closed: 'bg-line',
  idle: 'bg-line',
}

interface TabBarProps {
  onCloseTab: (tab: Tab) => void
  /** A dragged pane was dropped on tab `targetTabId`, or on empty tab-bar space (null). */
  onPaneDrop: (payload: PaneDragPayload, targetTabId: string | null) => void
}

export function TabBar({ onCloseTab, onPaneDrop }: TabBarProps) {
  const tabs = useTabStore((s) => s.tabs)
  const activeId = useTabStore((s) => s.activeId)
  const setActive = useTabStore((s) => s.setActive)
  const reorder = useTabStore((s) => s.reorder)
  const popoverOpen = useTabStore((s) => s.newTabPopoverOpen)
  const openPopover = useTabStore((s) => s.openNewTabPopover)
  const closePopover = useTabStore((s) => s.closeNewTabPopover)
  const homeActive = useTabStore((s) => s.homeActive)
  const showHome = useTabStore((s) => s.showHome)
  const sessions = useSessionStore((s) => s.sessions)
  const hosts = useHostStore((s) => s.hosts)

  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const hoverRef = useRef<{ timer: number; tabId: string } | null>(null)

  useEffect(() => {
    if (!popoverOpen) return
    const onPointerDown = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) closePopover()
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [popoverOpen, closePopover])

  function clearHover() {
    if (hoverRef.current) {
      window.clearTimeout(hoverRef.current.timer)
      hoverRef.current = null
    }
  }

  function handleReorderDrop(index: number) {
    if (dragIndex !== null && dragIndex !== index) reorder(dragIndex, index)
    setDragIndex(null)
  }

  function handleMiddleClick(e: ReactMouseEvent, tab: Tab) {
    if (e.button === 1) {
      e.preventDefault()
      onCloseTab(tab)
    }
  }

  // Always preventDefault (needed for both tab-reorder and pane-drop to
  // register as a valid drop target); a pane drag additionally starts a
  // hover-to-activate timer so the user can keep dragging into that tab's
  // panes once it comes to the front.
  function handleTabDragOver(e: DragEvent, tab: Tab) {
    e.preventDefault()
    if (!isPaneDrag(e.dataTransfer)) return
    if (hoverRef.current?.tabId === tab.id) return
    clearHover()
    const timer = window.setTimeout(() => {
      setActive(tab.id)
      hoverRef.current = null
    }, HOVER_ACTIVATE_MS)
    hoverRef.current = { timer, tabId: tab.id }
  }

  function handleTabDragLeave(tab: Tab) {
    if (hoverRef.current?.tabId === tab.id) clearHover()
  }

  function handleTabDrop(e: DragEvent, tab: Tab, index: number) {
    e.preventDefault()
    e.stopPropagation()
    clearHover()
    if (isPaneDrag(e.dataTransfer)) {
      const payload = decodePaneDrag(e.dataTransfer)
      if (payload) onPaneDrop(payload, tab.id)
      return
    }
    handleReorderDrop(index)
  }

  function handleBarDragOver(e: DragEvent) {
    if (isPaneDrag(e.dataTransfer)) e.preventDefault()
  }

  function handleBarDrop(e: DragEvent) {
    if (!isPaneDrag(e.dataTransfer)) return
    e.preventDefault()
    const payload = decodePaneDrag(e.dataTransfer)
    if (payload) onPaneDrop(payload, null)
  }

  return (
    <div className="tab-bar flex-none flex items-stretch h-9 bg-surface border-b border-line" onDragOver={handleBarDragOver} onDrop={handleBarDrop}>
      <button
        className={cn(
          'flex-none flex items-center justify-center w-9 border-none border-r border-line bg-transparent text-muted cursor-pointer hover:text-fg',
          homeActive && 'text-fg bg-canvas',
        )}
        onClick={showHome}
        aria-label="홈"
      >
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 11 12 3l9 8" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M5 10v10h14V10" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      <div className="flex items-stretch min-w-0 overflow-x-auto">
        {tabs.map((tab, index) => {
          const state = sessions[tab.sessionId]?.state ?? 'idle'
          return (
            <div
              key={tab.id}
              className={cn(
                'flex items-center gap-1.5 min-w-30 max-w-45 px-2 py-1 border-r border-line cursor-pointer text-muted',
                tab.id === activeId && 'bg-canvas text-fg',
              )}
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e: DragEvent) => handleTabDragOver(e, tab)}
              onDragLeave={() => handleTabDragLeave(tab)}
              onDrop={(e: DragEvent) => handleTabDrop(e, tab, index)}
              onClick={() => setActive(tab.id)}
              onMouseDown={(e) => handleMiddleClick(e, tab)}
            >
              <span className={cn('flex-shrink-0 w-[7px] h-[7px] rounded-full', DOT[state])} />
              <div className="min-w-0 flex-1">
                <div className="text-[12px] overflow-hidden text-ellipsis whitespace-nowrap">{tab.title}</div>
                <div className="text-[10px] opacity-70 overflow-hidden text-ellipsis whitespace-nowrap">{tab.subtitle}</div>
              </div>
              <button
                className="flex-shrink-0 border-none bg-transparent text-inherit cursor-pointer text-[13px] leading-none opacity-60 hover:opacity-100"
                onClick={(e) => {
                  e.stopPropagation()
                  onCloseTab(tab)
                }}
                aria-label={`${tab.title} 닫기`}
              >
                ×
              </button>
            </div>
          )
        })}
      </div>

      {/* Own wrapper (not the scrollable tab-bar__tabs) so the popover isn't
          clipped -- overflow-x:auto on an ancestor forces its overflow-y to
          a non-visible value too, per spec, which would hide an
          absolutely-positioned dropdown anchored inside it. */}
      <div className="relative flex-none">
        <button
          className="w-9 h-full border-none border-r border-line bg-transparent text-muted text-[16px] cursor-pointer hover:text-fg"
          onClick={openPopover}
          aria-label="새 탭"
        >
          +
        </button>
        {popoverOpen && (
          <div
            className="absolute top-full left-0 z-150 flex flex-col min-w-40 bg-surface border border-line rounded p-1 shadow-float"
            ref={popoverRef}
          >
            <button
              className="px-2.5 py-1.5 border-none bg-transparent text-fg text-[13px] text-left rounded-sm cursor-pointer hover:bg-canvas"
              onClick={() => void createLocalTab()}
            >
              로컬 쉘
            </button>
            {Object.values(hosts).map((h) => (
              <button
                key={h.id}
                className="px-2.5 py-1.5 border-none bg-transparent text-fg text-[13px] text-left rounded-sm cursor-pointer hover:bg-canvas"
                onClick={() => void createSSHTab(h)}
              >
                {h.name}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
