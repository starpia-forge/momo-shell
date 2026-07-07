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
import { SearchInput, SegmentedControl, Toast } from '../../../shared/ui'
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
    <div className="flex flex-col flex-1 min-h-0 text-[13px] p-3.5 gap-3">
      <span className="text-[13.5px] font-bold">명령어 히스토리</span>
      <SegmentedControl
        value={scopeMode}
        onChange={(v) => setScopeMode(v as HistoryScopeMode)}
        options={[
          { value: 'all', label: '전체' },
          { value: 'current', label: '현재 호스트' },
        ]}
      />
      <SearchInput
        containerClassName="h-8"
        placeholder="명령어 필터…"
        value={filter}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setFilter(e.target.value)}
      />
      <div className="flex-1 -mx-3.5 overflow-y-auto">
        {visible.map((entry) => (
          <div
            key={entry.id}
            className="group flex items-center gap-2 mx-2.25 px-2.5 py-2.25 rounded-md cursor-pointer hover:bg-surface2"
            onClick={() => handleCopy(entry.command)}
          >
            <span
              className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[12px]"
              title={entry.command}
            >
              {entry.command}
            </span>
            <span className="flex-none text-[10.5px] text-fg3 group-hover:hidden">{formatRelativeTime(entry.executedAt)}</span>
            <div className="hidden flex-none items-center gap-1.5 group-hover:flex">
              <span className="text-[11px] text-accent-text">복사</span>
              <button
                className="border-none bg-transparent text-fg2 cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-fg"
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
                className="border-none bg-transparent text-fg2 cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-red"
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
      <div className="text-[11px] text-fg3">클릭하면 클립보드에 복사됩니다</div>
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
