import type { DropZone } from '../../../shared/lib/paneDnd'
import './DropZoneOverlay.css'

interface DropZoneOverlayProps {
  zone: DropZone | null
}

/** Translucent placement preview shown over a pane while a drag hovers it. */
export function DropZoneOverlay({ zone }: DropZoneOverlayProps) {
  if (!zone) return null
  return <div className={`pane-dnd-overlay pane-dnd-overlay--${zone}`} />
}
