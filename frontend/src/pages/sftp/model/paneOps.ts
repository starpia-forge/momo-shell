import { joinRemotePath, parentRemotePath } from '../../../widgets/file-browser'
import {
  copyLocal,
  localHomeDir,
  listLocalDir,
  localRoots,
  mkdirLocal,
  moveLocal,
  removeLocal,
  renameLocal,
  statLocal,
} from '../../../shared/api/localfs'
import {
  copyRemoteFiles,
  homeDir as remoteHomeDir,
  listRemoteDir,
  mkdirRemote,
  removeRemote,
  renameRemote,
  statRemote,
  type RemoteEntry,
} from '../../../shared/api/transfer'
import { DRIVES_VIEW, joinLocalPath, localBaseName, parentLocalPath } from '../lib/paths'

/** Per-side directory operations shared by both of the SFTP page's panes --
 * a single FilePane component drives either side through this interface
 * instead of branching on `side` everywhere. Cross-side transfers (upload/
 * download) are session-aware and live in the page store instead, since
 * only the remote side has a session to key off. */
export interface PaneOps {
  side: 'local' | 'remote'
  list(path: string): Promise<RemoteEntry[]>
  homeDir(): Promise<string>
  join(dir: string, name: string): string
  parent(path: string): string | null
  baseName(path: string): string
  mkdir(path: string): Promise<void>
  rename(oldPath: string, newPath: string): Promise<void>
  remove(path: string): Promise<void>
  stat(path: string): Promise<RemoteEntry | null>
  /** Copies/moves within this same side (e.g. local->local, remote->remote). */
  copyWithin(srcPaths: string[], dstDir: string): Promise<void>
  moveWithin(srcPaths: string[], dstDir: string): Promise<void>
}

async function listLocal(path: string): Promise<RemoteEntry[]> {
  if (path === DRIVES_VIEW) {
    const roots = await localRoots()
    return roots.map((root) => ({ name: root, path: root, size: 0, mode: 0, modeText: '', modTime: 0, isDir: true }))
  }
  return listLocalDir(path)
}

export const localOps: PaneOps = {
  side: 'local',
  list: listLocal,
  homeDir: localHomeDir,
  join: joinLocalPath,
  parent: parentLocalPath,
  baseName: localBaseName,
  mkdir: mkdirLocal,
  rename: renameLocal,
  remove: removeLocal,
  stat: statLocal,
  copyWithin: copyLocal,
  moveWithin: moveLocal,
}

function remoteBaseName(p: string): string {
  const trimmed = p.replace(/\/+$/, '')
  if (trimmed === '') return '/'
  const idx = trimmed.lastIndexOf('/')
  return idx < 0 ? trimmed : trimmed.slice(idx + 1)
}

export function makeRemoteOps(sessionId: string): PaneOps {
  return {
    side: 'remote',
    list: (path) => listRemoteDir(sessionId, path),
    homeDir: () => remoteHomeDir(sessionId),
    join: joinRemotePath,
    parent: (path) => (path === '/' ? null : parentRemotePath(path)),
    baseName: remoteBaseName,
    mkdir: (path) => mkdirRemote(sessionId, path),
    rename: (oldPath, newPath) => renameRemote(sessionId, oldPath, newPath),
    remove: (path) => removeRemote(sessionId, path),
    stat: (path) => statRemote(sessionId, path),
    copyWithin: (srcPaths, dstDir) => copyRemoteFiles(sessionId, srcPaths, dstDir),
    moveWithin: (srcPaths, dstDir) =>
      srcPaths.reduce<Promise<void>>(
        (chain, src) => chain.then(() => renameRemote(sessionId, src, joinRemotePath(dstDir, remoteBaseName(src)))),
        Promise.resolve()
      ),
  }
}
