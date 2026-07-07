import { describe, expect, it } from 'vitest'
import { parseAppSettings } from './settings'

describe('parseAppSettings', () => {
  it('parses a full valid map', () => {
    expect(
      parseAppSettings({
        'appearance.theme': 'light',
        'appearance.accent': 'blue',
        'terminal.fontSize': '18',
        'terminal.scrollback': '5000',
        'general.language': 'ja',
      })
    ).toEqual({
      theme: 'light',
      accent: 'blue',
      fontSize: 18,
      scrollback: 5000,
      language: 'ja',
    })
  })

  it('falls back to defaults for missing keys', () => {
    expect(parseAppSettings({})).toEqual({
      theme: 'dark',
      accent: 'pink',
      fontSize: 14,
      scrollback: 10000,
      language: 'system',
    })
  })

  it('falls back to defaults for garbage numeric values', () => {
    expect(parseAppSettings({ 'terminal.fontSize': 'large', 'terminal.scrollback': 'lots' })).toEqual({
      theme: 'dark',
      accent: 'pink',
      fontSize: 14,
      scrollback: 10000,
      language: 'system',
    })
  })

  it('falls back to dark for an unknown theme value', () => {
    expect(parseAppSettings({ 'appearance.theme': 'solarized' }).theme).toBe('dark')
  })

  it('falls back to pink for an unknown accent value', () => {
    expect(parseAppSettings({ 'appearance.accent': 'chartreuse' }).accent).toBe('pink')
  })

  it('falls back to system for an unknown language value', () => {
    expect(parseAppSettings({ 'general.language': 'fr' }).language).toBe('system')
  })
})
