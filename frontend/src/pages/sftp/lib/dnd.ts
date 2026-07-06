// Drag/drop payload for the SFTP page's two panes -- mirrors
// shared/lib/paneDnd.ts's encode/decode/guard pattern with its own MIME so
// the two drag systems (pane docking vs. SFTP file transfer) never collide.
export const SFTP_MIME = 'application/x-momo-sftp'

export interface SftpDragPayload {
  side: 'local' | 'remote'
  paths: string[]
}

export function encodeSftpDrag(dataTransfer: DataTransfer, payload: SftpDragPayload): void {
  dataTransfer.setData(SFTP_MIME, JSON.stringify(payload))
  dataTransfer.effectAllowed = 'copyMove'
}

/** Only valid during the `drop` event -- `getData` returns "" during dragover. */
export function decodeSftpDrag(dataTransfer: DataTransfer): SftpDragPayload | null {
  const raw = dataTransfer.getData(SFTP_MIME)
  if (!raw) return null
  try {
    return JSON.parse(raw) as SftpDragPayload
  } catch {
    return null
  }
}

/** Safe during dragover (unlike getData). Excludes OS file drops. */
export function isSftpDrag(dataTransfer: DataTransfer): boolean {
  return dataTransfer.types.includes(SFTP_MIME) && !dataTransfer.types.includes('Files')
}
