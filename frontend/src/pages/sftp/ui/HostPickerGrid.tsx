import { useEffect, useMemo, useState } from 'react'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { Button, SearchInput, SegmentedControl, StatusDot, type DotStatus } from '../../../shared/ui'
import { useSftpStore } from '../model/store'

interface HostPickerGridProps {
  onBack: () => void
  onRegister: () => void
}

type SortMode = 'recent' | 'name'

/** R5/G2: the "연결" flow's host list -- visually mirrors the home page's
 * SavedHostsGrid (card grid, search, sort, status dot) but connects through
 * the SFTP store's own session (not connectHost()+onConnect), and omits the
 * edit/clone/delete context menu since this picker is connect-only. */
export function HostPickerGrid({ onBack, onRegister }: HostPickerGridProps) {
  const hosts = useHostStore((s) => s.hosts)
  const load = useHostStore((s) => s.load)
  const sessions = useSessionStore((s) => s.sessions)
  const connect = useSftpStore((s) => s.connect)
  const gateError = useSftpStore((s) => s.gateError)

  const [query, setQuery] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('recent')

  useEffect(() => {
    void load()
  }, [load])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    let list = Object.values(hosts)
    if (q) {
      list = list.filter(
        (h) =>
          h.name.toLowerCase().includes(q) ||
          h.address.toLowerCase().includes(q) ||
          h.labels.some((l) => l.toLowerCase().includes(q))
      )
    }
    return [...list].sort((a, b) =>
      sortMode === 'name' ? a.name.localeCompare(b.name) : (b.lastConnectedAt ?? 0) - (a.lastConnectedAt ?? 0)
    )
  }, [hosts, query, sortMode])

  function statusFor(hostId: string): DotStatus {
    const match = Object.values(sessions).find((s) => s.hostId === hostId && s.state !== 'closed')
    if (!match) return 'idle'
    if (match.state === 'running') return 'running'
    if (match.state === 'connecting') return 'connecting'
    if (match.state === 'error') return 'error'
    return 'idle'
  }

  function handleConnect(host: Host) {
    void connect(host)
  }

  return (
    <div className="flex-1 min-w-0 overflow-y-auto p-5">
      <div className="flex items-center gap-4 mb-4">
        <button className="border-none bg-transparent text-fg2 cursor-pointer text-[12.5px] hover:text-fg" onClick={onBack}>
          ← 돌아가기
        </button>
        <span className="text-[15px] font-bold">내 호스트</span>
        <span className="text-[12.5px] text-fg3">{filtered.length}개</span>
        <div className="flex-1" />
        <SearchInput containerClassName="w-60 h-8.5" placeholder="이름, 주소, 라벨 검색" value={query} onChange={(e) => setQuery(e.target.value)} />
        <SegmentedControl
          value={sortMode}
          onChange={setSortMode}
          options={[
            { value: 'recent', label: '최근 연결' },
            { value: 'name', label: '이름순' },
          ]}
        />
      </div>

      {gateError && <div className="text-[12px] text-red mb-3">{gateError}</div>}

      <div className="grid grid-cols-3 gap-3.5">
        {filtered.map((host) => (
          <div
            key={host.id}
            className="flex flex-col gap-2.5 px-4.5 py-4 rounded-xl border border-line bg-surface cursor-pointer text-left hover:border-accent"
            onDoubleClick={() => handleConnect(host)}
          >
            <div className="flex items-center gap-2 min-w-0">
              <StatusDot status={statusFor(host.id)} />
              <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13.5px] font-bold">{host.name}</span>
            </div>
            <div className="text-[11.5px] font-mono text-fg2 overflow-hidden text-ellipsis whitespace-nowrap">{host.address}</div>
            <Button variant="primary" size="sm" className="self-start" onClick={() => handleConnect(host)}>
              연결
            </Button>
          </div>
        ))}

        <button
          className="flex flex-col gap-1.5 px-4.5 py-4 rounded-xl border border-dashed border-line bg-surface cursor-pointer items-center justify-center text-fg2 text-[13px] hover:text-fg"
          onClick={onRegister}
        >
          ＋ 새 호스트
        </button>
      </div>

      {filtered.length === 0 && <div className="text-fg2 text-[12.5px] py-2">호스트를 추가해 시작하세요</div>}
    </div>
  )
}
