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
    <div className="absolute left-2 right-2 bottom-2 z-10 flex items-center gap-2 px-2.5 py-1.5 rounded-md bg-surface border border-line shadow-[0_2px_8px_rgba(0,0,0,0.3)] text-fg text-[12px]">
      {state.phase === 'detected' && state.direction === 'upload' && (
        <>
          <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
            원격에서 rz 대기 중 -- 보낼 파일을 선택하세요
          </span>
          <button
            className="flex-none border border-line bg-accent text-on-accent rounded px-2 py-[3px] text-[11px] cursor-pointer"
            onClick={handlePickFiles}
          >
            파일 선택
          </button>
          <button
            className="flex-none border border-line bg-canvas text-fg rounded px-2 py-[3px] text-[11px] cursor-pointer"
            onClick={handleCancel}
          >
            취소
          </button>
        </>
      )}
      {state.phase === 'active' && (
        <>
          <Spinner size={12} />
          <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
            {state.direction === 'upload' ? '전송 중' : '수신 중'}
            {progress && `: ${progress.file} (${formatBytes(progress.bytes)}${percent !== null ? ` / ${percent}%` : ''})`}
          </span>
          <button
            className="flex-none border border-line bg-canvas text-fg rounded px-2 py-[3px] text-[11px] cursor-pointer"
            onClick={handleCancel}
          >
            취소
          </button>
        </>
      )}
      {state.phase === 'done' && <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">전송 완료</span>}
      {state.phase === 'failed' && (
        <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-red">전송 실패</span>
      )}
      {state.phase === 'canceled' && <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">전송 취소됨</span>}
    </div>
  )
}
