// Pure layout tree for one tab's split panes. xterm instances live outside
// React (entities/session/lib/terminal-registry) so moving a leaf around this
// tree is a DOM re-parent, not a session restart -- these functions only ever
// rearrange sessionId references, never touch sessions themselves.
//
// Every mutator re-normalizes the whole tree: same-direction nested splits
// are flattened into one split, and a split left with a single child is
// promoted to that child. Node ids are always supplied by the caller (never
// generated in here) so the module stays pure and deterministic for tests.

import type { DropZone } from '../../../shared/lib/paneDnd'

export type { DropZone }

export interface LeafNode {
  type: 'leaf'
  id: string
  sessionId: string
}

export interface SplitNode {
  type: 'split'
  id: string
  direction: SplitDirection
  sizes: number[]
  children: PaneNode[]
}

export type PaneNode = LeafNode | SplitNode
export type SplitDirection = 'row' | 'column'
export type ArrowDirection = 'up' | 'down' | 'left' | 'right'

export function createLeaf(id: string, sessionId: string): LeafNode {
  return { type: 'leaf', id, sessionId }
}

export function findLeaf(node: PaneNode, leafId: string): LeafNode | null {
  if (node.type === 'leaf') return node.id === leafId ? node : null
  for (const child of node.children) {
    const found = findLeaf(child, leafId)
    if (found) return found
  }
  return null
}

export function leaves(node: PaneNode): LeafNode[] {
  if (node.type === 'leaf') return [node]
  return node.children.flatMap(leaves)
}

function normalizeSizes(sizes: number[]): number[] {
  const total = sizes.reduce((a, b) => a + b, 0)
  if (total <= 0) return sizes.map(() => 1 / sizes.length)
  return sizes.map((s) => s / total)
}

function normalize(node: PaneNode): PaneNode {
  if (node.type === 'leaf') return node
  const normalizedChildren = node.children.map(normalize)

  const flatChildren: PaneNode[] = []
  const flatSizes: number[] = []
  normalizedChildren.forEach((child, i) => {
    if (child.type === 'split' && child.direction === node.direction) {
      const share = node.sizes[i]
      child.children.forEach((grandchild, j) => {
        flatChildren.push(grandchild)
        flatSizes.push(child.sizes[j] * share)
      })
    } else {
      flatChildren.push(child)
      flatSizes.push(node.sizes[i])
    }
  })

  if (flatChildren.length === 1) return flatChildren[0]
  return { ...node, children: flatChildren, sizes: normalizeSizes(flatSizes) }
}

/**
 * Splits `leafId` in the given direction, inserting `newLeaf` before or
 * after it. If the leaf's direct parent already splits along the same
 * direction, the new leaf is inserted as a sibling instead of nesting.
 * `splitId` is the id for a newly created split node (unused when inserting
 * as a sibling of an existing same-direction split).
 */
export function splitLeaf(
  tree: PaneNode,
  leafId: string,
  direction: SplitDirection,
  before: boolean,
  newLeaf: LeafNode,
  splitId: string
): PaneNode {
  function recur(node: PaneNode): PaneNode {
    if (node.type === 'leaf') {
      if (node.id !== leafId) return node
      const children = before ? [newLeaf, node] : [node, newLeaf]
      return { type: 'split', id: splitId, direction, sizes: [0.5, 0.5], children }
    }

    const idx = node.children.findIndex((c) => c.type === 'leaf' && c.id === leafId)
    if (idx !== -1 && node.direction === direction) {
      const children = [...node.children]
      const sizes = [...node.sizes]
      const shrink = sizes[idx] / 2
      sizes[idx] = shrink
      const insertAt = before ? idx : idx + 1
      children.splice(insertAt, 0, newLeaf)
      sizes.splice(insertAt, 0, shrink)
      return { ...node, children, sizes: normalizeSizes(sizes) }
    }

    return { ...node, children: node.children.map(recur) }
  }
  return normalize(recur(tree))
}

/** Removes `leafId` from the tree. Returns null if it was the only leaf. */
export function removeLeaf(tree: PaneNode, leafId: string): PaneNode | null {
  function recur(node: PaneNode): PaneNode | null {
    if (node.type === 'leaf') return node.id === leafId ? null : node

    const children: PaneNode[] = []
    const sizes: number[] = []
    node.children.forEach((child, i) => {
      const result = recur(child)
      if (result) {
        children.push(result)
        sizes.push(node.sizes[i])
      }
    })

    if (children.length === 0) return null
    if (children.length === 1) return children[0]
    return { ...node, children, sizes: normalizeSizes(sizes) }
  }

  const result = recur(tree)
  return result ? normalize(result) : null
}

