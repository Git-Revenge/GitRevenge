import type { Ref } from './graph/rowCache'

/**
 * Turns refs into the rows each sidebar panel shows. The tree is built once per ref list, and
 * rows are flattened from it per view, so opening a folder or typing in the filter doesn't
 * rebuild anything.
 */

export type PanelKey = 'branches' | 'remotes' | 'tags' | 'stashes'

interface Leaf {
  ref: Ref
  label: string
  /** Branches stacked directly on this one. */
  kids: Leaf[]
}

interface Folder {
  key: string
  name: string
  folders: Folder[]
  leaves: Leaf[]
  /** Branches here and below, including stacked ones. */
  total: number
  holdsHead: boolean
  /** The kind marking this folder, when its depth carries marks and its branches share one. */
  kind?: string
  remote: boolean
}

export interface PanelTree {
  key: PanelKey
  root: Folder
}

export type Panels = Record<PanelKey, PanelTree>

interface RowBase {
  depth: number
  label: string
  /** The first row of a top-level group, after the panel's first row: draws a separator. */
  startsGroup: boolean
}

export interface FolderRow extends RowBase {
  type: 'folder'
  /** Stable across reloads and unique across panels, for remembering open state. */
  key: string
  open: boolean
  /** Branches inside, or matching branches inside while filtering. */
  count: number
  holdsHead: boolean
  kind?: string
  remote: boolean
}

export interface RefRow extends RowBase {
  type: 'ref'
  ref: Ref
  /** Sits under the branch it's stacked on, which is why it shows its full name. */
  stacked: boolean
  /** Doesn't match itself, but a stacked branch above it in the stack needs it shown. */
  context: boolean
  /** The kind to mark on this row; undefined when a folder above already says it. */
  mark?: string
}

export type TreeRow = FolderRow | RefRow

export interface RowOptions {
  /** Open state the person chose, by folder key; other folders use the default. */
  open?: Readonly<Record<string, boolean>>
  query?: string
  /** Keep only branches that need care. Only the Branches panel has them. */
  care?: boolean
}

/** A branch whose upstream was deleted, or whose stack parent was merged. */
export function needsCare(r: Ref): boolean {
  return r.gone || !!r.stackParentMerged
}

const collator = new Intl.Collator(undefined, { numeric: true })

/** Tags and release branches sort newest first; everything else A to Z. */
function sortLeaves(leaves: Leaf[], panel: PanelKey): void {
  const newestFirst = (l: Leaf) => panel === 'tags' || l.ref.branchKind === 'release'
  leaves.sort((a, b) => {
    if (a.ref.head !== b.ref.head) return a.ref.head ? -1 : 1
    const order = collator.compare(a.label, b.label)
    return newestFirst(a) && newestFirst(b) ? -order : order
  })
}

/** Which branches can sit under their parent: a listed parent, and no loop on the way up. */
function stackParents(refs: Ref[]): Map<string, string> {
  const parents = new Map<string, string>()
  const names = new Set(refs.map((r) => r.short))
  for (const r of refs) if (r.stackParent && names.has(r.stackParent)) parents.set(r.short, r.stackParent)
  const settled = new Set<string>()
  for (const r of refs) {
    const path = new Set<string>()
    let at: string | undefined = r.short
    while (at !== undefined && !settled.has(at) && !path.has(at)) {
      path.add(at)
      at = parents.get(at)
    }
    // Walked back into this path: everything on it is a loop or hangs off one, so it stays
    // where its name puts it.
    if (at !== undefined && path.has(at)) for (const p of path) parents.delete(p)
    for (const p of path) settled.add(p)
  }
  return parents
}

function newFolder(panel: PanelKey, path: string, name: string, remote: boolean): Folder {
  return { key: `${panel}:${path}`, name, folders: [], leaves: [], total: 0, holdsHead: false, remote }
}

