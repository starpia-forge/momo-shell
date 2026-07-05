import { subscribe, topics, type TaskInfo, type TransferProgressPayload } from '../../../shared/api'
import { useToastStore } from '../../../shared/ui'
import { useTransferCenterStore } from '../model/store'

const progressUnsubs = new Map<string, () => void>()
const notifiedTerminal = new Set<string>()

function subscribeProgress(taskId: string) {
  if (progressUnsubs.has(taskId)) return
  const unsub = subscribe<TransferProgressPayload>(topics.transferProgress(taskId), (p) => {
    useTransferCenterStore.setState((s) => {
      const existing = s.tasks[taskId]
      if (!existing) return s
      return { tasks: { ...s.tasks, [taskId]: { ...existing, bytes: p.bytes, total: p.total, currentFile: p.file } } }
    })
  })
  progressUnsubs.set(taskId, unsub)
}

function unsubscribeProgress(taskId: string) {
  progressUnsubs.get(taskId)?.()
  progressUnsubs.delete(taskId)
}

function notifyTerminal(task: TaskInfo) {
  if (notifiedTerminal.has(task.id)) return
  notifiedTerminal.add(task.id)
  const label = task.kind === 'upload' ? '업로드' : '다운로드'
  const fileName = task.currentFile || task.src
  if (task.state === 'done') {
    useToastStore.getState().push(`${label} 완료: ${fileName}`)
  } else if (task.state === 'failed') {
    useToastStore.getState().push(`${label} 실패: ${fileName}${task.error ? ` (${task.error})` : ''}`)
  } else if (task.state === 'canceled') {
    useToastStore.getState().push(`${label} 취소됨: ${fileName}`)
  }
}

/** Wires transfer:task / transfer:progress into the transfer-center store
 * and the app-wide toast queue. Call once at app startup. */
export function registerTransferCenter(): () => void {
  const unsubTask = subscribe<TaskInfo>(topics.transferTask(), (task) => {
    useTransferCenterStore.getState().upsertTask(task)
    if (task.state === 'queued' || task.state === 'running') {
      subscribeProgress(task.id)
    } else {
      unsubscribeProgress(task.id)
      notifyTerminal(task)
    }
  })

  return () => {
    unsubTask()
    for (const unsub of progressUnsubs.values()) unsub()
    progressUnsubs.clear()
  }
}
