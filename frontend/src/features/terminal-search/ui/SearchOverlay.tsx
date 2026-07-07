import { useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent } from 'react'
import { clearSearchSession, focusSession, searchSession } from '../../../entities/session'

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
    <div className="absolute top-9 right-3 z-25 flex items-center gap-2.5 px-3 py-1.75 rounded-md bg-surface2 border border-line shadow-float">
      <input
        ref={inputRef}
        className="w-55 px-2 py-1 rounded-sm border border-line bg-inputbg text-fg font-mono text-[12.5px] focus:outline-none focus:border-accent"
        value={term}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setTerm(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="검색 (Enter: 다음, Shift+Enter: 이전)"
      />
      <button
        className="border-none bg-transparent text-fg3 cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-fg"
        onClick={handleClose}
        aria-label="검색 닫기"
      >
        ×
      </button>
    </div>
  )
}
