import { useEffect, useRef } from 'react'
import { subscribe, topics, type FileDropPayload } from '../../../shared/api/events'
import { ConflictDialog } from '../../../shared/ui'
import { useTabStore } from '../../../widgets/tab-bar'
import { localOps } from '../model/paneOps'
import { initTransferWatcher, useSftpStore } from '../model/store'
import { ConnectGate } from './ConnectGate'
import { FilePane } from './FilePane'

const HOVER_FRESHNESS_MS = 1000

/** R3: two panes -- Local Directory (left) and Remote Directory (right).
 * Occupies the workspace's full center row, like the settings screen,
 * since the dual-pane layout needs the width the host sidebar/right dock
 * would otherwise take. */
export function SftpPage() {
  const sessionId = useSftpStore((s) => s.sessionId)
  const remoteOps = useSftpStore((s) => s.remoteOps)
  const initLocal = useSftpStore((s) => s.initLocal)
  const disconnect = useSftpStore((s) => s.disconnect)
  const dropIncoming = useSftpStore((s) => s.dropIncoming)
  const pendingConflict = useSftpStore((s) => s.pendingConflict)
  const clearPendingConflict = useSftpStore((s) => s.clearPendingConflict)

  const hoverRef = useRef<{ side: 'local' | 'remote'; at: number } | null>(null)

  useEffect(() => {
    void initLocal()
    initTransferWatcher()
  }, [initLocal])

  // Wails' native OS file drop reports only viewport coordinates (no DOM
  // target), so this page tracks which pane a drag was last hovering
  // (via FilePane's onFileDragHover) and falls back to a point lookup for
  // drops that land without a preceding dragover on this page.
  useEffect(() => {
    return subscribe<FileDropPayload>(topics.osFileDrop(), (payload) => {
      if (useTabStore.getState().screen !== 'sftp') return

      let side: 'local' | 'remote' | null = null
      if (hoverRef.current && Date.now() - hoverRef.current.at < HOVER_FRESHNESS_MS) {
        side = hoverRef.current.side
      } else {
        const el = document.elementFromPoint(payload.x, payload.y)
        const withPane = el?.closest<HTMLElement>('[data-sftp-pane]')
        side = (withPane?.dataset.sftpPane as 'local' | 'remote' | undefined) ?? null
      }
      if (!side) return
      void dropIncoming(side, payload.paths)
    })
  }, [dropIncoming])

  return (
    <div className="sftp-page flex-1 min-w-0 h-full flex gap-3 p-3 bg-canvas text-fg">
      <FilePane side="local" ops={localOps} onFileDragHover={() => (hoverRef.current = { side: 'local', at: Date.now() })} />
      {sessionId && remoteOps ? (
        <FilePane side="remote" ops={remoteOps} onDisconnect={disconnect} onFileDragHover={() => (hoverRef.current = { side: 'remote', at: Date.now() })} />
      ) : (
        <ConnectGate />
      )}
      <ConflictDialog
        open={pendingConflict !== null}
        conflicts={pendingConflict?.names ?? []}
        onChoice={(policy) => pendingConflict?.resume(policy)}
        onClose={clearPendingConflict}
      />
    </div>
  )
}
