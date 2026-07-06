import { useState, type DragEvent, type MouseEvent } from 'react'
import { cn } from '../../../shared/lib/cn'
import { isFileDrag } from '../../../shared/lib/paneDnd'
import { formatModTime, formatSize, NamePromptDialog } from '../../../widgets/file-browser'
import { ContextMenu, Spinner, Toast, type ContextMenuItem } from '../../../shared/ui'
import type { RemoteEntry } from '../../../shared/api/transfer'
import { decodeSftpDrag, encodeSftpDrag, isSftpDrag } from '../lib/dnd'
import type { PaneOps } from '../model/paneOps'
import { useSftpStore, type PaneState } from '../model/store'
import { PaneHeader } from './PaneHeader'
import { PropertiesDialog } from './PropertiesDialog'

interface FilePaneProps {
  side: 'local' | 'remote'
  ops: PaneOps
  /** Remote pane only -- shown as the ⏏ disconnect button in the header. */
  onDisconnect?: () => void
  /** Notifies SftpPage that an OS file drag is currently hovering this pane,
   * so its os:filedrop handler (coordinates only, no DOM target) knows
   * which pane to drop into. */
  onFileDragHover?: () => void
}

interface MenuState {
  x: number
  y: number
  entry: RemoteEntry
}

const TRANSFER_LABEL: Record<'local' | 'remote', string> = { local: '업로드', remote: '다운로드' }

/** One SFTP pane (local or remote), driven entirely through PaneOps so the
 * same component serves both sides -- see model/paneOps.ts's rationale. */
