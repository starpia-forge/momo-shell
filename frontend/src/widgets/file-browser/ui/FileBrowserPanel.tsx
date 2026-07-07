import { useEffect, useState, type DragEvent, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  browseForDownloadDirectory,
  browseForUploadFiles,
  chmodRemote,
  detectUploadConflicts,
  downloadFiles,
  homeDir,
  listRemoteDir,
  mkdirRemote,
  removeRemote,
  renameRemote,
  subscribe,
  topics,
  uploadFiles,
  type ConflictPolicy,
  type FileDropPayload,
  type RemoteEntry,
  type TransferTaskPayload,
} from '../../../shared/api'
import { markDropTargetHovered, resolveDropTarget } from '../../../shared/lib/fileDropTarget'
import { isFileDrag } from '../../../shared/lib/paneDnd'
import { cn } from '../../../shared/lib/cn'
import { ConflictDialog, ContextMenu, IconButton, Spinner, Toast, type ContextMenuItem } from '../../../shared/ui'
import { formatModTime, formatSize, joinRemotePath, parentRemotePath } from '../lib/format'
import { EMPTY_BROWSE_STATE, sessionBrowseState, useFileBrowserStore } from '../model/store'
import { NamePromptDialog } from './NamePromptDialog'

interface FileBrowserPanelProps {
  /** The currently-focused pane's session, or null if there is none. */
  sessionId: string | null
  /** SFTP only applies to SSH sessions -- local panes show a disabled state. */
  isSSH: boolean
}

interface MenuState {
  x: number
  y: number
  entry: RemoteEntry
}

