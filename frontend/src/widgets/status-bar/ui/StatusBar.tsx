import { useSessionStore } from '../../../entities/session'
import './StatusBar.css'

interface StatusBarProps {
  sessionId: string | null
}

export function StatusBar({ sessionId }: StatusBarProps) {
  const session = useSessionStore((s) => (sessionId ? s.sessions[sessionId] : undefined))

  return (
    <div className="status-bar">
      {session ? (
        <>
          <span>{session.shell || 'shell'}</span>
          <span>
            {session.cols}×{session.rows}
          </span>
          <span>{session.state}</span>
        </>
      ) : (
        <span>연결 중...</span>
      )}
    </div>
  )
}
