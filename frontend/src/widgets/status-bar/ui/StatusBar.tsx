import { useSessionStore } from '../../../entities/session'
import { StatusDot, type DotStatus } from '../../../shared/ui'

interface StatusBarProps {
  sessionId: string | null
}

const DOT_STATUS: Record<string, DotStatus> = {
  running: 'running',
  connecting: 'connecting',
  error: 'error',
}

const STATE_LABEL: Record<string, string> = {
  connecting: '연결 중',
  starting: '연결 중',
  running: '연결됨',
  closed: '유휴',
  error: '오류',
}

export function StatusBar({ sessionId }: StatusBarProps) {
  const session = useSessionStore((s) => (sessionId ? s.sessions[sessionId] : undefined))

  return (
    <div className="flex-none h-8 flex items-center gap-4.5 px-4.5 text-[11.5px] bg-surface border-t border-line text-fg2">
      {session ? (
        <>
          <span>{session.kind === 'ssh' ? 'SSH' : session.shell || 'shell'}</span>
          <span className="font-mono">
            {session.cols} × {session.rows}
          </span>
          <div className="flex-1" />
          <span className="flex items-center gap-1.5">
            <StatusDot status={DOT_STATUS[session.state] ?? 'idle'} />
            {STATE_LABEL[session.state] ?? session.state}
          </span>
        </>
      ) : (
        <span>연결 중...</span>
      )}
    </div>
  )
}
