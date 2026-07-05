import { create } from 'zustand'
import type { TaskInfo } from '../../../shared/api'

interface TransferCenterState {
  tasks: Record<string, TaskInfo>
  open: boolean
  toggleOpen: () => void
  close: () => void
  upsertTask: (task: TaskInfo) => void
}

export const useTransferCenterStore = create<TransferCenterState>((set) => ({
  tasks: {},
  open: false,
  toggleOpen: () => set((s) => ({ open: !s.open })),
  close: () => set({ open: false }),
  upsertTask: (task) => set((s) => ({ tasks: { ...s.tasks, [task.id]: task } })),
}))

export function isActiveTask(task: TaskInfo): boolean {
  return task.state === 'queued' || task.state === 'running'
}

export function sortedTasks(tasks: Record<string, TaskInfo>): TaskInfo[] {
  return Object.values(tasks).sort((a, b) => (a.id < b.id ? 1 : -1))
}
