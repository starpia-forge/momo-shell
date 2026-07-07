import { useLayoutEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { Button } from '../../../shared/ui'
import { attach, detach, fitSession } from '../lib/terminal-registry'
import { useSessionStore } from '../model/store'

interface TerminalPaneProps {
  sessionId: string
  /** Only meaningful for kind: 'ssh' -- shows a [재연결] button on the disconnect overlay. */
  onReconnect?: () => void
}

// Attach/detach only -- the Terminal instance itself lives in the registry
// for the session's lifetime, independent of this component's mount state.
export function TerminalPane({ sessionId, onReconnect }: TerminalPaneProps) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement>(null)
  const session = useSessionStore((s) => s.sessions[sessionId])

  useLayoutEffect(() => {
    const container = containerRef.current
    if (!container) return

    attach(sessionId, container)

    let trailingTimer: ReturnType<typeof setTimeout> | undefined
    const scheduleFit = () => {
      fitSession(sessionId)
      clearTimeout(trailingTimer)
      trailingTimer = setTimeout(() => fitSession(sessionId), 100)
    }

    const ro = new ResizeObserver(scheduleFit)
    ro.observe(container)

    return () => {
      ro.disconnect()
      clearTimeout(trailingTimer)
      detach(sessionId)
    }
  }, [sessionId])

  const ended = session?.state === 'closed' || session?.state === 'error'
  const isSSH = session?.kind === 'ssh'

  return (
    <div className="terminal-pane relative w-full h-full">
      <div ref={containerRef} className={cn('w-full h-full', ended && 'opacity-40')} />
      {ended && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="flex flex-col items-center gap-3 rounded-xl px-6 py-5 bg-surface2 border border-line shadow-menu text-fg text-[13px] pointer-events-auto">
            {isSSH ? (
              <>
                <div>{session?.error ? t('session.disconnectedWithError', { error: session.error }) : t('session.disconnected')}</div>
                {onReconnect && (
                  <Button variant="primary" onClick={onReconnect}>
                    {t('session.reconnect')}
                  </Button>
                )}
              </>
            ) : session?.state === 'error' ? (
              session.error ? t('session.errorWithMessage', { error: session.error }) : t('session.error')
            ) : session?.exitCode !== undefined ? (
              t('session.endedWithExitCode', { exitCode: session.exitCode })
            ) : (
              t('session.ended')
            )}
          </div>
        </div>
      )}
    </div>
  )
}
