import { useEffect, useRef, useState, type DragEvent, type MouseEvent as ReactMouseEvent } from 'react'
import { useTabStore, type Tab } from '../model/store'
import { useSessionStore } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { decodePaneDrag, isPaneDrag, type PaneDragPayload } from '../../../shared/lib/paneDnd'
import { createLocalTab, createSSHTab } from '../lib/createTab'
import './TabBar.css'

const HOVER_ACTIVATE_MS = 500

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
    <div className="tab-bar" onDragOver={handleBarDragOver} onDrop={handleBarDrop}>
      <div className="tab-bar__tabs">
        {tabs.map((tab, index) => {
          const state = sessions[tab.sessionId]?.state ?? 'idle'
          return (
            <div
              key={tab.id}
              className={`tab-bar__tab ${tab.id === activeId ? 'tab-bar__tab--active' : ''}`}
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e: DragEvent) => handleTabDragOver(e, tab)}
              onDragLeave={() => handleTabDragLeave(tab)}
              onDrop={(e: DragEvent) => handleTabDrop(e, tab, index)}
              onClick={() => setActive(tab.id)}
              onMouseDown={(e) => handleMiddleClick(e, tab)}
            >
              <span className={`tab-bar__dot tab-bar__dot--${state}`} />
              <div className="tab-bar__text">
                <div className="tab-bar__title">{tab.title}</div>
                <div className="tab-bar__subtitle">{tab.subtitle}</div>
              </div>
              <button
                className="tab-bar__close"
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
      <div className="tab-bar__add-wrap">
        <button className="tab-bar__add" onClick={openPopover} aria-label="새 탭">
          +
        </button>
        {popoverOpen && (
          <div className="tab-bar__popover" ref={popoverRef}>
            <button className="tab-bar__popover-item" onClick={() => void createLocalTab()}>
              로컬 쉘
            </button>
            {Object.values(hosts).map((h) => (
              <button key={h.id} className="tab-bar__popover-item" onClick={() => void createSSHTab(h)}>
                {h.name}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
