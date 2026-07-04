import type { DragEvent } from 'react'
import { encodePaneDrag, type PaneDragPayload } from '../../../shared/lib/paneDnd'

/** Spread onto a pane's header (or an Alt-drag overlay) to make it a drag source. */
export function dragSourceProps(payload: PaneDragPayload) {
  return {
    draggable: true,
    onDragStart: (e: DragEvent) => encodePaneDrag(e.dataTransfer, payload),
  }
}
