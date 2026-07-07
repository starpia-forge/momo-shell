import { useEffect, useRef, useState, type DragEvent, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react'
import { useTabStore, type Tab } from '../model/store'
import { useSessionStore, type SessionState } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { cn } from '../../../shared/lib/cn'
import { decodePaneDrag, isPaneDrag, type PaneDragPayload } from '../../../shared/lib/paneDnd'
import { IconButton, StatusDot, type DotStatus } from '../../../shared/ui'
import { createLocalTab, createSSHTab } from '../lib/createTab'

const HOVER_ACTIVATE_MS = 500

const DOT_STATUS: Record<SessionState | 'idle', DotStatus> = {
  running: 'running',
  connecting: 'connecting',
  error: 'error',
  starting: 'idle',
  closed: 'idle',
  idle: 'idle',
}

interface TabBarProps {
  onCloseTab: (tab: Tab) => void
  /** A dragged pane was dropped on tab `targetTabId`, or on empty tab-bar space (null). */
  onPaneDrop: (payload: PaneDragPayload, targetTabId: string | null) => void
  /** Slot rendered before the settings button (e.g. the transfer-center badge). */
  trailing?: ReactNode
}

export function TabBar({ onCloseTab, onPaneDrop, trailing }: TabBarProps) {
  const tabs = useTabStore((s) => s.tabs)
  const activeId = useTabStore((s) => s.activeId)
  const setActive = useTabStore((s) => s.setActive)
  const reorder = useTabStore((s) => s.reorder)
  const popoverOpen = useTabStore((s) => s.newTabPopoverOpen)
  const openPopover = useTabStore((s) => s.openNewTabPopover)
  const closePopover = useTabStore((s) => s.closeNewTabPopover)
  const screen = useTabStore((s) => s.screen)
  const showHome = useTabStore((s) => s.showHome)
  const showSettings = useTabStore((s) => s.showSettings)
  const showSftp = useTabStore((s) => s.showSftp)
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
    <div
      className="tab-bar flex-none flex items-center h-14 gap-2 px-4 bg-surface border-b border-line"
      onDragOver={handleBarDragOver}
      onDrop={handleBarDrop}
    >
      <div className="flex-none w-7.5 h-7.5 mr-1.5 rounded-md bg-accent flex items-center justify-center text-on-accent font-bold text-[15px] font-mono">
        m
      </div>

      <IconButton active={screen === 'home'} onClick={showHome} aria-label="홈">
        <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 11 12 3l9 8" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M5 10v10h14V10" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </IconButton>
      <button
        className={cn(
          'flex-none px-4 py-2 rounded-md bg-transparent text-fg2 text-[13px] font-medium cursor-pointer hover:text-fg',
          screen === 'sftp' && 'bg-surface2 text-fg font-bold',
        )}
        onClick={showSftp}
        aria-label="SFTP"
      >
        SFTP
      </button>

      <div className="flex-none w-px h-6 bg-line mx-1" />

      <div className="flex items-stretch gap-2 min-w-0 overflow-x-auto">
        {tabs.map((tab, index) => {
          const state = sessions[tab.sessionId]?.state ?? 'idle'
          const active = tab.id === activeId
          return (
            <div
              key={tab.id}
              className={cn(
                'flex-none flex items-center gap-2 w-44 box-border px-3 py-1.5 rounded-md border border-line cursor-pointer text-fg2',
                active && 'bg-surface2 text-fg',
              )}
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e: DragEvent) => handleTabDragOver(e, tab)}
              onDragLeave={() => handleTabDragLeave(tab)}
              onDrop={(e: DragEvent) => handleTabDrop(e, tab, index)}
              onClick={() => setActive(tab.id)}
              onMouseDown={(e) => handleMiddleClick(e, tab)}
            >
              <StatusDot status={DOT_STATUS[state]} />
              <div className="min-w-0 flex-1 leading-tight">
                <div className={cn('text-[12.5px] overflow-hidden text-ellipsis whitespace-nowrap', active && 'font-bold')}>
                  {tab.title}
                </div>
                <div className="text-[10.5px] font-mono text-fg3 overflow-hidden text-ellipsis whitespace-nowrap">
                  {tab.subtitle}
                </div>
              </div>
              <button
                className="flex-shrink-0 border-none bg-transparent text-fg3 cursor-pointer text-[13px] leading-none opacity-60 hover:opacity-100"
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

      {/* Own wrapper (not the scrollable tab list) so the popover isn't
          clipped -- overflow-x:auto on an ancestor forces its overflow-y to
          a non-visible value too, per spec, which would hide an
          absolutely-positioned dropdown anchored inside it. */}
      <div className="relative flex-none">
        <IconButton onClick={openPopover} aria-label="새 탭" className="text-[18px]">
          +
        </IconButton>
        {popoverOpen && (
          <div
            className="absolute top-full left-0 z-150 flex flex-col min-w-40 bg-surface2 border border-line rounded-lg p-1.5 shadow-menu"
            ref={popoverRef}
          >
            <button
              className="px-3 py-2 border-none bg-transparent text-fg text-[12.5px] text-left rounded-md cursor-pointer hover:bg-accent/14"
              onClick={() => void createLocalTab()}
            >
              로컬 쉘
            </button>
            {Object.values(hosts).map((h) => (
              <button
                key={h.id}
                className="px-3 py-2 border-none bg-transparent text-fg text-[12.5px] text-left rounded-md cursor-pointer hover:bg-accent/14"
                onClick={() => void createSSHTab(h)}
              >
                {h.name}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex-1" />

      {trailing}

      <IconButton active={screen === 'settings'} onClick={showSettings} aria-label="설정">
        <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="3" />
          <path
            d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1.08-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </IconButton>
    </div>
  )
}
