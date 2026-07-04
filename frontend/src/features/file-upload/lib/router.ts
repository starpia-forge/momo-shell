import { useSessionStore } from '../../../entities/session'
import { homeDir, subscribe, topics, writeSession, type FileDropPayload } from '../../../shared/api'
import { resolveDropTarget } from '../../../shared/lib/fileDropTarget'
import { useFileUploadStore } from '../model/store'
import { shellQuotePath } from './shellQuote'

async function resolveUploadDir(sessionId: string): Promise<string> {
  const cwd = useSessionStore.getState().sessions[sessionId]?.cwd
  if (cwd) return cwd
  try {
    return await homeDir(sessionId)
  } catch {
    return '/'
  }
}

/** Routes a native OS file drop to whichever pane was hovered: local panes
 * get the quoted path typed in (conventional terminal drop behavior); SSH
 * panes open the destination confirm bar. Drops on the file browser are
 * handled by that widget itself (it owns its own drop target), not here. */
export function registerFileDropRouter(): () => void {
  return subscribe<FileDropPayload>(topics.osFileDrop(), (payload) => {
    const target = resolveDropTarget(payload.x, payload.y)
    if (!target || target.isFileBrowser) return

    const session = useSessionStore.getState().sessions[target.sessionId]
    if (!session) return

    if (session.kind === 'local') {
      const quoted = payload.paths.map(shellQuotePath).join(' ')
      void writeSession(target.sessionId, new TextEncoder().encode(quoted))
      return
    }

    void resolveUploadDir(target.sessionId).then((cwd) => {
      useFileUploadStore.getState().setPending({ sessionId: target.sessionId, paths: payload.paths, cwd })
    })
  })
}
