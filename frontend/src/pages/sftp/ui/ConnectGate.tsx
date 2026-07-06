import { useState } from 'react'
import type { Host } from '../../../entities/host'
import { HostFormDialog } from '../../../features/host-crud'
import { Spinner } from '../../../shared/ui'
import { useSftpStore } from '../model/store'
import { HostPickerGrid } from './HostPickerGrid'

/** Shown in place of the remote pane whenever the SFTP page has no session.
 * idle: connect to a saved host (via the picker) or register+connect a new
 * one. picker: a home-page-style host card grid (G2). `view` is local UI
 * navigation state, not store state -- a successful connect unmounts this
 * component entirely, so the next disconnect naturally restarts at idle. */
export function ConnectGate() {
  const connecting = useSftpStore((s) => s.connecting)
  const gateError = useSftpStore((s) => s.gateError)
  const connect = useSftpStore((s) => s.connect)
  const [view, setView] = useState<'idle' | 'picker'>('idle')
  const [dialogOpen, setDialogOpen] = useState(false)

  function handleSaved(host: Host) {
    setDialogOpen(false)
    void connect(host)
  }

  if (connecting) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center gap-2 text-muted text-[13px]">
        <Spinner size={20} />
        연결 중...
      </div>
    )
  }

  return (
    <div className="flex-1 min-w-0 flex flex-col">
      {view === 'picker' ? (
        <HostPickerGrid onBack={() => setView('idle')} onRegister={() => setDialogOpen(true)} />
      ) : (
        <div className="flex-1 flex flex-col items-center justify-center gap-3 p-4">
          <div className="text-[13px] font-semibold">연결이 필요합니다</div>
          <div className="text-[12px] text-muted text-center">저장된 호스트에 연결하거나 새 호스트를 등록하세요.</div>
          {gateError && <div className="text-[12px] text-danger">{gateError}</div>}
          <button
            className="px-4 py-1.5 rounded border border-accent bg-accent/10 text-accent cursor-pointer text-[12px] hover:bg-accent/20"
            onClick={() => setView('picker')}
          >
            연결
          </button>
          <button
            className="px-3 py-1.5 rounded border border-dashed border-line bg-transparent text-muted cursor-pointer text-[12px] hover:border-accent hover:text-fg"
            onClick={() => setDialogOpen(true)}
          >
            + 새 호스트 등록
          </button>
        </div>
      )}
      {dialogOpen && <HostFormDialog open onClose={() => setDialogOpen(false)} onSaved={handleSaved} />}
    </div>
  )
}
