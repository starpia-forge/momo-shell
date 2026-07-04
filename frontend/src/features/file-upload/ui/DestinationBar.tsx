import { useEffect, useRef, useState } from 'react'
import { uploadFiles } from '../../../shared/api'
import { useFileUploadStore } from '../model/store'
import './DestinationBar.css'

const AUTO_PROCEED_MS = 3000

function commit(sessionId: string, paths: string[], cwd: string) {
  useFileUploadStore.getState().setPending(null)
  void uploadFiles(sessionId, paths, cwd)
}

function cancel() {
  useFileUploadStore.getState().setPending(null)
}

/** Confirms (or lets the user redirect) the destination of a pane file
 * drop, auto-proceeding after 3s so a quick drop-and-forget still works. */
export function DestinationBar() {
  const pending = useFileUploadStore((s) => s.pending)
  const [cwd, setCwd] = useState('')
  const [remainingMs, setRemainingMs] = useState(AUTO_PROCEED_MS)
  const cwdRef = useRef(cwd)
  cwdRef.current = cwd

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
        commit(pending.sessionId, pending.paths, cwdRef.current)
      } else {
        setRemainingMs(left)
      }
    }, 100)
    return () => window.clearInterval(interval)
    // Only the drop that opened this bar should (re)start the countdown --
    // editing cwd must not reset it.
  }, [pending])

  if (!pending) return null

  return (
    <div className="destination-bar">
      <span className="destination-bar__label">{pending.paths.length}개 파일을</span>
      <input
        className="destination-bar__path"
        value={cwd}
        autoFocus
        onChange={(e) => setCwd(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') commit(pending.sessionId, pending.paths, cwd)
          if (e.key === 'Escape') cancel()
        }}
      />
      <span className="destination-bar__label">로 업로드 ({Math.ceil(remainingMs / 1000)}s)</span>
      <button className="destination-bar__action" onClick={() => commit(pending.sessionId, pending.paths, cwd)}>
        지금 업로드
      </button>
      <button className="destination-bar__action destination-bar__action--cancel" onClick={cancel}>
        취소
      </button>
    </div>
  )
}
