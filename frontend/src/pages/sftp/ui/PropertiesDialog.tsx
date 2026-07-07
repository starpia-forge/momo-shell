import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  return (
    <Dialog open={entry !== null} onClose={onClose} title={t('sftp.properties.title')}>
      {entry && (
        <div className="flex flex-col min-w-70 text-[13px]">
          <Row label={t('sftp.properties.name')} value={entry.name} />
          <Row label={t('sftp.properties.path')} value={entry.path} />
          <Row label={t('sftp.properties.kind')} value={entry.isDir ? t('sftp.properties.directory') : t('sftp.properties.file')} />
          <Row label={t('sftp.properties.size')} value={entry.isDir ? '—' : formatSize(entry.size)} />
          <Row label={t('sftp.properties.permissions')} value={`${entry.modeText} (${(entry.mode & 0o777).toString(8)})`} />
          <Row label={t('sftp.properties.modifiedAt')} value={formatModTime(entry.modTime)} />
        </div>
      )}
    </Dialog>
  )
}
