/** The local pane's sentinel "path" meaning "show the Roots() listing"
 * instead of a real directory -- used above a Windows drive letter or below
 * a Unix root has nowhere shallower to go. */
export const DRIVES_VIEW = ''

const WINDOWS_DRIVE_RE = /^[A-Za-z]:[\\/]/

export function isWindowsPath(p: string): boolean {
  return WINDOWS_DRIVE_RE.test(p)
}

function sep(p: string): '\\' | '/' {
  return isWindowsPath(p) ? '\\' : '/'
}

/** Joins a local directory and a name using the host OS's separator
 * convention inferred from `dir` (Windows drive-letter paths use `\`,
 * everything else -- including DRIVES_VIEW -- uses `/`). */
export function joinLocalPath(dir: string, name: string): string {
  if (dir === DRIVES_VIEW) return name
  const s = sep(dir)
  return dir.endsWith(s) ? `${dir}${name}` : `${dir}${s}${name}`
}

/** Returns the parent directory, DRIVES_VIEW if `p` is a Windows drive root,
 * or null if `p` is already the shallowest place this pane can go (a Unix
 * root, or DRIVES_VIEW itself). */
export function parentLocalPath(p: string): string | null {
  if (p === DRIVES_VIEW) return null
  if (isWindowsPath(p)) {
    const trimmed = p.replace(/[\\/]+$/, '') // "C:\\" -> "C:", "C:\\a\\" -> "C:\\a"
    if (trimmed.length <= 2) return DRIVES_VIEW // was the drive root itself
    const lastSep = trimmed.lastIndexOf('\\')
    if (lastSep <= 2) return `${trimmed.slice(0, 2)}\\` // one level below the drive root -> drive root
    return trimmed.slice(0, lastSep)
  }
  const trimmed = p.replace(/\/+$/, '')
  if (trimmed === '') return null // was "/"
  const lastSep = trimmed.lastIndexOf('/')
  if (lastSep <= 0) return '/'
  return trimmed.slice(0, lastSep)
}

/** Display name for the pane header -- the last path segment, or the drive
 * root itself (e.g. "C:\\"), or a Unix root "/". */
export function localBaseName(p: string): string {
  if (p === DRIVES_VIEW) return ''
  if (isWindowsPath(p)) {
    const trimmed = p.replace(/[\\/]+$/, '')
    if (trimmed.length <= 2) return `${trimmed}\\` // "C:" -> "C:\\"
    const lastSep = trimmed.lastIndexOf('\\')
    return lastSep < 0 ? trimmed : trimmed.slice(lastSep + 1)
  }
  const trimmed = p.replace(/\/+$/, '')
  if (trimmed === '') return '/'
  const lastSep = trimmed.lastIndexOf('/')
  return lastSep < 0 ? trimmed : trimmed.slice(lastSep + 1)
}
