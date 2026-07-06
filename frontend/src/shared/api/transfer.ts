import {
  BrowseForDownloadDirectory,
  BrowseForUploadFiles,
  CancelTransfer,
  CancelZmodem,
  Chmod,
  CopyRemote,
  Download,
  HomeDir,
  ListRemoteDir,
  ListTasks,
  Mkdir,
  Remove,
  Rename,
  StartZmodemSend,
  StatRemote,
  Upload,
} from '../../../wailsjs/go/wails/TransferService'

export interface RemoteEntry {
  name: string
  path: string
  size: number
  mode: number
  modeText: string
  modTime: number // unix ms
  isDir: boolean
}

export type ConflictPolicy = 'overwrite' | 'rename' | 'skip'
export type TaskKind = 'upload' | 'download'
export type TaskState = 'queued' | 'running' | 'done' | 'failed' | 'canceled'

export interface TaskInfo {
  id: string
  sessionId: string
  kind: TaskKind
  state: TaskState
  src: string
  dst: string
  currentFile?: string
  bytes: number
  total: number
  offset: number
  error?: string
}

export async function listRemoteDir(sessionId: string, path: string): Promise<RemoteEntry[]> {
  return ListRemoteDir(sessionId, path)
}

export async function homeDir(sessionId: string): Promise<string> {
  return HomeDir(sessionId)
}

/** Wails only supports (value, error) returns; Go folds "not found" into a
 * nil pointer, which crosses the wire as null. */
export async function statRemote(sessionId: string, path: string): Promise<RemoteEntry | null> {
  return (await StatRemote(sessionId, path)) as unknown as RemoteEntry | null
}

/** Returns which of localPaths' basenames already exist in remoteDir --
 * callers prompt for an overwrite/rename/skip policy before uploadFiles
 * (the backend applies one policy per call, chosen up front by the caller;
 * it never prompts itself). */
export async function detectUploadConflicts(sessionId: string, localPaths: string[], remoteDir: string): Promise<string[]> {
  const entries = await listRemoteDir(sessionId, remoteDir)
  const existing = new Set(entries.map((e) => e.name))
  const basenames = localPaths.map((p) => p.split(/[\\/]/).pop() ?? p)
  return basenames.filter((name) => existing.has(name))
}

export async function uploadFiles(
  sessionId: string,
  localPaths: string[],
  remoteDir: string,
  policy: ConflictPolicy = 'overwrite'
): Promise<string[]> {
  return Upload(sessionId, localPaths, remoteDir, policy)
}

export async function downloadFiles(
  sessionId: string,
  remotePaths: string[],
  localDir: string,
  policy: ConflictPolicy = 'overwrite'
): Promise<string[]> {
  return Download(sessionId, remotePaths, localDir, policy)
}

export async function mkdirRemote(sessionId: string, path: string): Promise<void> {
  await Mkdir(sessionId, path)
}

export async function renameRemote(sessionId: string, oldPath: string, newPath: string): Promise<void> {
  await Rename(sessionId, oldPath, newPath)
}

export async function removeRemote(sessionId: string, path: string): Promise<void> {
  await Remove(sessionId, path)
}

export async function chmodRemote(sessionId: string, path: string, mode: number): Promise<void> {
  await Chmod(sessionId, path, mode)
}

/** Stream-copies files within sessionId's remote file system into dstDir --
 * SFTP has no server-side copy. Files only in v1. */
export async function copyRemoteFiles(sessionId: string, srcPaths: string[], dstDir: string): Promise<void> {
  await CopyRemote(sessionId, srcPaths, dstDir)
}

export async function cancelTransfer(taskId: string): Promise<void> {
  await CancelTransfer(taskId)
}

export async function listTasks(): Promise<TaskInfo[]> {
  return (await ListTasks()) as unknown as TaskInfo[]
}

/** Opens a native multi-file picker; returns [] if the user cancelled. */
export async function browseForUploadFiles(): Promise<string[]> {
  return BrowseForUploadFiles()
}

/** Opens a native folder picker; returns "" if the user cancelled. */
export async function browseForDownloadDirectory(): Promise<string> {
  return BrowseForDownloadDirectory()
}

/** Answers a pending transfer:zmodem "detected" (upload) event with the
 * files the user picked or dropped. Returns the new task's ID. */
export async function startZmodemSend(sessionId: string, localPaths: string[]): Promise<string> {
  return StartZmodemSend(sessionId, localPaths)
}

export async function cancelZmodem(sessionId: string): Promise<void> {
  await CancelZmodem(sessionId)
}
