import { useEffect, useRef, useState } from 'react'
import { openLocalSession, TerminalPane } from './entities/session'

function App() {
  const [sessionId, setSessionId] = useState<string | null>(null)
  const opened = useRef(false)

  useEffect(() => {
    if (opened.current) return
    opened.current = true
    openLocalSession({ cols: 80, rows: 24 }).then(setSessionId)
  }, [])

  return (
    <div id="App" style={{ width: '100vw', height: '100vh' }}>
      {sessionId && <TerminalPane sessionId={sessionId} />}
    </div>
  )
}

export default App
