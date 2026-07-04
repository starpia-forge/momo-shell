// Wails' native file drop (see WithFileSystemFactory-style OS integration in
// the Go backend) reports absolute paths via a Go-side event, decoupled from
// the browser's drag events -- so by the time os:filedrop fires we can no
// longer read which DOM element the pointer was over from the event itself.
// Panes and the file browser both call markDropTargetHovered on their own
// dragover handlers; the eventual os:filedrop handler resolves against
// whichever target was hovered most recently, falling back to
// elementFromPoint for platforms where drop coordinates can drift from the
// last dragover (e.g. Windows DPI scaling).
const FRESHNESS_MS = 1000

export interface DropTargetInfo {
  sessionId: string
  isFileBrowser: boolean
  at: number
}

let hovered: DropTargetInfo | null = null

export function markDropTargetHovered(sessionId: string, isFileBrowser = false): void {
  hovered = { sessionId, isFileBrowser, at: Date.now() }
}

export function resolveDropTarget(x: number, y: number): DropTargetInfo | null {
  if (hovered && Date.now() - hovered.at < FRESHNESS_MS) return hovered

  const el = document.elementFromPoint(x, y)
  const withSession = el?.closest<HTMLElement>('[data-session-id]')
  const sessionId = withSession?.dataset.sessionId
  if (!sessionId) return null
  return { sessionId, isFileBrowser: withSession.dataset.filebrowser === 'true', at: Date.now() }
}
