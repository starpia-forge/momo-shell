import { describe, expect, it } from 'vitest'
import { resolveLanguage } from './resolve'

describe('resolveLanguage', () => {
  it('resolves system setting via navigator language prefix', () => {
    expect(resolveLanguage('system', 'ko-KR')).toBe('ko')
    expect(resolveLanguage('system', 'zh-TW')).toBe('zh')
    expect(resolveLanguage('system', 'ja')).toBe('ja')
  })

  it('falls back to en for an unsupported system locale', () => {
    expect(resolveLanguage('system', 'fr-FR')).toBe('en')
  })

  it('is case-insensitive', () => {
    expect(resolveLanguage('system', 'KO-kr')).toBe('ko')
  })

  it('prefers an explicit setting over navigator language', () => {
    expect(resolveLanguage('ko', 'en-US')).toBe('ko')
  })

  it('falls back to system resolution for an unknown explicit value', () => {
    expect(resolveLanguage('fr', 'ja-JP')).toBe('ja')
  })
})
