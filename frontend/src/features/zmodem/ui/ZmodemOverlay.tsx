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
import { formatBytes } from '../../../shared/lib/formatBytes'
import { Button, ProgressBar } from '../../../shared/ui'

interface ZmodemOverlayProps {
  sessionId: string
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
  const direction = state.direction === 'upload' ? 'up' : 'down'

  return (
    <div className="absolute left-2.5 right-2.5 bottom-2.5 z-10 max-w-130 flex flex-col gap-2 px-4 py-3 rounded-md bg-accent/8 border border-accent/35 font-sans text-fg text-[12px]">
      {state.phase === 'detected' && state.direction === 'upload' && (
        <div className="flex items-center gap-2.5">
          <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
            원격에서 rz 대기 중 — 보낼 파일을 선택하세요
          </span>
          <Button variant="primary" size="sm" onClick={handlePickFiles}>
            파일 선택
          </Button>
          <Button size="sm" onClick={handleCancel}>
            취소
          </Button>
        </div>
      )}
      {state.phase === 'active' && (
        <>
          <div className="flex items-center gap-2.5">
            <span className="font-bold text-accent-text">{state.direction === 'upload' ? '전송 중' : '수신 중'}</span>
            {progress && <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-fg2">{progress.file}</span>}
            <Button size="sm" onClick={handleCancel}>
              취소
            </Button>
          </div>
          <ProgressBar value={percent ?? 0} direction={direction} />
          {progress && (
            <div className="flex font-mono text-[11px] text-fg3">
              <span>
                {percent !== null ? `${percent}% · ` : ''}
                {formatBytes(progress.bytes)}
              </span>
            </div>
          )}
        </>
      )}
      {state.phase === 'done' && <span>전송 완료</span>}
      {state.phase === 'failed' && <span className="text-red">전송 실패</span>}
      {state.phase === 'canceled' && <span>전송 취소됨</span>}
    </div>
  )
}
