import { useEffect, useState, type ChangeEvent } from 'react'
import {
  deleteHistoryEntry,
  listHistory,
  setClipboardText,
  subscribe,
  topics,
  writeSession,
  type HistoryAppendedPayload,
  type HistoryEntry,
} from '../../../shared/api'
import { Toast } from '../../../shared/ui'
import { formatRelativeTime } from '../lib/formatRelativeTime'
import { useHistoryPanelStore, type HistoryScopeMode } from '../model/store'

interface HistoryPanelProps {
  /** The currently-focused pane's session, or null if there is none. */
  focusedSessionId: string | null
  /** '' for local, otherwise the active tab's host ID -- what "현재 호스트" resolves to. */
  currentHostId: string
}

function matchesScope(entry: HistoryEntry, effectiveHostId: string): boolean {
  if (effectiveHostId === '') return true
  if (effectiveHostId === 'local') return !entry.hostId
  return entry.hostId === effectiveHostId
}

export function HistoryPanel({ focusedSessionId, currentHostId }: HistoryPanelProps) {
  const entries = useHistoryPanelStore((s) => s.entries)
  const filter = useHistoryPanelStore((s) => s.filter)
  const scopeMode = useHistoryPanelStore((s) => s.scopeMode)
  const setFilter = useHistoryPanelStore((s) => s.setFilter)
  const setScopeMode = useHistoryPanelStore((s) => s.setScopeMode)
  const [toast, setToast] = useState<string | null>(null)

  const effectiveHostId = scopeMode === 'current' ? currentHostId : ''

  // Only mounted while RightDock has this tab active, so no visibility gate
  // is needed here -- mounting itself is the signal to (re)load.
  useEffect(() => {
    let cancelled = false
    void listHistory({ hostId: effectiveHostId }).then((result) => {
      if (!cancelled) useHistoryPanelStore.getState().setEntries(result)
    })
    return () => {
      cancelled = true
    }
  }, [effectiveHostId])

  useEffect(
    () =>
      subscribe<HistoryAppendedPayload>(topics.historyAppended(), (payload) => {
        const entry: HistoryEntry = {
          id: payload.id,
          hostId: payload.hostId,
          command: payload.command,
          executedAt: payload.executedAt,
        }
        if (matchesScope(entry, effectiveHostId)) useHistoryPanelStore.getState().prepend(entry)
      }),
    [effectiveHostId]
  )

  const visible = entries.filter((e) => e.command.toLowerCase().includes(filter.toLowerCase()))

  function handleCopy(command: string) {
    void setClipboardText(command)
    setToast('복사됨')
  }

  function handleSend(command: string) {
    if (!focusedSessionId) return
    void writeSession(focusedSessionId, new TextEncoder().encode(command))
  }

  function handleDelete(id: number) {
    void deleteHistoryEntry(id)
    useHistoryPanelStore.getState().removeEntry(id)
  }

  return (
    <div className="flex flex-col flex-1 min-h-0 text-[13px]">
      <div className="flex gap-1.5 mt-2 px-2 pb-2">
        <input
          className="flex-1 min-w-0 px-2 py-1.5 rounded border border-line bg-canvas text-fg text-[12px]"
          placeholder="필터"
          value={filter}
          onChange={(e: ChangeEvent<HTMLInputElement>) => setFilter(e.target.value)}
        />
        <select
          className="flex-none px-1.5 py-1.5 rounded border border-line bg-canvas text-fg text-[11px]"
          value={scopeMode}
          onChange={(e: ChangeEvent<HTMLSelectElement>) => setScopeMode(e.target.value as HistoryScopeMode)}
        >
          <option value="all">전체</option>
          <option value="current">현재 호스트</option>
        </select>
      </div>
      <div className="flex-1 overflow-y-auto">
        {visible.map((entry) => (
          <div
            key={entry.id}
            className="group flex items-center gap-2 px-2 py-1.5 cursor-pointer hover:bg-canvas"
            onClick={() => handleCopy(entry.command)}
          >
            <span className="flex-none text-[10px] text-muted w-10">{formatRelativeTime(entry.executedAt)}</span>
            <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12px]" title={entry.command}>
              {entry.command}
            </span>
            <div className="invisible flex-none flex gap-0.5 group-hover:visible">
              <button
                className="border-none bg-transparent text-muted cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-fg"
                onClick={(e) => {
                  e.stopPropagation()
                  handleSend(entry.command)
                }}
                aria-label="포커스된 창에 입력"
                title="포커스된 창에 입력"
              >
                ↵
              </button>
              <button
                className="border-none bg-transparent text-muted cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-danger"
                onClick={(e) => {
                  e.stopPropagation()
                  handleDelete(entry.id)
                }}
                aria-label="삭제"
                title="삭제"
              >
                ×
              </button>
            </div>
          </div>
        ))}
      </div>
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
