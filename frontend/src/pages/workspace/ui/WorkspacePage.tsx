import { useEffect, useRef, useState } from 'react'
import { openLocalSession, TerminalPane } from '../../../entities/session'
import { StatusBar } from '../../../widgets/status-bar'
import './WorkspacePage.css'

export function WorkspacePage() {
  const [sessionId, setSessionId] = useState<string | null>(null)
  const opened = useRef(false)

  useEffect(() => {
    if (opened.current) return
    opened.current = true
    openLocalSession({ cols: 80, rows: 24 }).then(setSessionId)
  }, [])

  return (
    <div className="workspace">
      <div className="workspace__tab-bar" />
      <div className="workspace__body">
        <div className="workspace__sidebar" />
        <div className="workspace__pane">{sessionId && <TerminalPane sessionId={sessionId} />}</div>
      </div>
      <StatusBar sessionId={sessionId} />
    </div>
  )
}
