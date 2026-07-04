import { describe, expect, it } from 'vitest'
import { shellQuotePath } from './shellQuote'

describe('shellQuotePath', () => {
  it('wraps a plain path in single quotes', () => {
    expect(shellQuotePath('/tmp/file.txt')).toBe("'/tmp/file.txt'")
  })

  it('escapes an embedded single quote', () => {
    expect(shellQuotePath("/tmp/it's a file.txt")).toBe("'/tmp/it'\\''s a file.txt'")
  })

  it('leaves spaces intact inside the quotes', () => {
    expect(shellQuotePath('/tmp/my file.txt')).toBe("'/tmp/my file.txt'")
  })
})
