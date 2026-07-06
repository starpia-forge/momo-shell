import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DRIVES_VIEW } from '../lib/paths'

const localfsMocks = vi.hoisted(() => ({
  listLocalDir: vi.fn(),
  localHomeDir: vi.fn(),
  localRoots: vi.fn(),
  mkdirLocal: vi.fn(),
  renameLocal: vi.fn(),
  removeLocal: vi.fn(),
  statLocal: vi.fn(),
  copyLocal: vi.fn(),
  moveLocal: vi.fn(),
}))
vi.mock('../../../shared/api/localfs', () => localfsMocks)

const transferMocks = vi.hoisted(() => ({
  copyRemoteFiles: vi.fn(),
  homeDir: vi.fn(),
  listRemoteDir: vi.fn(),
  mkdirRemote: vi.fn(),
  removeRemote: vi.fn(),
  renameRemote: vi.fn(),
  statRemote: vi.fn(),
}))
vi.mock('../../../shared/api/transfer', () => transferMocks)

const { localOps, makeRemoteOps } = await import('./paneOps')

beforeEach(() => {
  vi.clearAllMocks()
})

describe('localOps.list', () => {
  it('synthesizes drive entries for DRIVES_VIEW', async () => {
    localfsMocks.localRoots.mockResolvedValue(['C:\\', 'D:\\'])
    const entries = await localOps.list(DRIVES_VIEW)
    expect(entries).toEqual([
      { name: 'C:\\', path: 'C:\\', size: 0, mode: 0, modeText: '', modTime: 0, isDir: true },
      { name: 'D:\\', path: 'D:\\', size: 0, mode: 0, modeText: '', modTime: 0, isDir: true },
    ])
    expect(localfsMocks.listLocalDir).not.toHaveBeenCalled()
  })

  it('passes through to listLocalDir for a real path', async () => {
    localfsMocks.listLocalDir.mockResolvedValue([])
    await localOps.list('C:\\Users')
    expect(localfsMocks.listLocalDir).toHaveBeenCalledWith('C:\\Users')
  })
})

describe('localOps path helpers', () => {
  it('parent walks up to the drives view', () => {
    expect(localOps.parent('C:\\')).toBe(DRIVES_VIEW)
  })

  it('baseName returns the last segment', () => {
    expect(localOps.baseName('C:\\a\\b')).toBe('b')
  })
})

describe('makeRemoteOps', () => {
  it('list delegates to listRemoteDir with the bound session', async () => {
    transferMocks.listRemoteDir.mockResolvedValue([])
    const ops = makeRemoteOps('s1')
    await ops.list('/home')
    expect(transferMocks.listRemoteDir).toHaveBeenCalledWith('s1', '/home')
  })

  it('parent returns null at the remote root instead of "/"', () => {
    const ops = makeRemoteOps('s1')
    expect(ops.parent('/')).toBeNull()
    expect(ops.parent('/home/u')).toBe('/home')
  })

  it('moveWithin renames each source into dstDir sequentially', async () => {
    transferMocks.renameRemote.mockResolvedValue(undefined)
    const ops = makeRemoteOps('s1')
    await ops.moveWithin(['/a/one.txt', '/b/two.txt'], '/dst')
    expect(transferMocks.renameRemote).toHaveBeenNthCalledWith(1, 's1', '/a/one.txt', '/dst/one.txt')
    expect(transferMocks.renameRemote).toHaveBeenNthCalledWith(2, 's1', '/b/two.txt', '/dst/two.txt')
  })

  it('copyWithin delegates to copyRemoteFiles with the bound session', async () => {
    transferMocks.copyRemoteFiles.mockResolvedValue(undefined)
    const ops = makeRemoteOps('s1')
    await ops.copyWithin(['/a.txt'], '/dst')
    expect(transferMocks.copyRemoteFiles).toHaveBeenCalledWith('s1', ['/a.txt'], '/dst')
  })
})
