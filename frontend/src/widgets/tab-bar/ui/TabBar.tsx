import { useEffect, useRef, useState, type DragEvent, type MouseEvent as ReactMouseEvent } from 'react'
import { useTabStore, type Tab } from '../model/store'
import { useSessionStore } from '../../../entities/session'
import { useHostStore } from '../../../entities/host'
import { closeTab } from '../lib/closeTab'
import { createLocalTab, createSSHTab } from '../lib/createTab'
import './TabBar.css'

export function TabBar() {
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

  useEffect(() => {
    if (!popoverOpen) return
    const onPointerDown = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) closePopover()
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [popoverOpen, closePopover])

  function handleDrop(index: number) {
    if (dragIndex !== null && dragIndex !== index) reorder(dragIndex, index)
    setDragIndex(null)
  }

  function handleMiddleClick(e: ReactMouseEvent, tab: Tab) {
    if (e.button === 1) {
      e.preventDefault()
      closeTab(tab)
    }
  }

  return (
    <div className="tab-bar">
      <div className="tab-bar__tabs">
        {tabs.map((tab, index) => {
          const state = sessions[tab.sessionId]?.state ?? 'idle'
          return (
            <div
              key={tab.id}
              className={`tab-bar__tab ${tab.id === activeId ? 'tab-bar__tab--active' : ''}`}
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e: DragEvent) => e.preventDefault()}
              onDrop={() => handleDrop(index)}
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
                  closeTab(tab)
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
