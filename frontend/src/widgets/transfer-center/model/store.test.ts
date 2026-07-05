import { beforeEach, describe, expect, it } from 'vitest'
import type { TaskInfo } from '../../../shared/api'
import { isActiveTask, sortedTasks, useTransferCenterStore } from './store'

function makeTask(overrides: Partial<TaskInfo> = {}): TaskInfo {
  return {
    id: 't1',
    sessionId: 's1',
    kind: 'upload',
    state: 'running',
    src: '/local/file.bin',
    dst: '/remote/file.bin',
    bytes: 0,
    total: 1000,
    offset: 0,
    ...overrides,
  }
}

beforeEach(() => {
  useTransferCenterStore.setState({ tasks: {}, open: false })
})

describe('upsertTask', () => {
  it('overwrites the same task id rather than accumulating history', () => {
    useTransferCenterStore.getState().upsertTask(makeTask({ bytes: 100 }))
    useTransferCenterStore.getState().upsertTask(makeTask({ bytes: 500 }))
    useTransferCenterStore.getState().upsertTask(makeTask({ bytes: 1000, state: 'done' }))

    const tasks = useTransferCenterStore.getState().tasks
    expect(Object.keys(tasks)).toHaveLength(1)
    expect(tasks['t1'].bytes).toBe(1000)
    expect(tasks['t1'].state).toBe('done')
  })

  it('keeps distinct tasks separate', () => {
    useTransferCenterStore.getState().upsertTask(makeTask({ id: 't1' }))
    useTransferCenterStore.getState().upsertTask(makeTask({ id: 't2' }))
    expect(Object.keys(useTransferCenterStore.getState().tasks)).toHaveLength(2)
  })
})

describe('isActiveTask', () => {
  it('treats queued and running as active', () => {
    expect(isActiveTask(makeTask({ state: 'queued' }))).toBe(true)
    expect(isActiveTask(makeTask({ state: 'running' }))).toBe(true)
  })

  it('treats terminal states as inactive', () => {
    expect(isActiveTask(makeTask({ state: 'done' }))).toBe(false)
    expect(isActiveTask(makeTask({ state: 'failed' }))).toBe(false)
    expect(isActiveTask(makeTask({ state: 'canceled' }))).toBe(false)
  })
})

describe('sortedTasks', () => {
  it('returns every task exactly once regardless of insertion order', () => {
    const tasks = {
      a: makeTask({ id: 'a' }),
      b: makeTask({ id: 'b' }),
      c: makeTask({ id: 'c' }),
    }
    const result = sortedTasks(tasks)
    expect(result.map((t) => t.id).sort()).toEqual(['a', 'b', 'c'])
  })
})

describe('toggleOpen/close', () => {
  it('toggles and closes independently of task state', () => {
    expect(useTransferCenterStore.getState().open).toBe(false)
    useTransferCenterStore.getState().toggleOpen()
    expect(useTransferCenterStore.getState().open).toBe(true)
    useTransferCenterStore.getState().close()
    expect(useTransferCenterStore.getState().open).toBe(false)
  })
})
