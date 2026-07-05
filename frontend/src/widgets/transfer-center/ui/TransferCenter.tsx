import { useEffect } from 'react'
import { cancelTransfer } from '../../../shared/api'
import { registerTransferCenter } from '../lib/register'
import { isActiveTask, sortedTasks, useTransferCenterStore } from '../model/store'
import './TransferCenter.css'

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
    <div className="transfer-center">
      {open && (
        <div className="transfer-center__panel">
          {list.map((task) => {
            const percent = task.total > 0 ? Math.round((task.bytes / task.total) * 100) : null
            return (
              <div key={task.id} className="transfer-center__row">
                <div className="transfer-center__row-main">
                  <span className="transfer-center__file">{task.currentFile || task.src}</span>
                  <span className="transfer-center__state">{STATE_LABEL[task.state] ?? task.state}</span>
                </div>
                {isActiveTask(task) && (
                  <div className="transfer-center__row-progress">
                    <div className="transfer-center__bar">
                      <div className="transfer-center__bar-fill" style={{ width: `${percent ?? 0}%` }} />
                    </div>
                    <span className="transfer-center__bytes">
                      {formatBytes(task.bytes)}
                      {percent !== null && ` / ${percent}%`}
                    </span>
                    <button className="transfer-center__cancel" onClick={() => void cancelTransfer(task.id)}>
                      취소
                    </button>
                  </div>
                )}
                {task.state === 'failed' && task.error && <span className="transfer-center__error">{task.error}</span>}
              </div>
            )
          })}
        </div>
      )}
      <button className="transfer-center__badge" onClick={() => useTransferCenterStore.getState().toggleOpen()}>
        전송 {activeCount > 0 ? activeCount : list.length}
      </button>
    </div>
  )
}
