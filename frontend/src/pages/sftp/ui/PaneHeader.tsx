import { useEffect, useState } from 'react'
import { DRIVES_VIEW } from '../lib/paths'
import type { PaneOps } from '../model/paneOps'

interface PaneHeaderProps {
  path: string
  ops: PaneOps
  onNavigate: (path: string) => void
  onNewFolder: () => void
  /** Remote pane only -- shows the ⏏ disconnect button when provided. */
  onDisconnect?: () => void
}

/** R4: shows the current directory's name and absolute path at the top of
 * each SFTP pane. Shared by both sides via PaneOps. */
export function PaneHeader({ path, ops, onNavigate, onNewFolder, onDisconnect }: PaneHeaderProps) {
  const [pathInput, setPathInput] = useState(path)

  useEffect(() => {
    setPathInput(path)
  }, [path])

  const parent = ops.parent(path)
  const displayName = path === DRIVES_VIEW ? '드라이브' : ops.baseName(path)

  return (
    <div className="flex-none border-b border-line">
      <div className="flex items-center gap-1.5 px-2 py-1.5">
        <button
          className="flex-none w-6 h-6 rounded border border-line bg-canvas text-fg text-[12px] leading-none cursor-pointer disabled:opacity-40 disabled:cursor-default"
          onClick={() => parent !== null && onNavigate(parent)}
          disabled={parent === null}
          aria-label="상위 폴더"
          title="상위 폴더"
        >
          ↑
        </button>
        <span className="flex-1 min-w-0 text-[13px] font-semibold overflow-hidden text-ellipsis whitespace-nowrap">{displayName}</span>
        <button
          className="flex-none w-6 h-6 rounded border border-line bg-canvas text-fg text-[12px] leading-none cursor-pointer"
          onClick={() => onNavigate(path)}
          aria-label="새로고침"
          title="새로고침"
        >
          ⟳
        </button>
        <button
          className="flex-none w-6 h-6 rounded border border-line bg-canvas text-fg text-[12px] leading-none cursor-pointer"
          onClick={() => void ops.homeDir().then(onNavigate)}
          aria-label="홈"
          title="홈"
        >
          ~
        </button>
        <button
          className="flex-none w-6 h-6 rounded border border-line bg-canvas text-fg text-[12px] leading-none cursor-pointer"
          onClick={onNewFolder}
          aria-label="새 폴더"
          title="새 폴더"
        >
          +
        </button>
        {onDisconnect && (
          <button
            className="flex-none w-6 h-6 rounded border border-line bg-canvas text-fg text-[12px] leading-none cursor-pointer"
            onClick={onDisconnect}
            aria-label="연결 해제"
            title="연결 해제"
          >
            ⏏
          </button>
        )}
      </div>
      <div className="px-2 pb-1.5">
        <input
          className="w-full px-1.5 py-1 rounded border border-line bg-canvas text-fg text-[11px]"
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onNavigate(pathInput)
          }}
        />
      </div>
    </div>
  )
}
