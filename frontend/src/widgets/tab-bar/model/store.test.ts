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
  useTabStore.setState({ tabs: [], activeId: null, screen: 'home', newTabPopoverOpen: false })
})

describe('screen', () => {
  it('starts on home', () => {
    expect(useTabStore.getState().screen).toBe('home')
  })

  it('showHome switches to home', () => {
    useTabStore.setState({ screen: 'workspace' })
    useTabStore.getState().showHome()
    expect(useTabStore.getState().screen).toBe('home')
  })

  it('showSettings switches to settings', () => {
    useTabStore.getState().showSettings()
    expect(useTabStore.getState().screen).toBe('settings')
  })

  it('showSftp switches to sftp', () => {
    useTabStore.getState().showSftp()
    expect(useTabStore.getState().screen).toBe('sftp')
  })

  it('addTab switches to workspace', () => {
    useTabStore.getState().addTab(makeTab())
    expect(useTabStore.getState().screen).toBe('workspace')
  })

  it('setActive switches to workspace', () => {
    useTabStore.getState().addTab(makeTab())
    useTabStore.getState().showSettings()
    useTabStore.getState().setActive('t1')
    expect(useTabStore.getState().screen).toBe('workspace')
  })

  it('activateByIndex switches to workspace only when the index resolves to a tab', () => {
    useTabStore.getState().addTab(makeTab())
    useTabStore.getState().showSettings()
    useTabStore.getState().activateByIndex(5)
    expect(useTabStore.getState().screen).toBe('settings')

    useTabStore.getState().activateByIndex(0)
    expect(useTabStore.getState().screen).toBe('workspace')
  })

  it('removeTab restores home once the last tab is closed, but not while tabs remain', () => {
    useTabStore.getState().addTab(makeTab({ id: 't1' }))
    useTabStore.getState().addTab(makeTab({ id: 't2' }))
    useTabStore.getState().removeTab('t1')
    expect(useTabStore.getState().screen).toBe('workspace')

    useTabStore.getState().removeTab('t2')
    expect(useTabStore.getState().screen).toBe('home')
  })

  it('removeTab leaves a non-workspace screen (e.g. settings) untouched while tabs remain', () => {
    useTabStore.getState().addTab(makeTab({ id: 't1' }))
    useTabStore.getState().addTab(makeTab({ id: 't2' }))
    useTabStore.getState().showSettings()
    useTabStore.getState().removeTab('t1')
    expect(useTabStore.getState().screen).toBe('settings')
  })

  it('addTab while on sftp switches to workspace', () => {
    useTabStore.getState().showSftp()
    useTabStore.getState().addTab(makeTab())
    expect(useTabStore.getState().screen).toBe('workspace')
  })

  it('removeTab of the last tab falls back to home even while on sftp', () => {
    useTabStore.getState().addTab(makeTab())
    useTabStore.getState().showSftp()
    useTabStore.getState().removeTab('t1')
    expect(useTabStore.getState().screen).toBe('home')
  })
})
