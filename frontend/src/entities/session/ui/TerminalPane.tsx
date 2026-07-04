import { useLayoutEffect, useRef } from 'react'
import { attach, detach, fitSession } from '../lib/terminal-registry'

interface TerminalPaneProps {
  sessionId: string
}

// Attach/detach only -- the Terminal instance itself lives in the registry
// for the session's lifetime, independent of this component's mount state.
export function TerminalPane({ sessionId }: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null)

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

  return <div ref={containerRef} style={{ width: '100%', height: '100%' }} />
}
