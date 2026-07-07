import { formatModTime, formatSize } from '../../../widgets/file-browser'
import { Dialog } from '../../../shared/ui'
import type { RemoteEntry } from '../../../shared/api/transfer'

interface PropertiesDialogProps {
  entry: RemoteEntry | null
  onClose: () => void
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4 py-1.5 border-b border-line last:border-b-0">
      <span className="text-fg2">{label}</span>
      <span className="text-right break-all">{value}</span>
    </div>
  )
}

/** Shared by both SFTP panes -- takes a plain RemoteEntry, agnostic to which
 * side (local/remote) it came from. */
export function PropertiesDialog({ entry, onClose }: PropertiesDialogProps) {
  return (
    <Dialog open={entry !== null} onClose={onClose} title="속성">
      {entry && (
        <div className="flex flex-col min-w-70 text-[13px]">
          <Row label="이름" value={entry.name} />
          <Row label="경로" value={entry.path} />
          <Row label="종류" value={entry.isDir ? '디렉터리' : '파일'} />
          <Row label="크기" value={entry.isDir ? '—' : formatSize(entry.size)} />
          <Row label="권한" value={`${entry.modeText} (${(entry.mode & 0o777).toString(8)})`} />
          <Row label="수정 시각" value={formatModTime(entry.modTime)} />
        </div>
      )}
    </Dialog>
  )
}
