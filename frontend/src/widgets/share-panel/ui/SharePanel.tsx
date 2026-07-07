import { useEffect } from 'react'
import { useHostStore } from '../../../entities/host'
import { Button } from '../../../shared/ui'
import { useSharePanelStore } from '../model/store'
import { loadSharePanel, enableSharing, disableSharing, updateSharedHosts, revokeClient } from '../lib/actions'

export function SharePanel() {
  const status = useSharePanelStore((s) => s.status)
  const clients = useSharePanelStore((s) => s.clients)
  const hosts = useHostStore((s) => s.hosts)

  useEffect(() => {
    void loadSharePanel()
  }, [])

  if (!status) return null

  const sharedIds = new Set(status.sharedHostIds)
  const hostList = Object.values(hosts)

  function toggleHost(id: string, checked: boolean) {
    const next = checked ? [...status!.sharedHostIds, id] : status!.sharedHostIds.filter((existing) => existing !== id)
    void updateSharedHosts(next)
  }

  return (
    <div className="flex flex-col gap-4 p-3.5 overflow-y-auto text-[13px] text-fg">
      <span className="text-[13.5px] font-bold">공유</span>
      <div className="flex items-center justify-between">
        <span className="text-fg2">공유 상태</span>
        <Button
          variant={status.enabled ? 'primary' : 'default'}
          onClick={() => void (status.enabled ? disableSharing() : enableSharing(status.sharedHostIds))}
        >
          {status.enabled ? '켜짐' : '꺼짐'}
        </Button>
      </div>

      {status.enabled && (
        <div className="flex flex-col gap-1 p-3 bg-inputbg rounded-md">
          <span className="text-[11px] text-fg2">페어링 PIN</span>
          <span className="font-mono text-[22px] tracking-[0.15em]">{status.pin}</span>
          <span className="text-[11px] text-fg2">연결 요청 시 상대에게 알려주세요</span>
        </div>
      )}

      <div className="flex flex-col gap-1.5">
        <div className="text-[11px] font-bold text-fg3 tracking-wide">
          공유할 호스트 ({hostList.length}개 중 {sharedIds.size}개 선택)
        </div>
        {hostList.length === 0 ? (
          <div className="text-[12px] text-fg2">등록된 호스트가 없습니다</div>
        ) : (
          <ul className="flex flex-col gap-1 m-0 p-0">
            {hostList.map((h) => (
              <li key={h.id}>
                <label className="flex items-center gap-1.5 cursor-pointer">
                  <input type="checkbox" checked={sharedIds.has(h.id)} onChange={(e) => toggleHost(h.id, e.target.checked)} />
                  {h.name}
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="text-[11px] font-bold text-fg3 tracking-wide">연결된 피어</div>
        {clients.length === 0 ? (
          <div className="text-[12px] text-fg2">아직 연결된 피어가 없습니다</div>
        ) : (
          <ul className="flex flex-col gap-1 m-0 p-0">
            {clients.map((c) => (
              <li key={c.id} className="flex items-center justify-between gap-2">
                <span>{c.name}</span>
                <Button variant="danger" onClick={() => void revokeClient(c.id)}>
                  회수
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
