import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { cancelTransfer } from '../../../shared/api'
import { formatBytes } from '../../../shared/lib/formatBytes'
import { ProgressBar } from '../../../shared/ui'
import { registerTransferCenter } from '../lib/register'
import { isActiveTask, sortedTasks, useTransferCenterStore } from '../model/store'

/** Popover anchored under the tab bar's TransferBadge, listing every
 * upload/download task with per-task progress and cancel. ZMODEM (rz/sz)
 * transfers have their own overlay on the pane and aren't listed here --
 * this is the SFTP upload/download queue from M1. */
export function TransferCenter() {
  const { t } = useTranslation()
  const STATE_LABEL: Record<string, string> = {
    queued: t('transfer.stateQueued'),
    running: t('transfer.stateRunning'),
    done: t('transfer.stateDone'),
    failed: t('transfer.stateFailed'),
    canceled: t('transfer.stateCanceled'),
  }
  const tasks = useTransferCenterStore((s) => s.tasks)
  const open = useTransferCenterStore((s) => s.open)

  useEffect(() => registerTransferCenter(), [])

  const list = sortedTasks(tasks)

  if (!open || list.length === 0) return null

  return (
    <div className="fixed right-6 bottom-4 z-400 w-100 max-h-90 overflow-y-auto flex flex-col gap-1 p-2.5 rounded-xl bg-surface2 border border-line shadow-modal">
      {list.map((task) => {
        const percent = task.total > 0 ? Math.round((task.bytes / task.total) * 100) : null
        const direction = task.kind === 'upload' ? 'up' : 'down'
        return (
          <div key={task.id} className="flex flex-col gap-1.75 px-3 py-2.5 rounded-md bg-inputbg">
            <div className="flex items-center gap-2">
              <span className={direction === 'up' ? 'text-[11px] font-mono font-bold text-accent-text' : 'text-[11px] font-mono font-bold text-blue'}>
                {direction === 'up' ? '↑' : '↓'}
              </span>
              <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px] font-medium">
                {task.currentFile || task.src}
              </span>
              {isActiveTask(task) && (
                <button
                  className="flex-none border-none bg-transparent text-fg3 cursor-pointer text-[12px] hover:text-fg"
                  onClick={() => void cancelTransfer(task.id)}
                  aria-label={t('common.cancel')}
                >
                  ✕
                </button>
              )}
            </div>
            {isActiveTask(task) && <ProgressBar value={percent ?? 0} direction={direction} className="h-1.25" />}
            <div className="flex font-mono text-[10.5px] text-fg3">
              <span>
                {isActiveTask(task)
                  ? `${formatBytes(task.bytes)} / ${formatBytes(task.total)}${percent !== null ? ` · ${percent}%` : ''}`
                  : STATE_LABEL[task.state]}
              </span>
              <div className="flex-1" />
              <span className="overflow-hidden text-ellipsis whitespace-nowrap">{task.dst}</span>
            </div>
            {task.state === 'failed' && task.error && <span className="text-red text-[11px]">{task.error}</span>}
          </div>
        )
      })}
    </div>
  )
}
