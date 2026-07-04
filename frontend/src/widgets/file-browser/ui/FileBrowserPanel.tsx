import { useEffect, useState, type MouseEvent } from 'react'
import {
  browseForDownloadDirectory,
  browseForUploadFiles,
  chmodRemote,
  downloadFiles,
  homeDir,
  listRemoteDir,
  mkdirRemote,
  removeRemote,
  renameRemote,
  subscribe,
  topics,
  uploadFiles,
  type RemoteEntry,
  type TransferTaskPayload,
} from '../../../shared/api'
import { ContextMenu, Spinner, Toast, type ContextMenuItem } from '../../../shared/ui'
import { formatModTime, formatSize, joinRemotePath, parentRemotePath } from '../lib/format'
import { EMPTY_BROWSE_STATE, sessionBrowseState, useFileBrowserStore } from '../model/store'
import { NamePromptDialog } from './NamePromptDialog'
import './FileBrowserPanel.css'

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
  const state = useFileBrowserStore((s) => (sessionId ? (s.bySession[sessionId] ?? EMPTY_BROWSE_STATE) : EMPTY_BROWSE_STATE))
  const [pathInput, setPathInput] = useState('')
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<RemoteEntry | null>(null)
  const [toast, setToast] = useState<string | null>(null)

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

  if (!sessionId) {
    return <div className="file-browser file-browser--empty">세션이 없습니다</div>
  }
  if (!isSSH) {
    return <div className="file-browser file-browser--empty">로컬 세션에서는 파일 브라우저를 사용할 수 없습니다</div>
  }

  const currentPath = state.path ?? '/'

  function handleUpload() {
    void browseForUploadFiles().then((paths) => {
      if (paths.length === 0 || !sessionId) return
      void uploadFiles(sessionId, paths, currentPath).catch((err) => setToast(String(err)))
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
    if (!window.confirm(`${entry.name} 삭제할까요?`)) return
    void removeRemote(sessionId, entry.path)
      .then(() => refresh(currentPath))
      .catch((err) => setToast(String(err)))
  }

  function handleChmod(entry: RemoteEntry) {
    if (!sessionId) return
    const input = window.prompt(`${entry.name}의 권한 (예: 755)`, (entry.mode & 0o777).toString(8))
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

  const menuItems: ContextMenuItem[] = menu
    ? [
        ...(menu.entry.isDir ? [] : [{ label: '다운로드', onClick: () => handleDownload(menu.entry) }]),
        { label: '이름 변경', onClick: () => setRenameTarget(menu.entry) },
        { label: '권한 변경', onClick: () => handleChmod(menu.entry) },
        { label: '삭제', danger: true, onClick: () => handleDelete(menu.entry) },
      ]
    : []

  return (
    <div className="file-browser">
      <div className="file-browser__toolbar">
        <button
          className="file-browser__tool"
          onClick={() => void refresh(parentRemotePath(currentPath))}
          disabled={currentPath === '/'}
          aria-label="뒤로"
          title="뒤로"
        >
          ←
        </button>
        <button className="file-browser__tool" onClick={() => void homeDir(sessionId).then(refresh)} aria-label="홈" title="홈">
          ~
        </button>
        <input
          className="file-browser__path"
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void refresh(pathInput)
          }}
        />
        <button className="file-browser__tool" onClick={() => void refresh(currentPath)} aria-label="새로고침" title="새로고침">
          ⟳
        </button>
        <button className="file-browser__tool" onClick={handleUpload} aria-label="업로드" title="업로드">
          ⬆
        </button>
        <button className="file-browser__tool" onClick={() => setMkdirOpen(true)} aria-label="새 폴더" title="새 폴더">
          +
        </button>
      </div>

      {state.loading && (
        <div className="file-browser__status">
          <Spinner size={12} /> 불러오는 중...
        </div>
      )}
      {state.error && <div className="file-browser__status file-browser__status--error">{state.error}</div>}

      {!state.loading && !state.error && (
        <div className="file-browser__list">
          {state.entries.map((entry) => (
            <div
              key={entry.path}
              className="file-browser__row"
              onDoubleClick={() => handleEntryDoubleClick(entry)}
              onContextMenu={(e) => openMenu(e, entry)}
            >
              <span className="file-browser__icon">{entry.isDir ? '📁' : '📄'}</span>
              <span className="file-browser__name" title={entry.name}>
                {entry.name}
              </span>
              <span className="file-browser__size">{entry.isDir ? '—' : formatSize(entry.size)}</span>
              <span className="file-browser__date">{formatModTime(entry.modTime)}</span>
              <span className="file-browser__mode">{entry.modeText}</span>
            </div>
          ))}
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      <NamePromptDialog open={mkdirOpen} title="새 폴더" onConfirm={handleMkdirConfirm} onClose={() => setMkdirOpen(false)} />
      <NamePromptDialog
        open={renameTarget !== null}
        title="이름 변경"
        initialValue={renameTarget?.name}
        onConfirm={handleRenameConfirm}
        onClose={() => setRenameTarget(null)}
      />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  )
}