/**
 * Moves `srcLeafId` to one of `targetLeafId`'s five drop zones. `center`
 * swaps the two leaves' sessions in place; the edge zones remove the source
 * leaf and re-split the target in the corresponding direction. `splitId` is
 * used only when an edge-zone move creates a new split node.
 */
export function moveLeaf(
  tree: PaneNode,
  srcLeafId: string,
  targetLeafId: string,
  zone: DropZone,
  splitId: string
): PaneNode {
  if (srcLeafId === targetLeafId) return tree
  const src = findLeaf(tree, srcLeafId)
  const target = findLeaf(tree, targetLeafId)
  if (!src || !target) return tree
  const srcSessionId = src.sessionId
  const targetSessionId = target.sessionId

  if (zone === 'center') {
    function swap(node: PaneNode): PaneNode {
      if (node.type === 'leaf') {
        if (node.id === srcLeafId) return { ...node, sessionId: targetSessionId }
        if (node.id === targetLeafId) return { ...node, sessionId: srcSessionId }
        return node
      }
      return { ...node, children: node.children.map(swap) }
    }
    return swap(tree)
  }

  const direction: SplitDirection = zone === 'left' || zone === 'right' ? 'row' : 'column'
  const before = zone === 'left' || zone === 'top'

  const withoutSrc = removeLeaf(tree, srcLeafId)
  if (!withoutSrc) return tree
  const movedLeaf = createLeaf(srcLeafId, src.sessionId)
  return splitLeaf(withoutSrc, targetLeafId, direction, before, movedLeaf, splitId)
}

/** Resets every child of the split `splitId` to an equal share. */
export function equalize(tree: PaneNode, splitId: string): PaneNode {
  function recur(node: PaneNode): PaneNode {
    if (node.type === 'leaf') return node
    const children = node.children.map(recur)
    if (node.id === splitId) return { ...node, children, sizes: children.map(() => 1 / children.length) }
    return { ...node, children }
  }
  return recur(tree)
}

/** Sets the split `splitId`'s child sizes (renormalized to sum to 1). */
export function setSizes(tree: PaneNode, splitId: string, sizes: number[]): PaneNode {
  function recur(node: PaneNode): PaneNode {
    if (node.type === 'leaf') return node
    const children = node.children.map(recur)
    if (node.id === splitId && sizes.length === node.children.length) {
      return { ...node, children, sizes: normalizeSizes(sizes) }
    }
    return { ...node, children }
  }
  return recur(tree)
}

interface Rect {
  x: number
  y: number
  w: number
  h: number
}

function computeRects(node: PaneNode, rect: Rect, out: Map<string, Rect>): void {
  if (node.type === 'leaf') {
    out.set(node.id, rect)
    return
  }
  let offset = 0
  node.children.forEach((child, i) => {
    const size = node.sizes[i]
    const childRect: Rect =
      node.direction === 'row'
        ? { x: rect.x + offset * rect.w, y: rect.y, w: size * rect.w, h: rect.h }
        : { x: rect.x, y: rect.y + offset * rect.h, w: rect.w, h: size * rect.h }
    computeRects(child, childRect, out)
    offset += size
  })
}

/**
 * Finds the leaf geometrically nearest to `leafId` in the given arrow
 * direction (for Alt+arrow focus movement), or null if none exists. Ties
 * broken by perpendicular-axis distance from center.
 */
export function neighborLeaf(tree: PaneNode, leafId: string, direction: ArrowDirection): string | null {
  const rects = new Map<string, Rect>()
  computeRects(tree, { x: 0, y: 0, w: 1, h: 1 }, rects)
  const from = rects.get(leafId)
  if (!from) return null

  const fromCenterX = from.x + from.w / 2
  const fromCenterY = from.y + from.h / 2
  const EPS = 1e-6

  let best: { id: string; dist: number } | null = null
  for (const [id, rect] of rects) {
    if (id === leafId) continue
    const centerX = rect.x + rect.w / 2
    const centerY = rect.y + rect.h / 2

    let inDirection = false
    let dist = 0
    switch (direction) {
      case 'left':
        inDirection = rect.x + rect.w <= from.x + EPS
        dist = from.x - (rect.x + rect.w) + Math.abs(centerY - fromCenterY)
        break
      case 'right':
        inDirection = rect.x >= from.x + from.w - EPS
        dist = rect.x - (from.x + from.w) + Math.abs(centerY - fromCenterY)
        break
      case 'up':
        inDirection = rect.y + rect.h <= from.y + EPS
        dist = from.y - (rect.y + rect.h) + Math.abs(centerX - fromCenterX)
        break
      case 'down':
        inDirection = rect.y >= from.y + from.h - EPS
        dist = rect.y - (from.y + from.h) + Math.abs(centerX - fromCenterX)
        break
    }
    if (!inDirection) continue
    if (!best || dist < best.dist) best = { id, dist }
  }
  return best?.id ?? null
}
