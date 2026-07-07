import { describe, expect, it } from 'vitest'
import ko from './locales/ko.json'
import en from './locales/en.json'
import zh from './locales/zh.json'
import ja from './locales/ja.json'

const PLURAL_SUFFIX = /_(zero|one|two|few|many|other)$/

// flattenKeys walks a nested translation object into dot-path keys,
// stripping CLDR plural suffixes so languages that only need "_other"
// aren't flagged as missing the "_one"/"_other" split English requires.
function flattenKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object') {
      keys.push(...flattenKeys(value as Record<string, unknown>, path))
    } else {
      keys.push(path.replace(PLURAL_SUFFIX, ''))
    }
  }
  return keys
}

function leafValues(obj: Record<string, unknown>): string[] {
  const values: string[] = []
  for (const value of Object.values(obj)) {
    if (value !== null && typeof value === 'object') {
      values.push(...leafValues(value as Record<string, unknown>))
    } else {
      values.push(String(value))
    }
  }
  return values
}

describe('locale key parity', () => {
  const koKeys = new Set(flattenKeys(ko))

  it.each([
    ['en', en],
    ['zh', zh],
    ['ja', ja],
  ])('%s has the same key set as ko (normalized for plurals)', (_name, locale) => {
    const keys = new Set(flattenKeys(locale))
    const missing = [...koKeys].filter((k) => !keys.has(k))
    const extra = [...keys].filter((k) => !koKeys.has(k))
    expect({ missing, extra }).toEqual({ missing: [], extra: [] })
  })

  it.each([
    ['ko', ko],
    ['en', en],
    ['zh', zh],
    ['ja', ja],
  ])('%s has no empty leaf values', (_name, locale) => {
    for (const value of leafValues(locale)) {
      expect(value.length).toBeGreaterThan(0)
    }
  })
})
