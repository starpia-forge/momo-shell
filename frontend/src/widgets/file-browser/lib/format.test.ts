import { describe, expect, it } from 'vitest'
import { formatSize, joinRemotePath, parentRemotePath } from './format'

describe('formatSize', () => {
  it('renders bytes under 1024 as-is', () => {
    expect(formatSize(512)).toBe('512 B')
  })

  it('renders kilobytes with one decimal under 10', () => {
    expect(formatSize(1536)).toBe('1.5 KB')
  })

  it('renders larger units with no decimal at 10+', () => {
    expect(formatSize(15 * 1024)).toBe('15 KB')
  })

  it('scales up through megabytes', () => {
    expect(formatSize(5 * 1024 * 1024)).toBe('5.0 MB')
  })
})

describe('joinRemotePath', () => {
  it('joins a root dir without doubling the slash', () => {
    expect(joinRemotePath('/', 'file.txt')).toBe('/file.txt')
  })

  it('joins an empty dir the same as root', () => {
    expect(joinRemotePath('', 'file.txt')).toBe('/file.txt')
  })

  it('joins a nested dir', () => {
    expect(joinRemotePath('/var/www', 'app.log')).toBe('/var/www/app.log')
  })

  it('strips a trailing slash on the dir before joining', () => {
    expect(joinRemotePath('/var/www/', 'app.log')).toBe('/var/www/app.log')
  })
})

describe('parentRemotePath', () => {
  it('returns root for a top-level entry', () => {
    expect(parentRemotePath('/file.txt')).toBe('/')
  })

  it('returns the containing directory for a nested entry', () => {
    expect(parentRemotePath('/var/www/app.log')).toBe('/var/www')
  })

  it('returns root for root itself', () => {
    expect(parentRemotePath('/')).toBe('/')
  })

  it('ignores a trailing slash', () => {
    expect(parentRemotePath('/var/www/')).toBe('/var')
  })
})
