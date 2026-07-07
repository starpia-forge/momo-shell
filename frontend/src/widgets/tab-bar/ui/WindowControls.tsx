import { useEffect, useState } from 'react'
import { Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../../../wailsjs/runtime/runtime'

export function WindowControls() {
  const [maximised, setMaximised] = useState(false)

  useEffect(() => {
    void WindowIsMaximised().then(setMaximised)
    const onResize = () => void WindowIsMaximised().then(setMaximised)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  function handleToggleMaximise() {
    WindowToggleMaximise()
    void WindowIsMaximised().then(setMaximised)
  }

  return (
    <div className="flex items-center gap-0.5 [--wails-draggable:no-drag]">
      <button
        className="w-[34px] h-[30px] rounded-lg flex items-center justify-center text-fg2 bg-transparent border-none cursor-pointer hover:bg-surface2"
        onClick={() => WindowMinimise()}
        aria-label="최소화"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round">
          <line x1="1.5" y1="6" x2="10.5" y2="6" />
        </svg>
      </button>
      <button
        className="w-[34px] h-[30px] rounded-lg flex items-center justify-center text-fg2 bg-transparent border-none cursor-pointer hover:bg-surface2"
        onClick={handleToggleMaximise}
        aria-label={maximised ? '이전 크기로 복원' : '최대화'}
      >
        {maximised ? (
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round">
            <rect x="4" y="2" width="6" height="6" rx="1.2" />
            <path d="M2 4v5a1 1 0 0 0 1 1h5" />
          </svg>
        ) : (
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round">
            <rect x="2" y="2" width="8" height="8" rx="1.5" />
          </svg>
        )}
      </button>
      <button
        className="w-[34px] h-[30px] rounded-lg flex items-center justify-center text-fg2 bg-transparent border-none cursor-pointer hover:bg-red/20 hover:text-red"
        onClick={() => Quit()}
        aria-label="닫기"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round">
          <line x1="2" y1="2" x2="10" y2="10" />
          <line x1="10" y1="2" x2="2" y2="10" />
        </svg>
      </button>
    </div>
  )
}
