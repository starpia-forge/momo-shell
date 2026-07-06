import { describe, expect, it } from 'vitest'
import { DRIVES_VIEW, isWindowsPath, joinLocalPath, localBaseName, parentLocalPath } from './paths'

describe('isWindowsPath', () => {
  it('detects drive-letter paths', () => {
    expect(isWindowsPath('C:\\Users\\me')).toBe(true)
    expect(isWindowsPath('C:/Users/me')).toBe(true)
  })

  it('rejects unix paths and the drives-view sentinel', () => {
    expect(isWindowsPath('/home/me')).toBe(false)
    expect(isWindowsPath(DRIVES_VIEW)).toBe(false)
  })
})

describe('joinLocalPath', () => {
  it('joins windows paths with backslash', () => {
    expect(joinLocalPath('C:\\Users\\me', 'docs')).toBe('C:\\Users\\me\\docs')
    expect(joinLocalPath('C:\\', 'docs')).toBe('C:\\docs')
  })

  it('joins unix paths with slash', () => {
    expect(joinLocalPath('/home/me', 'docs')).toBe('/home/me/docs')
    expect(joinLocalPath('/', 'docs')).toBe('/docs')
  })

  it('joins a drive letter directly from the drives view', () => {
    expect(joinLocalPath(DRIVES_VIEW, 'C:\\')).toBe('C:\\')
  })
})

describe('parentLocalPath', () => {
  it('walks up a windows path to the drive root, then to the drives view', () => {
    expect(parentLocalPath('C:\\a\\b')).toBe('C:\\a')
    expect(parentLocalPath('C:\\a')).toBe('C:\\')
    expect(parentLocalPath('C:\\')).toBe(DRIVES_VIEW)
  })

  it('walks up a unix path to root, then returns null', () => {
    expect(parentLocalPath('/home/u')).toBe('/home')
    expect(parentLocalPath('/home')).toBe('/')
    expect(parentLocalPath('/')).toBeNull()
  })

  it('returns null for the drives view itself', () => {
    expect(parentLocalPath(DRIVES_VIEW)).toBeNull()
  })
})

describe('localBaseName', () => {
  it('returns the last segment for nested paths', () => {
    expect(localBaseName('C:\\a\\b')).toBe('b')
    expect(localBaseName('/home/u')).toBe('u')
  })

  it('returns the root itself for drive roots and unix root', () => {
    expect(localBaseName('C:\\')).toBe('C:\\')
    expect(localBaseName('/')).toBe('/')
  })
})
