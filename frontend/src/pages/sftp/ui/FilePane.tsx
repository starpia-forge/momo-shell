import { useState, type DragEvent, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { isFileDrag } from '../../../shared/lib/paneDnd'
import { formatModTime, formatSize, NamePromptDialog } from '../../../widgets/file-browser'
import { ContextMenu, Spinner, StatusDot, Toast, type ContextMenuItem } from '../../../shared/ui'
import type { RemoteEntry } from '../../../shared/api/transfer'
import { decodeSftpDrag, encodeSftpDrag, isSftpDrag } from '../lib/dnd'
import type { PaneOps } from '../model/paneOps'
import { useSftpStore, type PaneState } from '../model/store'
import { PaneHeader } from './PaneHeader'
import { PropertiesDialog } from './PropertiesDialog'

interface FilePaneProps {
  side: 'local' | 'remote'
  ops: PaneOps
  /** Remote pane only -- shown as the "연결 해제" action in the header. */
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

/** One SFTP pane (local or remote), driven entirely through PaneOps so the
 * same component serves both sides -- see model/paneOps.ts's rationale. */
export function FilePane({ side, ops, onDisconnect, onFileDragHover }: FilePaneProps) {
  const { t } = useTranslation()
  const TRANSFER_LABEL: Record<'local' | 'remote', string> = {
    local: t('sftp.filePane.upload'),
    remote: t('sftp.filePane.download'),
  }
  const pane = useSftpStore((s) => s[side]) as PaneState
  const hostLabel = useSftpStore((s) => s.hostLabel)
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
    if (!window.confirm(t('sftp.filePane.confirmDelete', { name: entry.name }))) return
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
        { label: t('common.copy'), onClick: () => setClipboard({ side, op: 'copy', paths: [menu.entry.path] }) },
        { label: t('common.move'), onClick: () => setClipboard({ side, op: 'move', paths: [menu.entry.path] }) },
        ...(clipboard ? [{ label: t('common.paste'), onClick: () => void paste(side) }] : []),
        { label: t('common.rename'), onClick: () => setRenameTarget(menu.entry) },
        { label: t('sftp.properties.title'), onClick: () => setPropsEntry(menu.entry) },
        { label: t('common.delete'), danger: true, divider: true, onClick: () => handleDelete(menu.entry) },
      ]
    : []

  const emptyMenuItems: ContextMenuItem[] = emptyMenu
    ? [
        ...(clipboard ? [{ label: t('common.paste'), onClick: () => void paste(side) }] : []),
        { label: t('common.newFolder'), onClick: () => setMkdirOpen(true) },
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

  const selectedCount = pane.selected ? 1 : 0
  const selectedEntry = pane.selected ? pane.entries.find((e) => e.path === pane.selected) : undefined

  return (
    <div
      className={cn(
        'flex-1 basis-0 min-w-0 min-h-0 flex flex-col text-[13px] rounded-xl bg-surface border overflow-hidden',
        dragOver ? 'border-[1.5px] border-dashed border-accent bg-accent/8' : 'border-line',
      )}
      data-sftp-pane={side}
      onClick={() => select(side, null)}
      onContextMenu={openEmptyMenu}
      onDragOver={handlePaneDragOver}
      onDragLeave={handlePaneDragLeave}
      onDrop={handlePaneDrop}
    >
      <PaneHeader
        title={side === 'local' ? t('sftp.filePane.local') : t('sftp.filePane.remote')}
        subtitle={
          side === 'local' ? (
            <span className="text-[11.5px] text-fg3">{t('sftp.filePane.thisComputer')}</span>
          ) : (
            <span className="flex items-center gap-1.5 text-[11.5px] text-green">
              <StatusDot status="running" />
              {hostLabel}
            </span>
          )
        }
        path={path}
        ops={ops}
        onNavigate={(p) => void refresh(p)}
        onNewFolder={() => setMkdirOpen(true)}
        onDisconnect={onDisconnect}
      />

      <div className="flex px-4 pt-2 pb-1.5 text-[11px] text-fg3 gap-3">
        <span className="flex-1">{t('sftp.filePane.colName')}</span>
        <span className="w-19 text-right">{t('sftp.filePane.colSize')}</span>
        <span className="w-24 text-right">{t('sftp.filePane.colModified')}</span>
        <span className="w-20.5 text-right font-mono">{t('sftp.filePane.colPermissions')}</span>
      </div>

      {pane.loading && (
        <div className="flex items-center gap-1.5 px-4 pb-2 text-fg2 text-[12px]">
          <Spinner size={12} /> {t('sftp.filePane.loading')}
        </div>
      )}
      {pane.error && <div className="px-4 pb-2 text-red text-[12px]">{pane.error}</div>}

      {!pane.loading && !pane.error && (
        <div className="flex-1 overflow-y-auto px-2">
          {pane.entries.map((entry) => (
            <div
              key={entry.path}
              className={cn(
                'flex items-center gap-3 px-2.5 py-2.25 rounded-md cursor-default',
                pane.selected === entry.path ? 'bg-accent/13 border border-accent/40' : 'border border-transparent hover:bg-surface2',
              )}
              draggable
              onDragStart={(e) => encodeSftpDrag(e.dataTransfer, { side, paths: [entry.path] })}
              onClick={(e) => {
                e.stopPropagation()
                select(side, entry.path)
              }}
              onDoubleClick={() => handleEntryDoubleClick(entry)}
              onContextMenu={(e) => openEntryMenu(e, entry)}
            >
              <span className={cn('flex-none rounded-[3px]', entry.isDir ? 'w-3 h-2.5 bg-blue' : 'w-3 h-3 bg-surface2')} />
              <span className={cn('flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13px]', entry.isDir && 'font-medium')} title={entry.name}>
                {entry.name}
              </span>
              <span className="flex-none w-19 text-right text-[12px] text-fg3">{entry.isDir ? '—' : formatSize(entry.size)}</span>
              <span className="flex-none w-24 text-right text-[12px] text-fg3">{formatModTime(entry.modTime)}</span>
              <span className="flex-none w-20.5 text-right text-[11px] text-fg3 font-mono">{entry.modeText}</span>
            </div>
          ))}
        </div>
      )}

      <div className="flex-none h-8.5 flex items-center px-4 border-t border-line text-[11.5px] text-fg3">
        {t('sftp.filePane.itemCount', { count: pane.entries.length })}
        {selectedCount > 0 &&
          selectedEntry &&
          !selectedEntry.isDir &&
          ` · ${t('sftp.filePane.selectedCount', { count: selectedCount, size: formatSize(selectedEntry.size) })}`}
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      {emptyMenu && <ContextMenu x={emptyMenu.x} y={emptyMenu.y} items={emptyMenuItems} onClose={() => setEmptyMenu(null)} />}
      <NamePromptDialog open={mkdirOpen} title={t('common.newFolder')} onConfirm={handleNewFolder} onClose={() => setMkdirOpen(false)} />
      <NamePromptDialog
        open={renameTarget !== null}
        title={t('common.rename')}
        initialValue={renameTarget?.name}
        onConfirm={handleRenameConfirm}
        onClose={() => setRenameTarget(null)}
      />
      <PropertiesDialog entry={propsEntry} onClose={() => setPropsEntry(null)} />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
