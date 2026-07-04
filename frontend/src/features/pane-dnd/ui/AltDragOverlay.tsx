import type { PaneDragPayload } from '../../../shared/lib/paneDnd'
import { dragSourceProps } from '../lib/dragSource'
import { useAltHeld } from '../lib/useAltHeld'
import './AltDragOverlay.css'

interface AltDragOverlayProps {
  payload: PaneDragPayload
}

/** While Alt is held, covers the pane body as a drag source -- lets a pane be
 * dragged from its content area without stealing normal clicks/selection
 * from the terminal underneath the rest of the time. */
export function AltDragOverlay({ payload }: AltDragOverlayProps) {
  const altHeld = useAltHeld()
  if (!altHeld) return null
  return <div className="pane-dnd-alt-overlay" {...dragSourceProps(payload)} />
}
