import { useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent } from 'react'
import { clearSearchSession, focusSession, searchSession } from '../../../entities/session'
import './SearchOverlay.css'

interface SearchOverlayProps {
  sessionId: string
  onClose: () => void
}

export function SearchOverlay({ sessionId, onClose }: SearchOverlayProps) {
  const [term, setTerm] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleClose() {
    clearSearchSession(sessionId)
    onClose()
    focusSession(sessionId)
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault()
      searchSession(sessionId, term, e.shiftKey ? 'prev' : 'next')
    } else if (e.key === 'Escape') {
      e.preventDefault()
      handleClose()
    }
  }

  return (
    <div className="terminal-search">
      <input
        ref={inputRef}
        className="terminal-search__input"
        value={term}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setTerm(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="검색 (Enter: 다음, Shift+Enter: 이전)"
      />
      <button className="terminal-search__close" onClick={handleClose} aria-label="검색 닫기">
        ×
      </button>
    </div>
  )
}
