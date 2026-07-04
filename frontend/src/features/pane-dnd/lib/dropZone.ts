import type { DropZone } from '../../../shared/lib/paneDnd'

const EDGE_BAND = 0.25

/** relX/relY are 0..1 positions within the target pane's bounding rect. */
export function computeDropZone(relX: number, relY: number): DropZone {
  if (relX < EDGE_BAND) return 'left'
  if (relX > 1 - EDGE_BAND) return 'right'
  if (relY < EDGE_BAND) return 'top'
  if (relY > 1 - EDGE_BAND) return 'bottom'
  return 'center'
}
