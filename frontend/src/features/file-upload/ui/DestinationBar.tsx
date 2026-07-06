import { useEffect, useRef, useState } from 'react'
import { detectUploadConflicts, uploadFiles, type ConflictPolicy } from '../../../shared/api'
import { ConflictDialog } from '../../../shared/ui'
import { useFileUploadStore } from '../model/store'

const AUTO_PROCEED_MS = 3000

function cancel() {
  useFileUploadStore.getState().setPending(null)
}

/** Confirms (or lets the user redirect) the destination of a pane file
 * drop, auto-proceeding after 3s so a quick drop-and-forget still works. */
export function DestinationBar() {
  const pending = useFileUploadStore((s) => s.pending)
  const [cwd, setCwd] = useState('')
  const [remainingMs, setRemainingMs] = useState(AUTO_PROCEED_MS)
  const [conflict, setConflict] = useState<{ sessionId: string; paths: string[]; dir: string; names: string[] } | null>(null)
  const cwdRef = useRef(cwd)
  cwdRef.current = cwd

  async function commit(sessionId: string, paths: string[], destCwd: string) {
    useFileUploadStore.getState().setPending(null)
    try {
      const names = await detectUploadConflicts(sessionId, paths, destCwd)
      if (names.length > 0) {
        setConflict({ sessionId, paths, dir: destCwd, names })
        return
      }
      await uploadFiles(sessionId, paths, destCwd)
    } catch {
      // Upload/detection errors surface via the transfer center's own task
      // state; nothing else to show from here.
    }
  }

  function handleConflictChoice(policy: ConflictPolicy) {
    if (!conflict) return
    void uploadFiles(conflict.sessionId, conflict.paths, conflict.dir, policy)
  }

  useEffect(() => {
    if (!pending) return
    setCwd(pending.cwd)
    cwdRef.current = pending.cwd
    setRemainingMs(AUTO_PROCEED_MS)

    const start = Date.now()
    const interval = window.setInterval(() => {
      const left = AUTO_PROCEED_MS - (Date.now() - start)
      if (left <= 0) {
        window.clearInterval(interval)
        void commit(pending.sessionId, pending.paths, cwdRef.current)
      } else {
        setRemainingMs(left)
      }
    }, 100)
    return () => window.clearInterval(interval)
    // Only the drop that opened this bar should (re)start the countdown --
    // editing cwd must not reset it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pending])

  return (
    <>
      {pending && (
        <div className="fixed left-1/2 bottom-6 -translate-x-1/2 z-100 flex items-center gap-2 px-3 py-2 rounded-lg bg-surface border border-line shadow-[0_4px_16px_rgba(0,0,0,0.35)] text-fg text-[12px]">
          <span className="whitespace-nowrap text-muted">{pending.paths.length}개 파일을</span>
          <input
            className="min-w-55 px-2 py-1 rounded border border-line bg-canvas text-fg text-[12px]"
            value={cwd}
            autoFocus
            onChange={(e) => setCwd(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void commit(pending.sessionId, pending.paths, cwd)
              if (e.key === 'Escape') cancel()
            }}
          />
          <span className="whitespace-nowrap text-muted">로 업로드 ({Math.ceil(remainingMs / 1000)}s)</span>
          <button
            className="border border-line bg-accent text-on-accent rounded px-2.5 py-1 text-[12px] cursor-pointer"
            onClick={() => void commit(pending.sessionId, pending.paths, cwd)}
          >
            지금 업로드
          </button>
          <button
            className="border border-line bg-canvas text-fg rounded px-2.5 py-1 text-[12px] cursor-pointer"
            onClick={cancel}
          >
            취소
          </button>
        </div>
      )}
      <ConflictDialog
        open={conflict !== null}
        conflicts={conflict?.names ?? []}
        onChoice={handleConflictChoice}
        onClose={() => setConflict(null)}
      />
    </>
  )
}
