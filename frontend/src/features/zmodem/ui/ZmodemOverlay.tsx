import { useEffect, useState } from 'react'
import {
  browseForUploadFiles,
  cancelZmodem,
  startZmodemSend,
  subscribe,
  topics,
  type TransferProgressPayload,
  type TransferZmodemPayload,
} from '../../../shared/api'
import { Spinner } from '../../../shared/ui'
import './ZmodemOverlay.css'

interface ZmodemOverlayProps {
  sessionId: string
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const kb = n / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  return `${(kb / 1024).toFixed(1)} MB`
}

/** Renders over a terminal pane while ZMODEM (rz/sz) owns its shell channel
 * -- the backend already blocks keystrokes during this; this is the visual
 * half (detection prompt, progress, cancel). */
export function ZmodemOverlay({ sessionId }: ZmodemOverlayProps) {
  const [state, setState] = useState<TransferZmodemPayload | null>(null)
  const [progress, setProgress] = useState<TransferProgressPayload | null>(null)

  useEffect(
    () =>
      subscribe<TransferZmodemPayload>(topics.transferZmodem(sessionId), (payload) => {
        setState(payload)
        if (payload.phase !== 'active') setProgress(null)
        if (payload.phase === 'done' || payload.phase === 'failed' || payload.phase === 'canceled') {
          setTimeout(() => setState((s) => (s === payload ? null : s)), 2000)
        }
      }),
    [sessionId]
  )

  useEffect(() => {
    if (!state?.taskId || state.phase !== 'active') return
    return subscribe<TransferProgressPayload>(topics.transferProgress(state.taskId), setProgress)
  }, [state?.taskId, state?.phase])

  if (!state) return null

  function handlePickFiles() {
    void browseForUploadFiles().then((paths) => {
      if (paths.length === 0) return
      void startZmodemSend(sessionId, paths)
    })
  }

  function handleCancel() {
    void cancelZmodem(sessionId)
  }

  const percent = progress && progress.total > 0 ? Math.round((progress.bytes / progress.total) * 100) : null

  return (
    <div className="zmodem-overlay">
      {state.phase === 'detected' && state.direction === 'upload' && (
        <>
          <span className="zmodem-overlay__text">원격에서 rz 대기 중 -- 보낼 파일을 선택하세요</span>
          <button className="zmodem-overlay__action" onClick={handlePickFiles}>
            파일 선택
          </button>
          <button className="zmodem-overlay__action zmodem-overlay__action--cancel" onClick={handleCancel}>
            취소
          </button>
        </>
      )}
      {state.phase === 'active' && (
        <>
          <Spinner size={12} />
          <span className="zmodem-overlay__text">
            {state.direction === 'upload' ? '전송 중' : '수신 중'}
            {progress && `: ${progress.file} (${formatBytes(progress.bytes)}${percent !== null ? ` / ${percent}%` : ''})`}
          </span>
          <button className="zmodem-overlay__action zmodem-overlay__action--cancel" onClick={handleCancel}>
            취소
          </button>
        </>
      )}
      {state.phase === 'done' && <span className="zmodem-overlay__text">전송 완료</span>}
      {state.phase === 'failed' && <span className="zmodem-overlay__text zmodem-overlay__text--error">전송 실패</span>}
      {state.phase === 'canceled' && <span className="zmodem-overlay__text">전송 취소됨</span>}
    </div>
  )
}
