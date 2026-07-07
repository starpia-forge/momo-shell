import { useEffect, useState, type ReactNode } from 'react'
import { Button, IconButton } from '../../../shared/ui'
import type { PaneOps } from '../model/paneOps'

interface PaneHeaderProps {
  title: string
  subtitle: ReactNode
  path: string
  ops: PaneOps
  onNavigate: (path: string) => void
  onNewFolder: () => void
  /** Remote pane only -- shows the "연결 해제" action when provided. */
  onDisconnect?: () => void
}

/** R4: identity + action row, then a path bar, at the top of each SFTP pane.
 * Shared by both sides via PaneOps. */
export function PaneHeader({ title, subtitle, path, ops, onNavigate, onNewFolder, onDisconnect }: PaneHeaderProps) {
  const [pathInput, setPathInput] = useState(path)

  useEffect(() => {
    setPathInput(path)
  }, [path])

  const parent = ops.parent(path)

  return (
    <div className="flex-none border-b border-line">
      <div className="flex items-center gap-2.5 h-11.5 px-4">
        <span className="text-[13.5px] font-bold">{title}</span>
        {subtitle}
        <div className="flex-1" />
        <Button size="sm" onClick={onNewFolder}>
          새 폴더
        </Button>
        {onDisconnect && (
          <Button size="sm" onClick={onDisconnect}>
            연결 해제
          </Button>
        )}
      </div>
      <div className="flex items-center gap-2.5 h-10 px-3 border-t border-line">
        <IconButton size={28} className="text-[13px]" onClick={() => parent !== null && onNavigate(parent)} disabled={parent === null} aria-label="상위 폴더" title="상위 폴더">
          ↑
        </IconButton>
        <input
          className="flex-1 min-w-0 px-3 py-1.75 rounded-md border border-line bg-inputbg text-fg2 font-mono text-[12px] focus:outline-none focus:border-accent"
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onNavigate(pathInput)
          }}
        />
        <IconButton size={28} className="text-[12px]" onClick={() => onNavigate(path)} aria-label="새로고침" title="새로고침">
          ⟳
        </IconButton>
        <IconButton size={28} className="text-[12px]" onClick={() => void ops.homeDir().then(onNavigate)} aria-label="홈" title="홈">
          ~
        </IconButton>
      </div>
    </div>
  )
}