export function FileBrowserPanel({ sessionId, isSSH }: FileBrowserPanelProps) {
  const { t } = useTranslation()
  const state = useFileBrowserStore((s) => (sessionId ? (s.bySession[sessionId] ?? EMPTY_BROWSE_STATE) : EMPTY_BROWSE_STATE))
  const [pathInput, setPathInput] = useState('')
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<RemoteEntry | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [fileDragOver, setFileDragOver] = useState(false)
  const [conflict, setConflict] = useState<{ paths: string[]; dir: string; names: string[] } | null>(null)

  async function refresh(path: string) {
    if (!sessionId) return
    useFileBrowserStore.getState().setLoading(sessionId, true)
    try {
      const entries = await listRemoteDir(sessionId, path)
      entries.sort((a, b) => (a.isDir === b.isDir ? a.name.localeCompare(b.name) : a.isDir ? -1 : 1))
      useFileBrowserStore.getState().setPath(sessionId, path)
      useFileBrowserStore.getState().setEntries(sessionId, entries)
    } catch (err) {
      useFileBrowserStore.getState().setError(sessionId, String(err))
    }
  }

  // Resolve the home directory once per session, the first time it's browsed.
  useEffect(() => {
    if (!sessionId || !isSSH) return
    if (sessionBrowseState(sessionId).path !== null) return
    void homeDir(sessionId)
      .then((home) => refresh(home))
      .catch((err) => useFileBrowserStore.getState().setError(sessionId, String(err)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, isSSH])

  useEffect(() => {
    setPathInput(state.path ?? '')
  }, [state.path])

  // A completed upload into this session invalidates whatever directory is
  // currently on screen -- simplest correct behavior without tracking each
  // task's destination separately from where the user has since navigated.
  useEffect(() => {
    if (!sessionId) return
    return subscribe<TransferTaskPayload>(topics.transferTask(), (task) => {
      if (task.sessionId !== sessionId || task.kind !== 'upload' || task.state !== 'done') return
      const current = sessionBrowseState(sessionId).path
      if (current) void refresh(current)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId])

  // Owns its own OS file-drop handling rather than routing through
  // features/file-upload (a lower FSD layer that can't import this widget):
  // an OS drop resolves to this panel via the shared hover/point resolver,
  // then uploads straight into whatever directory is currently browsed.
  useEffect(() => {
    if (!sessionId) return
    return subscribe<FileDropPayload>(topics.osFileDrop(), (payload) => {
      const target = resolveDropTarget(payload.x, payload.y)
      if (!target || !target.isFileBrowser || target.sessionId !== sessionId) return
      const current = sessionBrowseState(sessionId).path
      if (!current) return
      void uploadWithConflictCheck(sessionId, payload.paths, current)
    })
  }, [sessionId])

  if (!sessionId) {
    return <div className="flex-1 flex items-center justify-center p-4 text-center text-fg2 text-[12px]">{t('fileBrowser.noSession')}</div>
  }
  if (!isSSH) {
    return (
      <div className="flex-1 flex items-center justify-center p-4 text-center text-fg2 text-[12px]">
        {t('fileBrowser.localSessionUnsupported')}
      </div>
    )
  }

  const currentPath = state.path ?? '/'

  // Detects name collisions before uploading and, if any exist, prompts for
  // an overwrite/rename/skip policy instead of silently overwriting (the
  // backend honors whatever single policy the caller picks up front).
  async function uploadWithConflictCheck(targetSessionId: string, paths: string[], dir: string) {
    try {
      const names = await detectUploadConflicts(targetSessionId, paths, dir)
      if (names.length > 0) {
        setConflict({ paths, dir, names })
        return
      }
      await uploadFiles(targetSessionId, paths, dir)
    } catch (err) {
      setToast(String(err))
    }
  }

  function handleConflictChoice(policy: ConflictPolicy) {
    if (!sessionId || !conflict) return
    void uploadFiles(sessionId, conflict.paths, conflict.dir, policy).catch((err) => setToast(String(err)))
  }

  function handleUpload() {
    void browseForUploadFiles().then((paths) => {
      if (paths.length === 0 || !sessionId) return
      void uploadWithConflictCheck(sessionId, paths, currentPath)
    })
  }

  function handleEntryDoubleClick(entry: RemoteEntry) {
    if (entry.isDir) {
      void refresh(entry.path)
      return
    }
    handleDownload(entry)
  }

  function handleDownload(entry: RemoteEntry) {
    void browseForDownloadDirectory().then((dir) => {
      if (!dir || !sessionId) return
      void downloadFiles(sessionId, [entry.path], dir).catch((err) => setToast(String(err)))
    })
  }

  function handleMkdirConfirm(name: string) {
    if (!sessionId) return
    void mkdirRemote(sessionId, joinRemotePath(currentPath, name))
      .then(() => refresh(currentPath))
      .catch((err) => setToast(String(err)))
  }

  function handleRenameConfirm(name: string) {
    if (!sessionId || !renameTarget) return
    const newPath = joinRemotePath(parentRemotePath(renameTarget.path), name)
    void renameRemote(sessionId, renameTarget.path, newPath)
      .then(() => refresh(currentPath))
      .catch((err) => setToast(String(err)))
  }

  function handleDelete(entry: RemoteEntry) {
    if (!sessionId) return
    if (!window.confirm(t('sftp.filePane.confirmDelete', { name: entry.name }))) return
    void removeRemote(sessionId, entry.path)
      .then(() => refresh(currentPath))
      .catch((err) => setToast(String(err)))
  }

  function handleChmod(entry: RemoteEntry) {
    if (!sessionId) return
    const input = window.prompt(t('fileBrowser.chmodPrompt', { name: entry.name }), (entry.mode & 0o777).toString(8))
    if (!input) return
    const mode = parseInt(input, 8)
    if (Number.isNaN(mode)) return
    void chmodRemote(sessionId, entry.path, mode)
      .then(() => refresh(currentPath))
      .catch((err) => setToast(String(err)))
  }

  function openMenu(e: MouseEvent, entry: RemoteEntry) {
    e.preventDefault()
    setMenu({ x: e.clientX, y: e.clientY, entry })
  }

  function handlePanelDragOver(e: DragEvent) {
    if (!sessionId || !isFileDrag(e.dataTransfer)) return
    e.preventDefault()
    setFileDragOver(true)
    markDropTargetHovered(sessionId, true)
  }

  function handlePanelDragLeave(e: DragEvent) {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFileDragOver(false)
  }

  const menuItems: ContextMenuItem[] = menu
    ? [
        ...(menu.entry.isDir ? [] : [{ label: t('common.download'), onClick: () => handleDownload(menu.entry) }]),
        { label: t('common.rename'), onClick: () => setRenameTarget(menu.entry) },
        { label: t('fileBrowser.changePermissions'), onClick: () => handleChmod(menu.entry) },
        { label: t('common.delete'), danger: true, onClick: () => handleDelete(menu.entry) },
      ]
    : []

  return (
    <div
      className={cn(
        'file-browser [--wails-drop-target:drop] flex flex-col flex-1 min-h-0 text-[13px]',
        fileDragOver && 'outline outline-[1px] outline-accent outline-offset-[-1px] bg-accent/8',
      )}
      data-session-id={sessionId}
      data-filebrowser="true"
      onDragOver={handlePanelDragOver}
      onDragLeave={handlePanelDragLeave}
      onDrop={(e) => {
        e.preventDefault()
        setFileDragOver(false)
      }}
    >
      <div className="flex items-center gap-1.5 p-3.5">
        <IconButton
          size={26}
          className="text-[12px]"
          onClick={() => void refresh(parentRemotePath(currentPath))}
          disabled={currentPath === '/'}
          aria-label={t('fileBrowser.back')}
          title={t('fileBrowser.back')}
        >
          ←
        </IconButton>
        <IconButton size={26} className="text-[12px]" onClick={() => void homeDir(sessionId).then(refresh)} aria-label={t('common.home')} title={t('common.home')}>
          ~
        </IconButton>
        <input
          className="flex-1 min-w-0 px-2.5 py-1.5 rounded-md border border-line bg-inputbg text-fg font-mono text-[12px] focus:outline-none focus:border-accent"
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void refresh(pathInput)
          }}
        />
        <IconButton size={26} className="text-[12px]" onClick={() => void refresh(currentPath)} aria-label={t('common.refresh')} title={t('common.refresh')}>
          ⟳
        </IconButton>
        <IconButton size={26} className="text-[12px]" onClick={handleUpload} aria-label={t('common.upload')} title={t('common.upload')}>
          ⬆
        </IconButton>
        <IconButton size={26} className="text-[16px]" onClick={() => setMkdirOpen(true)} aria-label={t('common.newFolder')} title={t('common.newFolder')}>
          +
        </IconButton>
      </div>

      {state.loading && (
        <div className="flex items-center gap-1.5 px-3.5 pb-2 text-fg2 text-[12px]">
          <Spinner size={12} /> {t('common.loading')}
        </div>
      )}
      {state.error && <div className="px-3.5 pb-2 text-red text-[12px]">{state.error}</div>}

      {!state.loading && !state.error && (
        <div className="flex-1 overflow-y-auto px-2">
          {state.entries.map((entry) => (
            <div
              key={entry.path}
              className="flex items-center gap-2 px-2.5 py-1.75 rounded-md cursor-default hover:bg-surface2"
              onDoubleClick={() => handleEntryDoubleClick(entry)}
              onContextMenu={(e) => openMenu(e, entry)}
            >
              <span className={cn('flex-none w-2.5 h-2.5 rounded-[3px]', entry.isDir ? 'bg-blue' : 'bg-surface2')} />
              <span
                className={cn(
                  'flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px]',
                  entry.isDir && 'font-medium',
                )}
                title={entry.name}
              >
                {entry.name}
              </span>
              <span className="flex-none w-14 text-right text-[11px] text-fg3">{entry.isDir ? '—' : formatSize(entry.size)}</span>
              <span className="flex-none w-17 text-right text-[11px] text-fg3">{formatModTime(entry.modTime)}</span>
              <span className="flex-none w-19 text-right text-[10px] text-fg3 font-mono">{entry.modeText}</span>
            </div>
          ))}
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      <NamePromptDialog open={mkdirOpen} title={t('common.newFolder')} onConfirm={handleMkdirConfirm} onClose={() => setMkdirOpen(false)} />
      <NamePromptDialog
        open={renameTarget !== null}
        title={t('common.rename')}
        initialValue={renameTarget?.name}
        onConfirm={handleRenameConfirm}
        onClose={() => setRenameTarget(null)}
      />
      <ConflictDialog
        open={conflict !== null}
        conflicts={conflict?.names ?? []}
        onChoice={handleConflictChoice}
        onClose={() => setConflict(null)}
      />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
