import { describe, expect, it } from 'vitest'
import { computeDropZone } from './dropZone'

describe('computeDropZone', () => {
  it('returns center for the middle of the pane', () => {
    expect(computeDropZone(0.5, 0.5)).toBe('center')
  })

  it('returns left/right for the outer 25% horizontal bands', () => {
    expect(computeDropZone(0.1, 0.5)).toBe('left')
    expect(computeDropZone(0.9, 0.5)).toBe('right')
  })

  it('returns top/bottom for the outer 25% vertical bands', () => {
    expect(computeDropZone(0.5, 0.1)).toBe('top')
    expect(computeDropZone(0.5, 0.9)).toBe('bottom')
  })

  it('prioritizes left/right over top/bottom in a corner', () => {
    expect(computeDropZone(0.1, 0.1)).toBe('left')
    expect(computeDropZone(0.9, 0.9)).toBe('right')
  })

  it('treats the exact 25%/75% boundary as still center', () => {
    expect(computeDropZone(0.25, 0.5)).toBe('center')
    expect(computeDropZone(0.75, 0.5)).toBe('center')
  })
})