export function FilePane({ side, ops, onDisconnect, onFileDragHover }: FilePaneProps) {
  const pane = useSftpStore((s) => s[side]) as PaneState
  const refresh = useSftpStore((s) => (side === 'local' ? s.refreshLocal : s.refreshRemote))
  const select = useSftpStore((s) => s.select)
  const clipboard = useSftpStore((s) => s.clipboard)
  const setClipboard = useSftpStore((s) => s.setClipboard)
  const transferSelection = useSftpStore((s) => s.transferSelection)
  const paste = useSftpStore((s) => s.paste)

  const [menu, setMenu] = useState<MenuState | null>(null)
  const [emptyMenu, setEmptyMenu] = useState<{ x: number; y: number } | null>(null)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<RemoteEntry | null>(null)
  const [propsEntry, setPropsEntry] = useState<RemoteEntry | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)

  // Home-directory resolution happens once via the page's initLocal() (local)
  // or connect() (remote) -- this pane only renders whatever path is current.
  const path = pane.path ?? ''

  function handleEntryDoubleClick(entry: RemoteEntry) {
    if (entry.isDir) void refresh(entry.path)
  }

  function handleNewFolder(name: string) {
    void ops
      .mkdir(ops.join(path, name))
      .then(() => refresh(path))
      .catch((err) => setToast(String(err)))
  }

  function handleRenameConfirm(name: string) {
    if (!renameTarget) return
    const newPath = ops.join(path, name)
    void ops
      .rename(renameTarget.path, newPath)
      .then(() => refresh(path))
      .catch((err) => setToast(String(err)))
  }

  function handleDelete(entry: RemoteEntry) {
    if (!window.confirm(`${entry.name} 삭제할까요?`)) return
    void ops
      .remove(entry.path)
      .then(() => refresh(path))
      .catch((err) => setToast(String(err)))
  }

  function openEntryMenu(e: MouseEvent, entry: RemoteEntry) {
    e.preventDefault()
    e.stopPropagation()
    select(side, entry.path)
    setMenu({ x: e.clientX, y: e.clientY, entry })
  }

  function openEmptyMenu(e: MouseEvent) {
    e.preventDefault()
    setEmptyMenu({ x: e.clientX, y: e.clientY })
  }

  const menuItems: ContextMenuItem[] = menu
    ? [
        { label: TRANSFER_LABEL[side], onClick: () => void transferSelection(side, [menu.entry.path]) },
        { label: '복사', onClick: () => setClipboard({ side, op: 'copy', paths: [menu.entry.path] }) },
        { label: '이동', onClick: () => setClipboard({ side, op: 'move', paths: [menu.entry.path] }) },
        ...(clipboard ? [{ label: '붙여넣기', onClick: () => void paste(side) }] : []),
        { label: '이름 변경', onClick: () => setRenameTarget(menu.entry) },
        { label: '속성', onClick: () => setPropsEntry(menu.entry) },
        { label: '삭제', danger: true, onClick: () => handleDelete(menu.entry) },
      ]
    : []

  const emptyMenuItems: ContextMenuItem[] = emptyMenu
    ? [
        ...(clipboard ? [{ label: '붙여넣기', onClick: () => void paste(side) }] : []),
        { label: '새 폴더', onClick: () => setMkdirOpen(true) },
      ]
    : []

  function handlePaneDragOver(e: DragEvent) {
    if (isSftpDrag(e.dataTransfer)) {
      e.preventDefault()
      setDragOver(true)
      return
    }
    if (isFileDrag(e.dataTransfer)) {
      setDragOver(true)
      onFileDragHover?.()
    }
  }

  function handlePaneDragLeave(e: DragEvent) {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragOver(false)
  }

  function handlePaneDrop(e: DragEvent) {
    e.preventDefault()
    setDragOver(false)
    if (isFileDrag(e.dataTransfer)) return // handled by SftpPage's os:filedrop subscription instead
    const payload = decodeSftpDrag(e.dataTransfer)
    if (!payload || payload.side === side) return // same-pane drop is a no-op in v1
    void transferSelection(payload.side, payload.paths)
  }

  return (
    <div
      className={cn(
        'flex-1 basis-0 min-w-0 min-h-0 flex flex-col text-[13px]',
        side === 'local' && 'border-r border-line',
        dragOver && 'outline outline-[1px] outline-accent outline-offset-[-1px] bg-accent/8'
      )}
      data-sftp-pane={side}
      onClick={() => select(side, null)}
      onContextMenu={openEmptyMenu}
      onDragOver={handlePaneDragOver}
      onDragLeave={handlePaneDragLeave}
      onDrop={handlePaneDrop}
    >
      <PaneHeader path={path} ops={ops} onNavigate={(p) => void refresh(p)} onNewFolder={() => setMkdirOpen(true)} onDisconnect={onDisconnect} />

      {pane.loading && (
        <div className="flex items-center gap-1.5 p-2 text-muted text-[12px]">
          <Spinner size={12} /> 불러오는 중...
        </div>
      )}
      {pane.error && <div className="p-2 text-danger text-[12px]">{pane.error}</div>}

      {!pane.loading && !pane.error && (
        <div className="flex-1 overflow-y-auto">
          {pane.entries.map((entry) => (
            <div
              key={entry.path}
              className={`flex items-center gap-1.5 px-2 py-1 cursor-default hover:bg-canvas ${pane.selected === entry.path ? 'bg-canvas' : ''}`}
              draggable
              onDragStart={(e) => encodeSftpDrag(e.dataTransfer, { side, paths: [entry.path] })}
              onClick={(e) => {
                e.stopPropagation()
                select(side, entry.path)
              }}
              onDoubleClick={() => handleEntryDoubleClick(entry)}
              onContextMenu={(e) => openEntryMenu(e, entry)}
            >
              <span className="flex-none text-[12px]">{entry.isDir ? '📁' : '📄'}</span>
              <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12px]" title={entry.name}>
                {entry.name}
              </span>
              <span className="flex-none w-14 text-right text-[11px] text-muted">{entry.isDir ? '—' : formatSize(entry.size)}</span>
              <span className="flex-none w-17 text-right text-[11px] text-muted">{formatModTime(entry.modTime)}</span>
            </div>
          ))}
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      {emptyMenu && <ContextMenu x={emptyMenu.x} y={emptyMenu.y} items={emptyMenuItems} onClose={() => setEmptyMenu(null)} />}
      <NamePromptDialog open={mkdirOpen} title="새 폴더" onConfirm={handleNewFolder} onClose={() => setMkdirOpen(false)} />
      <NamePromptDialog
        open={renameTarget !== null}
        title="이름 변경"
        initialValue={renameTarget?.name}
        onConfirm={handleRenameConfirm}
        onClose={() => setRenameTarget(null)}
      />
      <PropertiesDialog entry={propsEntry} onClose={() => setPropsEntry(null)} />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