/** Counts a leaf and everything stacked on it, without recursing through long stacks. */
function measure(leaf: Leaf): { total: number; head: boolean } {
  let total = 0
  let head = false
  const pending = [leaf]
  while (pending.length) {
    const l = pending.pop()!
    total++
    head ||= l.ref.head
    pending.push(...l.kids)
  }
  return { total, head }
}

/** The one kind every branch in this folder shares, or undefined. */
function sharedKind(f: Folder): string | undefined {
  const kinds = new Set<string | undefined>()
  const visit = (x: Folder) => {
    for (const l of x.leaves) kinds.add(l.ref.branchKind || undefined)
    x.folders.forEach(visit)
  }
  visit(f)
  const [only] = kinds
  return kinds.size === 1 ? only : undefined
}

function buildPanel(panel: PanelKey, refs: Ref[]): PanelTree {
  const root = newFolder(panel, '', '', false)
  const parents = panel === 'branches' ? stackParents(refs) : new Map<string, string>()
  const leaves = new Map<string, Leaf>()
  const foldersByPath = new Map<string, Folder>()

  for (const ref of refs) {
    // A stash's subject is prose, so it stays whole; a stacked branch shows its full name.
    const nests = panel !== 'stashes' && !parents.has(ref.short)
    const parts = nests ? ref.short.split('/') : [panel === 'stashes' ? ref.subject || ref.short : ref.short]
    const leaf: Leaf = { ref, label: parts[parts.length - 1], kids: [] }
    leaves.set(ref.short, leaf)
    if (parents.has(ref.short)) continue
    let at = root
    let path = ''
    for (let i = 0; i < parts.length - 1; i++) {
      path += (path ? '/' : '') + parts[i]
      let next = foldersByPath.get(path)
      if (!next) {
        next = newFolder(panel, path, parts[i], panel === 'remotes' && i === 0)
        foldersByPath.set(path, next)
        at.folders.push(next)
      }
      at = next
    }
    at.leaves.push(leaf)
  }
  for (const [child, parent] of parents) leaves.get(parent)!.kids.push(leaves.get(child)!)
  if (panel !== 'stashes') for (const leaf of leaves.values()) sortLeaves(leaf.kids, panel)

  const kindDepth = panel === 'branches' ? 0 : panel === 'remotes' ? 1 : -1
  const finish = (f: Folder, depth: number) => {
    const kept: Folder[] = []
    for (const child of f.folders) {
      finish(child, depth + 1)
      // One branch and nothing else stays flat, except that a remote is always a folder.
      if (child.total === 1 && !child.remote) {
        const leaf = child.leaves[0]
        leaf.label = `${child.name}/${leaf.label}`
        f.leaves.push(leaf)
      } else {
        kept.push(child)
      }
    }
    f.folders = kept.sort((a, b) => collator.compare(a.name, b.name))
    if (panel !== 'stashes') sortLeaves(f.leaves, panel)
    f.total = 0
    for (const l of f.leaves) {
      const m = measure(l)
      f.total += m.total
      f.holdsHead ||= m.head
    }
    for (const c of f.folders) {
      f.total += c.total
      f.holdsHead ||= c.holdsHead
    }
    if (depth + 1 === kindDepth) for (const c of f.folders) c.kind = sharedKind(c)
  }
  // The root isn't shown, so its children are the top level, depth 0.
  finish(root, -1)
  return { key: panel, root }
}

/** Sorts refs into panels and builds each one's tree. Hidden branches stay out unless shown. */
export function buildPanels(refs: Ref[], showHidden = false): Panels {
  const byPanel: Record<PanelKey, Ref[]> = { branches: [], remotes: [], tags: [], stashes: [] }
  const panelOf = { local: 'branches', remote: 'remotes', tag: 'tags', stash: 'stashes' } as const
  for (const r of refs) if (showHidden || !r.hidden) byPanel[panelOf[r.kind]].push(r)
  return {
    branches: buildPanel('branches', byPanel.branches),
    remotes: buildPanel('remotes', byPanel.remotes),
    tags: buildPanel('tags', byPanel.tags),
    stashes: buildPanel('stashes', byPanel.stashes),
  }
}

