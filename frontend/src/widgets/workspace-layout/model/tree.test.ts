import { describe, expect, it } from 'vitest'
import {
  createLeaf,
  equalize,
  findLeaf,
  leaves,
  moveLeaf,
  neighborLeaf,
  removeLeaf,
  setSizes,
  splitLeaf,
  type PaneNode,
} from './tree'

describe('splitLeaf', () => {
  it('splits a single leaf into a row with 50/50 sizes', () => {
    const tree = createLeaf('a', 'sess-a')
    const result = splitLeaf(tree, 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')

    expect(result).toEqual({
      type: 'split',
      id: 'split-1',
      direction: 'row',
      sizes: [0.5, 0.5],
      children: [createLeaf('a', 'sess-a'), createLeaf('b', 'sess-b')],
    })
  })

  it('inserts before when requested', () => {
    const tree = createLeaf('a', 'sess-a')
    const result = splitLeaf(tree, 'a', 'column', true, createLeaf('b', 'sess-b'), 'split-1') as PaneNode
    expect(result).toMatchObject({
      direction: 'column',
      children: [{ id: 'b' }, { id: 'a' }],
    })
  })

  it('inserts as a sibling instead of nesting when the parent already splits the same direction', () => {
    let tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    tree = splitLeaf(tree, 'b', 'row', false, createLeaf('c', 'sess-c'), 'split-2')

    expect(tree).toMatchObject({
      type: 'split',
      direction: 'row',
      children: [{ id: 'a' }, { id: 'b' }, { id: 'c' }],
    })
    const sizes = (tree as { sizes: number[] }).sizes
    expect(sizes).toHaveLength(3)
    expect(sizes.reduce((a, b) => a + b, 0)).toBeCloseTo(1)
  })

  it('flattens a same-direction split nested inside another (row-in-row)', () => {
    // Manually construct a nested row-in-row tree, then trigger normalization
    // via any mutator (splitLeaf on an unrelated leaf).
    const nested: PaneNode = {
      type: 'split',
      id: 'outer',
      direction: 'row',
      sizes: [0.5, 0.5],
      children: [
        createLeaf('a', 'sess-a'),
        {
          type: 'split',
          id: 'inner',
          direction: 'row',
          sizes: [0.5, 0.5],
          children: [createLeaf('b', 'sess-b'), createLeaf('c', 'sess-c')],
        },
      ],
    }

    // split an unrelated new leaf into 'a' in a different direction so the
    // row-in-row nesting above is untouched by the split itself and only
    // normalize() is responsible for flattening it.
    const result = splitLeaf(nested, 'a', 'column', false, createLeaf('d', 'sess-d'), 'split-3')

    // 'a' became a column split wrapping [a, d] (kept nested, different
    // direction); the sibling row-in-row [b, c] is same-direction as the
    // outer row and gets flattened into it directly, so the outer row ends
    // up with 3 children instead of a nested row split.
    expect(result).toMatchObject({ type: 'split', direction: 'row' })
    const root = result as PaneNode & { children: PaneNode[]; sizes: number[] }
    expect(root.children).toHaveLength(3)
    const nestedRowSplit = root.children.find((c) => c.type === 'split' && c.direction === 'row')
    expect(nestedRowSplit).toBeUndefined() // fully merged into the outer row, not left as a nested row
    expect(leaves(result).map((l) => l.id).sort()).toEqual(['a', 'b', 'c', 'd'])
    expect(root.sizes.reduce((a, b) => a + b, 0)).toBeCloseTo(1)
  })
})

describe('removeLeaf', () => {
  it('returns null when removing the only leaf', () => {
    expect(removeLeaf(createLeaf('a', 'sess-a'), 'a')).toBeNull()
  })

  it('promotes the remaining sibling when a 2-child split loses one child', () => {
    const tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    const result = removeLeaf(tree, 'b')
    expect(result).toEqual(createLeaf('a', 'sess-a'))
  })

  it('renormalizes sizes summing to 1 when a 3-child split loses one child', () => {
    let tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    tree = splitLeaf(tree, 'b', 'row', false, createLeaf('c', 'sess-c'), 'split-2')

    const result = removeLeaf(tree, 'b') as PaneNode & { sizes: number[]; children: PaneNode[] }
    expect(result.children).toHaveLength(2)
    expect(result.sizes.reduce((a, b) => a + b, 0)).toBeCloseTo(1)
  })
})

describe('moveLeaf', () => {
  const twoLeafTree = () => splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')

  it('is a no-op when moving a leaf onto itself', () => {
    const tree = twoLeafTree()
    expect(moveLeaf(tree, 'a', 'a', 'right', 'split-2')).toBe(tree)
  })

  it('swaps sessions in place for the center zone', () => {
    const tree = twoLeafTree()
    const result = moveLeaf(tree, 'a', 'b', 'center', 'split-2')
    expect(findLeaf(result, 'a')?.sessionId).toBe('sess-b')
    expect(findLeaf(result, 'b')?.sessionId).toBe('sess-a')
  })

  it('re-splits the target in the drop direction for edge zones', () => {
    const tree = twoLeafTree() // row: [a, b]
    const result = moveLeaf(tree, 'a', 'b', 'top', 'split-new')

    // 'a' should now sit above 'b' in a column split; the original row
    // split (which only had 'b' left) is promoted away entirely.
    expect(result).toMatchObject({ type: 'split', direction: 'column' })
    expect(leaves(result).map((l) => l.id).sort()).toEqual(['a', 'b'])
    expect(findLeaf(result, 'a')?.sessionId).toBe('sess-a')
  })
})

describe('equalize', () => {
  it('resets an unevenly-sized split to equal shares', () => {
    let tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    tree = setSizes(tree, 'split-1', [0.8, 0.2])
    tree = splitLeaf(tree, 'b', 'row', false, createLeaf('c', 'sess-c'), 'split-2')

    const result = equalize(tree, 'split-1') as PaneNode & { sizes: number[] }
    expect(result.sizes).toEqual([1 / 3, 1 / 3, 1 / 3])
  })
})

describe('setSizes', () => {
  it('normalizes the given sizes to sum to 1', () => {
    const tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    const result = setSizes(tree, 'split-1', [1, 3]) as PaneNode & { sizes: number[] }
    expect(result.sizes).toEqual([0.25, 0.75])
  })
})

describe('neighborLeaf', () => {
  // Build a 2x2 grid: a row split of two column splits.
  //   +---+---+
  //   | a | c |
  //   +---+---+
  //   | b | d |
  //   +---+---+
  const grid: PaneNode = {
    type: 'split',
    id: 'root',
    direction: 'row',
    sizes: [0.5, 0.5],
    children: [
      {
        type: 'split',
        id: 'left-col',
        direction: 'column',
        sizes: [0.5, 0.5],
        children: [createLeaf('a', 'sess-a'), createLeaf('b', 'sess-b')],
      },
      {
        type: 'split',
        id: 'right-col',
        direction: 'column',
        sizes: [0.5, 0.5],
        children: [createLeaf('c', 'sess-c'), createLeaf('d', 'sess-d')],
      },
    ],
  }

  it('finds the leaf to the right', () => {
    expect(neighborLeaf(grid, 'a', 'right')).toBe('c')
  })

  it('finds the leaf below', () => {
    expect(neighborLeaf(grid, 'a', 'down')).toBe('b')
  })

  it('finds the leaf to the left', () => {
    expect(neighborLeaf(grid, 'd', 'left')).toBe('b')
  })

  it('finds the leaf above', () => {
    expect(neighborLeaf(grid, 'd', 'up')).toBe('c')
  })

  it('returns null when there is no neighbor in that direction', () => {
    expect(neighborLeaf(grid, 'a', 'up')).toBeNull()
  })
})

describe('leaves', () => {
  it('flattens all leaves in depth order', () => {
    let tree = splitLeaf(createLeaf('a', 'sess-a'), 'a', 'row', false, createLeaf('b', 'sess-b'), 'split-1')
    tree = splitLeaf(tree, 'a', 'column', false, createLeaf('c', 'sess-c'), 'split-2')
    expect(leaves(tree).map((l) => l.id).sort()).toEqual(['a', 'b', 'c'])
  })
})
