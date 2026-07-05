import { useEffect } from 'react'
import { useHostStore } from '../../../entities/host'
import { Button } from '../../../shared/ui'
import { useSharePanelStore } from '../model/store'
import { loadSharePanel, enableSharing, disableSharing, updateSharedHosts, revokeClient } from '../lib/actions'
import './SharePanel.css'

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
    <div className="share-panel">
      <div className="share-panel__row">
        <span className="share-panel__row-label">공유 상태</span>
        <Button
          variant={status.enabled ? 'primary' : 'default'}
          onClick={() => void (status.enabled ? disableSharing() : enableSharing(status.sharedHostIds))}
        >
          {status.enabled ? '켜짐' : '꺼짐'}
        </Button>
      </div>

      {status.enabled && (
        <div className="share-panel__pin">
          <span className="share-panel__pin-label">페어링 PIN</span>
          <span className="share-panel__pin-value">{status.pin}</span>
          <span className="share-panel__pin-hint">연결 요청 시 상대에게 알려주세요</span>
        </div>
      )}

      <div className="share-panel__section">
        <div className="share-panel__section-title">
          공유할 호스트 ({hostList.length}개 중 {sharedIds.size}개 선택)
        </div>
        {hostList.length === 0 ? (
          <div className="share-panel__empty">등록된 호스트가 없습니다</div>
        ) : (
          <ul className="share-panel__host-list">
            {hostList.map((h) => (
              <li key={h.id} className="share-panel__host-item">
                <label>
                  <input type="checkbox" checked={sharedIds.has(h.id)} onChange={(e) => toggleHost(h.id, e.target.checked)} />
                  {h.name}
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="share-panel__section">
        <div className="share-panel__section-title">연결된 피어</div>
        {clients.length === 0 ? (
          <div className="share-panel__empty">아직 연결된 피어가 없습니다</div>
        ) : (
          <ul className="share-panel__client-list">
            {clients.map((c) => (
              <li key={c.id} className="share-panel__client-item">
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