/** Decides which leaves match a view, and which folders and parents that keeps showing. */
function viewOf(panel: PanelKey, query: string, care: boolean) {
  const q = query.trim().toLowerCase()
  const careOn = care && panel === 'branches'
  const filtering = !!q || careOn
  const matches = (l: Leaf) => {
    if (careOn && !needsCare(l.ref)) return false
    if (!q) return true
    const r = l.ref
    return r.short.toLowerCase().includes(q) || (r.kind === 'stash' && !!r.subject?.toLowerCase().includes(q))
  }
  const leafCounts = new Map<Leaf, number>()
  const leafCount = (l: Leaf): number => {
    let n = leafCounts.get(l)
    if (n === undefined) {
      n = (matches(l) ? 1 : 0) + l.kids.reduce((sum, k) => sum + leafCount(k), 0)
      leafCounts.set(l, n)
    }
    return n
  }
  const folderCounts = new Map<Folder, number>()
  const folderCount = (f: Folder): number => {
    let n = folderCounts.get(f)
    if (n === undefined) {
      n = f.leaves.reduce((sum, l) => sum + leafCount(l), 0) + f.folders.reduce((sum, c) => sum + folderCount(c), 0)
      folderCounts.set(f, n)
    }
    return n
  }
  return { filtering, matches, leafCount, folderCount }
}

/** The rows a panel shows, top to bottom. */
export function panelRows(tree: PanelTree, options: RowOptions = {}): TreeRow[] {
  const { open = {}, query = '', care = false } = options
  const view = viewOf(tree.key, query, care)
  const rows: TreeRow[] = []

  const addLeaf = (l: Leaf, depth: number, stacked: boolean, markedKind: string | undefined, startsGroup: boolean) => {
    const kind = l.ref.branchKind || undefined
    rows.push({
      type: 'ref',
      ref: l.ref,
      label: l.label,
      depth,
      stacked,
      context: view.filtering && !view.matches(l),
      mark: kind && kind !== markedKind ? kind : undefined,
      startsGroup,
    })
    for (const k of l.kids) if (view.leafCount(k) > 0) addLeaf(k, depth + 1, true, markedKind, false)
  }

  const addFolder = (f: Folder, depth: number, markedKind: string | undefined) => {
    const shown = f.leaves.filter((l) => view.leafCount(l) > 0)
    const isTrunk = (l: Leaf) => l.ref.branchKind === 'trunk'
    for (const l of shown) if (isTrunk(l)) addLeaf(l, depth, false, markedKind, false)
    for (const c of f.folders) {
      const count = view.folderCount(c)
      if (count === 0) continue
      const isOpen = view.filtering || (open[c.key] ?? c.kind !== 'bot')
      rows.push({
        type: 'folder',
        key: c.key,
        label: c.name,
        depth,
        open: isOpen,
        count,
        holdsHead: c.holdsHead,
        kind: c.kind,
        remote: c.remote,
        startsGroup: depth === 0 && rows.length > 0,
      })
      if (isOpen) addFolder(c, depth + 1, c.kind ?? markedKind)
    }
    let first = depth === 0 && rows.length > 0
    for (const l of shown) {
      if (isTrunk(l)) continue
      addLeaf(l, depth, false, markedKind, first)
      first = false
    }
  }

  addFolder(tree.root, 0, undefined)
  return rows
}

/** How many refs in this panel match the filter, for the rail's badges. */
export function panelMatches(tree: PanelTree, query: string): number {
  return viewOf(tree.key, query, false).folderCount(tree.root)
}

/** How many branches in this panel need care. */
export function careCount(tree: PanelTree): number {
  return viewOf(tree.key, '', true).folderCount(tree.root)
}
