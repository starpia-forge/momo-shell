import { describe, expect, it } from 'vitest'
import { parseAppSettings } from './settings'

describe('parseAppSettings', () => {
  it('parses a full valid map', () => {
    expect(parseAppSettings({ 'appearance.theme': 'light', 'terminal.fontSize': '18', 'terminal.scrollback': '5000' })).toEqual({
      theme: 'light',
      fontSize: 18,
      scrollback: 5000,
    })
  })

  it('falls back to defaults for missing keys', () => {
    expect(parseAppSettings({})).toEqual({ theme: 'dark', fontSize: 14, scrollback: 10000 })
  })

  it('falls back to defaults for garbage numeric values', () => {
    expect(parseAppSettings({ 'terminal.fontSize': 'large', 'terminal.scrollback': 'lots' })).toEqual({
      theme: 'dark',
      fontSize: 14,
      scrollback: 10000,
    })
  })

  it('falls back to dark for an unknown theme value', () => {
    expect(parseAppSettings({ 'appearance.theme': 'solarized' }).theme).toBe('dark')
  })
})
