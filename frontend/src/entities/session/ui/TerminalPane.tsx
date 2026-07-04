import { useLayoutEffect, useRef } from 'react'
import { attach, detach, fitSession } from '../lib/terminal-registry'
import { useSessionStore } from '../model/store'
import './TerminalPane.css'

interface TerminalPaneProps {
  sessionId: string
}

// Attach/detach only -- the Terminal instance itself lives in the registry
// for the session's lifetime, independent of this component's mount state.
export function TerminalPane({ sessionId }: TerminalPaneProps) {
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

  return (
    <div className="terminal-pane">
      <div ref={containerRef} className={ended ? 'terminal-pane__host terminal-pane__host--dimmed' : 'terminal-pane__host'} />
      {ended && (
        <div className="terminal-pane__overlay">
          <div className="terminal-pane__overlay-message">
            {session?.state === 'error'
              ? `세션 오류${session.error ? `: ${session.error}` : ''}`
              : `세션 종료${session?.exitCode !== undefined ? ` (exit ${session.exitCode})` : ''}`}
          </div>
        </div>
      )}
    </div>
  )
}
