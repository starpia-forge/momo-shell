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
    <div className="absolute top-1 right-1 z-25 flex items-center gap-1 px-1.5 py-1 rounded bg-surface border border-line shadow-float">
      <input
        ref={inputRef}
        className="w-55 px-1.5 py-1 rounded-sm border border-line bg-canvas text-fg text-[12px]"
        value={term}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setTerm(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="검색 (Enter: 다음, Shift+Enter: 이전)"
      />
      <button
        className="border-none bg-transparent text-fg2 cursor-pointer text-[13px] leading-none px-1 py-0.5 hover:text-fg"
        onClick={handleClose}
        aria-label="검색 닫기"
      >
        ×
      </button>
    </div>
  )
}
