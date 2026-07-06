import { Copy, HomeDir, ListDir, Mkdir, Move, Remove, Rename, Roots, Stat } from '../../../wailsjs/go/wails/LocalFSService'
import type { RemoteEntry } from './transfer'

export async function listLocalDir(path: string): Promise<RemoteEntry[]> {
  return ListDir(path)
}

export async function localHomeDir(): Promise<string> {
  return HomeDir()
}

export async function localRoots(): Promise<string[]> {
  return Roots()
}

/** Wails only supports (value, error) returns; Go folds "not found" into a
 * nil pointer, which crosses the wire as null. */
export async function statLocal(path: string): Promise<RemoteEntry | null> {
  return (await Stat(path)) as unknown as RemoteEntry | null
}

export async function mkdirLocal(path: string): Promise<void> {
  await Mkdir(path)
}

export async function renameLocal(oldPath: string, newPath: string): Promise<void> {
  await Rename(oldPath, newPath)
}

export async function removeLocal(path: string): Promise<void> {
  await Remove(path)
}

export async function copyLocal(srcPaths: string[], dstDir: string): Promise<void> {
  await Copy(srcPaths, dstDir)
}

export async function moveLocal(srcPaths: string[], dstDir: string): Promise<void> {
  await Move(srcPaths, dstDir)
}

/** Returns which of names already exist in localDir -- the download-direction
 * mirror of shared/api/transfer's detectUploadConflicts. */
export async function detectLocalConflicts(names: string[], localDir: string): Promise<string[]> {
  const entries = await listLocalDir(localDir)
  const existing = new Set(entries.map((e) => e.name))
  return names.filter((name) => existing.has(name))
}
