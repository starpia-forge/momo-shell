import { useSessionStore } from '../../../entities/session'

interface StatusBarProps {
  sessionId: string | null
}

export function StatusBar({ sessionId }: StatusBarProps) {
  const session = useSessionStore((s) => (sessionId ? s.sessions[sessionId] : undefined))

  return (
    <div className="flex-none h-6 flex items-center gap-4 px-3 text-[12px] bg-surface border-t border-line text-muted">
      {session ? (
        <>
          <span>{session.kind === 'ssh' ? 'SSH' : session.shell || 'shell'}</span>
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
