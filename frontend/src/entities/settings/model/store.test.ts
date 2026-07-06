import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSettingsStore } from './store'
import * as settingsApi from '../../../shared/api/settings'

beforeEach(() => {
  useSettingsStore.setState({ theme: 'dark', fontSize: 14, scrollback: 10000, loaded: false })
  vi.restoreAllMocks()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('load', () => {
  it('merges the RPC result into state and sets loaded', async () => {
    vi.spyOn(settingsApi, 'getAllSettings').mockResolvedValue({ theme: 'light', fontSize: 18, scrollback: 5000 })
    await useSettingsStore.getState().load()
    expect(useSettingsStore.getState()).toMatchObject({ theme: 'light', fontSize: 18, scrollback: 5000, loaded: true })
  })
})

describe('setTheme', () => {
  it('applies optimistically and persists', async () => {
    const setSetting = vi.spyOn(settingsApi, 'setSetting').mockResolvedValue(undefined)
    await useSettingsStore.getState().setTheme('light')
    expect(useSettingsStore.getState().theme).toBe('light')
    expect(setSetting).toHaveBeenCalledWith('appearance.theme', 'light')
  })

  it('rolls back and rethrows on persist failure', async () => {
    vi.spyOn(settingsApi, 'setSetting').mockRejectedValue(new Error('boom'))
    await expect(useSettingsStore.getState().setTheme('light')).rejects.toThrow('boom')
    expect(useSettingsStore.getState().theme).toBe('dark')
  })
})

describe('setScrollback', () => {
  it('clamps to the valid range', async () => {
    vi.spyOn(settingsApi, 'setSetting').mockResolvedValue(undefined)
    await useSettingsStore.getState().setScrollback(500)
    expect(useSettingsStore.getState().scrollback).toBe(1000)

    await useSettingsStore.getState().setScrollback(500000)
    expect(useSettingsStore.getState().scrollback).toBe(100000)
  })

  it('rolls back and rethrows on persist failure', async () => {
    vi.spyOn(settingsApi, 'setSetting').mockRejectedValue(new Error('boom'))
    await expect(useSettingsStore.getState().setScrollback(20000)).rejects.toThrow('boom')
    expect(useSettingsStore.getState().scrollback).toBe(10000)
  })
})

describe('setFontSize', () => {
  it('clamps to the valid range and applies immediately', () => {
    useSettingsStore.getState().setFontSize(4)
    expect(useSettingsStore.getState().fontSize).toBe(8)

    useSettingsStore.getState().setFontSize(100)
    expect(useSettingsStore.getState().fontSize).toBe(32)
  })

  it('debounces persistence: rapid calls persist only the final value once', async () => {
    vi.useFakeTimers()
    const setSetting = vi.spyOn(settingsApi, 'setSetting').mockResolvedValue(undefined)

    useSettingsStore.getState().setFontSize(15)
    useSettingsStore.getState().setFontSize(16)
    useSettingsStore.getState().setFontSize(17)

    await vi.advanceTimersByTimeAsync(500)

    expect(setSetting).toHaveBeenCalledTimes(1)
    expect(setSetting).toHaveBeenCalledWith('terminal.fontSize', '17')
  })
})

describe('resetFontSize', () => {
  it('resets to the default of 14', () => {
    useSettingsStore.getState().setFontSize(30)
    useSettingsStore.getState().resetFontSize()
    expect(useSettingsStore.getState().fontSize).toBe(14)
  })
})
