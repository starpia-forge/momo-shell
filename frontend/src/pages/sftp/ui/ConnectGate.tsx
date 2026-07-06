import { useEffect, useState } from 'react'
import { useHostStore, type Host } from '../../../entities/host'
import { HostFormDialog } from '../../../features/host-crud'
import { Spinner } from '../../../shared/ui'
import { useSftpStore } from '../model/store'

/** Shown in place of the remote pane whenever the SFTP page has no session.
 * Lets the user connect to a saved host or register+connect a new one. */
export function ConnectGate() {
  const hosts = useHostStore((s) => s.hosts)
  const load = useHostStore((s) => s.load)
  const connecting = useSftpStore((s) => s.connecting)
  const gateError = useSftpStore((s) => s.gateError)
  const connect = useSftpStore((s) => s.connect)
  const [dialogOpen, setDialogOpen] = useState(false)

  useEffect(() => {
    void load()
  }, [load])

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

  const hostList = Object.values(hosts)

  return (
    <div className="flex-1 flex flex-col items-center justify-center gap-3 p-4">
      <div className="text-[13px] font-semibold">연결이 필요합니다</div>
      <div className="text-[12px] text-muted text-center">저장된 호스트에 연결하거나 새 호스트를 등록하세요.</div>
      {gateError && <div className="text-[12px] text-danger">{gateError}</div>}
      <div className="w-70 max-h-50 overflow-y-auto rounded border border-line">
        {hostList.map((host) => (
          <div
            key={host.id}
            className="flex flex-col gap-0.5 px-2.5 py-1.5 cursor-pointer hover:bg-canvas text-[13px]"
            onDoubleClick={() => void connect(host)}
            onKeyDown={(e) => e.key === 'Enter' && void connect(host)}
            tabIndex={0}
            role="button"
          >
            <span>{host.name}</span>
            <span className="text-[11px] text-muted">{host.address}</span>
          </div>
        ))}
        {hostList.length === 0 && <div className="px-2.5 py-3 text-center text-[12px] text-muted">저장된 호스트가 없습니다</div>}
      </div>
      <button
        className="px-3 py-1.5 rounded border border-dashed border-line bg-transparent text-muted cursor-pointer text-[12px] hover:border-accent hover:text-fg"
        onClick={() => setDialogOpen(true)}
      >
        + 새 호스트 등록
      </button>
      {dialogOpen && <HostFormDialog open onClose={() => setDialogOpen(false)} onSaved={handleSaved} />}
    </div>
  )
}
