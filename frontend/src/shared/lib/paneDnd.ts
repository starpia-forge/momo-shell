// Drag/drop payload shared across workspace-layout (pane docking) and,
// later, tab-bar (drag-to-detach) -- both need the same MIME + codec.
export const PANE_MIME = 'application/x-momo-pane'

/**
 * Lives here (shared) rather than in workspace-layout's tree model so that
 * features/pane-dnd (a lower FSD layer than widgets) can reference it too,
 * without either side importing across the widgets/features boundary.
 */
export type DropZone = 'top' | 'bottom' | 'left' | 'right' | 'center'

export interface PaneDragPayload {
  tabId: string
  leafId: string
  sessionId: string
}

export function encodePaneDrag(dataTransfer: DataTransfer, payload: PaneDragPayload): void {
  dataTransfer.setData(PANE_MIME, JSON.stringify(payload))
  dataTransfer.effectAllowed = 'move'
}

/** Only valid during the `drop` event -- `getData` returns "" during dragover. */
export function decodePaneDrag(dataTransfer: DataTransfer): PaneDragPayload | null {
  const raw = dataTransfer.getData(PANE_MIME)
  if (!raw) return null
  try {
    return JSON.parse(raw) as PaneDragPayload
  } catch {
    return null
  }
}

/** Safe during dragover (unlike getData). Excludes OS file drops, reserved for Phase 4. */
export function isPaneDrag(dataTransfer: DataTransfer): boolean {
  return dataTransfer.types.includes(PANE_MIME) && !dataTransfer.types.includes('Files')
}
