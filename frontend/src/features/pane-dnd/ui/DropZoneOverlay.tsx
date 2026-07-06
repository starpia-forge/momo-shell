import { cn } from '../../../shared/lib/cn'
import type { DropZone } from '../../../shared/lib/paneDnd'

interface DropZoneOverlayProps {
  zone: DropZone | null
}

const ZONE: Record<DropZone, string> = {
  left: 'right-1/2',
  right: 'left-1/2',
  top: 'bottom-1/2',
  bottom: 'top-1/2',
  center: '',
}

/** Translucent placement preview shown over a pane while a drag hovers it. */
export function DropZoneOverlay({ zone }: DropZoneOverlayProps) {
  if (!zone) return null
  return <div className={cn('absolute inset-0 bg-accent opacity-25 pointer-events-none z-20', ZONE[zone])} />
}
