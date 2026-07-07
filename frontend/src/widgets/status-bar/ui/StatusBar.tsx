import { useTranslation } from 'react-i18next'
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

export function StatusBar({ sessionId }: StatusBarProps) {
  const { t } = useTranslation()
  const STATE_LABEL: Record<string, string> = {
    connecting: t('statusBar.stateConnecting'),
    starting: t('statusBar.stateConnecting'),
    running: t('home.savedHosts.statusRunning'),
    closed: t('home.savedHosts.statusIdle'),
    error: t('home.savedHosts.statusError'),
  }
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
        <span>{t('common.connecting')}</span>
      )}
    </div>
  )
}
