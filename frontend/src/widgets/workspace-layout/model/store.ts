import { create } from 'zustand'
import {
  createLeaf,
  equalize as treeEqualize,
  findLeaf,
  leaves,
  neighborLeaf,
  removeLeaf,
  setSizes as treeSetSizes,
  splitLeaf as treeSplitLeaf,
  type ArrowDirection,
  type PaneNode,
  type SplitDirection,
} from './tree'

interface CloseLeafResult {
  sessionId: string
  becameEmpty: boolean
}

interface WorkspaceLayoutStore {
  /** One layout tree per tab, keyed by Tab.id. */
  trees: Record<string, PaneNode>
  /** The currently-focused leaf id per tab, keyed by Tab.id. */
  focusedLeaf: Record<string, string>
  /** Bumped per split node id on equalize, so its PanelGroup remounts to re-apply sizes. */
  splitVersion: Record<string, number>
  ensureTree: (tabId: string, sessionId: string, leafId: string) => void
  removeTree: (tabId: string) => void
  closeLeaf: (tabId: string, leafId: string) => CloseLeafResult | null
  replaceLeafSession: (tabId: string, leafId: string, sessionId: string) => void
  setSizes: (tabId: string, splitId: string, sizes: number[]) => void
  setFocus: (tabId: string, leafId: string) => void
  moveFocus: (tabId: string, direction: ArrowDirection) => void
  splitLeaf: (tabId: string, leafId: string, direction: SplitDirection, newLeafId: string, newSessionId: string, splitId: string) => void
  equalize: (tabId: string, splitId: string) => void
}

function omit<T>(record: Record<string, T>, key: string): Record<string, T> {
  return Object.fromEntries(Object.entries(record).filter(([k]) => k !== key))
}

export const useWorkspaceLayoutStore = create<WorkspaceLayoutStore>((set, get) => ({
  trees: {},
  focusedLeaf: {},
  splitVersion: {},

  ensureTree: (tabId, sessionId, leafId) =>
    set((s) => {
      if (s.trees[tabId]) return s
      return {
        trees: { ...s.trees, [tabId]: createLeaf(leafId, sessionId) },
        focusedLeaf: { ...s.focusedLeaf, [tabId]: leafId },
      }
    }),

  removeTree: (tabId) =>
    set((s) => ({
      trees: omit(s.trees, tabId),
      focusedLeaf: omit(s.focusedLeaf, tabId),
    })),

  closeLeaf: (tabId, leafId) => {
    let result: CloseLeafResult | null = null
    set((s) => {
      const tree = s.trees[tabId]
      if (!tree) return s
      const leaf = findLeaf(tree, leafId)
      if (!leaf) return s

      const newTree = removeLeaf(tree, leafId)
      result = { sessionId: leaf.sessionId, becameEmpty: newTree === null }

      if (newTree === null) {
        return { trees: omit(s.trees, tabId), focusedLeaf: omit(s.focusedLeaf, tabId) }
      }

      const focusedLeaf = { ...s.focusedLeaf }
      if (focusedLeaf[tabId] === leafId) {
        focusedLeaf[tabId] = leaves(newTree)[0]?.id ?? leafId
      }
      return { trees: { ...s.trees, [tabId]: newTree }, focusedLeaf }
    })
    return result
  },

  replaceLeafSession: (tabId, leafId, sessionId) =>
    set((s) => {
      const tree = s.trees[tabId]
      if (!tree) return s
      function recur(node: PaneNode): PaneNode {
        if (node.type === 'leaf') return node.id === leafId ? { ...node, sessionId } : node
        return { ...node, children: node.children.map(recur) }
      }
      return { trees: { ...s.trees, [tabId]: recur(tree) } }
    }),

  setSizes: (tabId, splitId, sizes) =>
    set((s) => {
      const tree = s.trees[tabId]
      if (!tree) return s
      return { trees: { ...s.trees, [tabId]: treeSetSizes(tree, splitId, sizes) } }
    }),

  setFocus: (tabId, leafId) => set((s) => ({ focusedLeaf: { ...s.focusedLeaf, [tabId]: leafId } })),

  moveFocus: (tabId, direction) => {
    const s = get()
    const tree = s.trees[tabId]
    const focusedLeafId = s.focusedLeaf[tabId]
    if (!tree || !focusedLeafId) return
    const nextId = neighborLeaf(tree, focusedLeafId, direction)
    if (nextId) s.setFocus(tabId, nextId)
  },

  splitLeaf: (tabId, leafId, direction, newLeafId, newSessionId, splitId) =>
    set((s) => {
      const tree = s.trees[tabId]
      if (!tree) return s
      const newLeaf = createLeaf(newLeafId, newSessionId)
      const newTree = treeSplitLeaf(tree, leafId, direction, false, newLeaf, splitId)
      return {
        trees: { ...s.trees, [tabId]: newTree },
        focusedLeaf: { ...s.focusedLeaf, [tabId]: newLeafId },
      }
    }),

  equalize: (tabId, splitId) =>
    set((s) => {
      const tree = s.trees[tabId]
      if (!tree) return s
      return {
        trees: { ...s.trees, [tabId]: treeEqualize(tree, splitId) },
        splitVersion: { ...s.splitVersion, [splitId]: (s.splitVersion[splitId] ?? 0) + 1 },
      }
    }),
}))
