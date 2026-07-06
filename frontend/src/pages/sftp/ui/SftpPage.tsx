import { useEffect } from 'react'
import { localOps } from '../model/paneOps'
import { useSftpStore } from '../model/store'
import { ConnectGate } from './ConnectGate'
import { FilePane } from './FilePane'

/** R3: two panes -- Local Directory (left) and Remote Directory (right).
 * Occupies the workspace's full center row, like the settings screen,
 * since the dual-pane layout needs the width the host sidebar/right dock
 * would otherwise take. */
export function SftpPage() {
  const sessionId = useSftpStore((s) => s.sessionId)
  const remoteOps = useSftpStore((s) => s.remoteOps)
  const initLocal = useSftpStore((s) => s.initLocal)
  const disconnect = useSftpStore((s) => s.disconnect)

  useEffect(() => {
    void initLocal()
  }, [initLocal])

  return (
    <div className="sftp-page flex-1 min-w-0 h-full flex bg-canvas text-fg">
      <FilePane side="local" ops={localOps} />
      {sessionId && remoteOps ? <FilePane side="remote" ops={remoteOps} onDisconnect={disconnect} /> : <ConnectGate />}
    </div>
  )
}
