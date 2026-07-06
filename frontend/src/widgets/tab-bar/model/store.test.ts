import { beforeEach, describe, expect, it } from 'vitest'
import { useTabStore, type Tab } from './store'

function makeTab(overrides: Partial<Tab> = {}): Tab {
  return {
    id: 't1',
    kind: 'local',
    sessionId: 's1',
    title: '로컬 쉘',
    subtitle: '',
    ...overrides,
  }
}

beforeEach(() => {
  useTabStore.setState({ tabs: [], activeId: null, homeActive: true, newTabPopoverOpen: false })
})

describe('homeActive', () => {
  it('starts true', () => {
    expect(useTabStore.getState().homeActive).toBe(true)
  })

  it('showHome sets it true', () => {
    useTabStore.setState({ homeActive: false })
    useTabStore.getState().showHome()
    expect(useTabStore.getState().homeActive).toBe(true)
  })

  it('addTab clears it', () => {
    useTabStore.getState().addTab(makeTab())
    expect(useTabStore.getState().homeActive).toBe(false)
  })

  it('setActive clears it', () => {
    useTabStore.getState().addTab(makeTab())
    useTabStore.getState().showHome()
    useTabStore.getState().setActive('t1')
    expect(useTabStore.getState().homeActive).toBe(false)
  })

  it('activateByIndex clears it only when the index resolves to a tab', () => {
    useTabStore.getState().addTab(makeTab())
    useTabStore.getState().showHome()
    useTabStore.getState().activateByIndex(5)
    expect(useTabStore.getState().homeActive).toBe(true)

    useTabStore.getState().activateByIndex(0)
    expect(useTabStore.getState().homeActive).toBe(false)
  })

  it('removeTab restores home once the last tab is closed, but not while tabs remain', () => {
    useTabStore.getState().addTab(makeTab({ id: 't1' }))
    useTabStore.getState().addTab(makeTab({ id: 't2' }))
    useTabStore.getState().removeTab('t1')
    expect(useTabStore.getState().homeActive).toBe(false)

    useTabStore.getState().removeTab('t2')
    expect(useTabStore.getState().homeActive).toBe(true)
  })
})
