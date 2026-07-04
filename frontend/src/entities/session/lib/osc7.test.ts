import { describe, expect, it } from 'vitest'
import { parseOsc7Path } from './osc7'

describe('parseOsc7Path', () => {
  it('extracts the path from a file:// URI with a hostname', () => {
    expect(parseOsc7Path('file://myhost/home/user/project')).toBe('/home/user/project')
  })

  it('extracts the path from a file:// URI with no hostname', () => {
    expect(parseOsc7Path('file:///home/user')).toBe('/home/user')
  })

  it('URI-decodes percent-escaped characters', () => {
    expect(parseOsc7Path('file://myhost/home/user/my%20project')).toBe('/home/user/my project')
  })

  it('returns null for a non-OSC7-shaped payload', () => {
    expect(parseOsc7Path('not-a-uri')).toBeNull()
  })
})
