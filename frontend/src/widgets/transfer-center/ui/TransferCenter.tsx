import { useEffect } from 'react'
import { cancelTransfer } from '../../../shared/api'
import { registerTransferCenter } from '../lib/register'
import { isActiveTask, sortedTasks, useTransferCenterStore } from '../model/store'

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const kb = n / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  return `${(kb / 1024).toFixed(1)} MB`
}

const STATE_LABEL: Record<string, string> = {
  queued: '대기 중',
  running: '진행 중',
  done: '완료',
  failed: '실패',
  canceled: '취소됨',
}

/** Global transfer queue: a StatusBar-adjacent badge showing the active
 * upload/download count, expanding into a task list with per-task cancel.
 * ZMODEM (rz/sz) transfers have their own overlay on the pane and aren't
 * listed here -- this is the SFTP upload/download queue from M1. */
export function TransferCenter() {
  const tasks = useTransferCenterStore((s) => s.tasks)
  const open = useTransferCenterStore((s) => s.open)

  useEffect(() => registerTransferCenter(), [])

  const list = sortedTasks(tasks)
  const activeCount = list.filter(isActiveTask).length

  if (list.length === 0) return null

  return (
    <div className="fixed right-3 bottom-0 h-6 z-400 flex items-center">
      {open && (
        <div className="absolute right-0 bottom-7 w-80 max-h-80 overflow-y-auto flex flex-col gap-1.5 p-2 rounded-md bg-surface border border-line shadow-float">
          {list.map((task) => {
            const percent = task.total > 0 ? Math.round((task.bytes / task.total) * 100) : null
            return (
              <div key={task.id} className="flex flex-col gap-1 text-[12px] py-1 border-b border-line last:border-b-0">
                <div className="flex justify-between gap-2">
                  <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-fg">
                    {task.currentFile || task.src}
                  </span>
                  <span className="flex-none text-muted">{STATE_LABEL[task.state] ?? task.state}</span>
                </div>
                {isActiveTask(task) && (
                  <div className="flex items-center gap-1.5">
                    <div className="flex-1 h-1 rounded-full bg-line overflow-hidden">
                      <div className="h-full bg-accent" style={{ width: `${percent ?? 0}%` }} />
                    </div>
                    <span className="flex-none text-muted text-[11px]">
                      {formatBytes(task.bytes)}
                      {percent !== null && ` / ${percent}%`}
                    </span>
                    <button
                      className="flex-none border border-line bg-canvas text-fg rounded px-1.5 py-0.5 text-[11px] cursor-pointer"
                      onClick={() => void cancelTransfer(task.id)}
                    >
                      취소
                    </button>
                  </div>
                )}
                {task.state === 'failed' && task.error && <span className="text-danger text-[11px]">{task.error}</span>}
              </div>
            )
          })}
        </div>
      )}
      <button
        className="border-none bg-transparent text-muted text-[12px] cursor-pointer px-1 hover:text-fg"
        onClick={() => useTransferCenterStore.getState().toggleOpen()}
      >
        전송 {activeCount > 0 ? activeCount : list.length}
      </button>
    </div>
  )
}
