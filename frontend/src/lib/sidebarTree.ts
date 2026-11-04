import type { Ref } from './graph/rowCache'

/**
 * Turns refs into the rows each sidebar panel shows. The tree is built once per ref list, and
 * rows are flattened from it per view, so opening a folder or typing in the filter doesn't
 * rebuild anything.
 */

export type PanelKey = 'branches' | 'remotes' | 'tags' | 'stashes'

/** Name: A to Z, with tags and releases newest version first. Recent: last updated first. */
export type SortOrder = 'name' | 'recent'

export interface PanelOptions {
  /** Include branches the repository's hide patterns match. */
  showHidden?: boolean
  sort?: SortOrder
}

interface Leaf {
  ref: Ref
  label: string
  /** Branches stacked directly on this one. */
  kids: Leaf[]
  /** This branch and those stacked on it, however deep. */
  total: number
  holdsHead: boolean
  newest: number
}

interface Folder {
  key: string
  name: string
  folders: Folder[]
  leaves: Leaf[]
  /** Branches here and below, including stacked ones. */
  total: number
  holdsHead: boolean
  newest: number
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
  /** A stack of branches rather than a folder of them. */
  stack?: boolean
  /** A word on what's inside, beside the count. */
  note?: string
  /** A stack's branches, all of them whatever a filter shows, the top first. */
  members?: Ref[]
}

export interface RefRow extends RowBase {
  type: 'ref'
  ref: Ref
  /** Sits under the branch it's stacked on, which is why it shows its full name. */
  stacked: boolean
  /** Doesn't match itself, but a stacked branch above it in the stack needs it shown. */
  context: boolean
  /** A member of a stack's group, joined to the others by a line. */
  railed?: boolean
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
  /** Branches known to be merged, by name. A stack of nothing else starts closed. */
  merged?: Readonly<Record<string, string>>
}

/** A stack this size gets a row of its own. Below it, a child indented under its parent reads
 * fine, and a row for the pair would only add to the list. */
const STACK_GROUP_SIZE = 3

/** A branch and those stacked on it, the top of the stack first, as the graph has them. */
const stackMembers = (l: Leaf): Leaf[] => [...l.kids.flatMap(stackMembers), l]

/** A branch whose upstream was deleted, or whose stack parent was merged. */
export function needsCare(r: Ref): boolean {
  return r.gone || !!r.stackParentMerged
}

const collator = new Intl.Collator(undefined, { numeric: true })

/** The checked-out branch or its stack first, then by the chosen order. By name, tags and
 * release branches run newest version first, and everything else A to Z. */
function sortLeaves(leaves: Leaf[], panel: PanelKey, sort: SortOrder): void {
  const newestFirst = (l: Leaf) => panel === 'tags' || l.ref.branchKind === 'release'
  leaves.sort((a, b) => {
    if (a.holdsHead !== b.holdsHead) return a.holdsHead ? -1 : 1
    if (sort === 'recent' && a.newest !== b.newest) return b.newest - a.newest
    const order = collator.compare(a.label, b.label)
    return newestFirst(a) && newestFirst(b) ? -order : order
  })
}

function sortFolders(folders: Folder[], sort: SortOrder): void {
  folders.sort((a, b) => {
    if (a.holdsHead !== b.holdsHead) return a.holdsHead ? -1 : 1
    if (sort === 'recent' && a.newest !== b.newest) return b.newest - a.newest
    return collator.compare(a.name, b.name)
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
  return { key: `${panel}:${path}`, name, folders: [], leaves: [], total: 0, holdsHead: false, newest: 0, remote }
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

function buildPanel(panel: PanelKey, refs: Ref[], sort: SortOrder): PanelTree {
  const root = newFolder(panel, '', '', false)
  const parents = panel === 'branches' ? stackParents(refs) : new Map<string, string>()
  const leaves = new Map<string, Leaf>()
  const foldersByPath = new Map<string, Folder>()

  for (const ref of refs) {
    // A stash's subject is prose, so it stays whole; a stacked branch shows its full name.
    const nests = panel !== 'stashes' && !parents.has(ref.short)
    const parts = nests ? ref.short.split('/') : [panel === 'stashes' ? ref.subject || ref.short : ref.short]
    const leaf: Leaf = { ref, label: parts[parts.length - 1], kids: [], total: 1, holdsHead: ref.head, newest: ref.updated ?? 0 }
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
  // Parents take on what's stacked on them, so a whole stack rises with its newest branch.
  const settle = (l: Leaf) => {
    for (const k of l.kids) {
      settle(k)
      l.total += k.total
      l.holdsHead ||= k.holdsHead
      l.newest = Math.max(l.newest, k.newest)
    }
    sortLeaves(l.kids, panel, sort)
  }
  for (const leaf of leaves.values()) if (!parents.has(leaf.ref.short)) settle(leaf)

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
    f.folders = kept
    sortFolders(f.folders, sort)
    // Git already lists stashes newest first.
    if (panel !== 'stashes') sortLeaves(f.leaves, panel, sort)
    f.total = 0
    for (const x of [...f.leaves, ...f.folders]) {
      f.total += x.total
      f.holdsHead ||= x.holdsHead
      f.newest = Math.max(f.newest, x.newest)
    }
    if (depth + 1 === kindDepth) for (const c of f.folders) c.kind = sharedKind(c)
  }
  // The root isn't shown, so its children are the top level, depth 0.
  finish(root, -1)
  return { key: panel, root }
}

/** Sorts refs into panels and builds each one's tree. Hidden branches stay out unless shown. */
export function buildPanels(refs: Ref[], { showHidden = false, sort = 'name' }: PanelOptions = {}): Panels {
  const byPanel: Record<PanelKey, Ref[]> = { branches: [], remotes: [], tags: [], stashes: [] }
  const panelOf = { local: 'branches', remote: 'remotes', tag: 'tags', stash: 'stashes' } as const
  for (const r of refs) if (showHidden || !r.hidden) byPanel[panelOf[r.kind]].push(r)
  return {
    branches: buildPanel('branches', byPanel.branches, sort),
    remotes: buildPanel('remotes', byPanel.remotes, sort),
    tags: buildPanel('tags', byPanel.tags, sort),
    stashes: buildPanel('stashes', byPanel.stashes, sort),
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
  const { open = {}, query = '', care = false, merged = {} } = options
  const view = viewOf(tree.key, query, care)
  const rows: TreeRow[] = []

  const addStack = (l: Leaf, depth: number, markedKind: string | undefined, startsGroup: boolean) => {
    const members = stackMembers(l)
    const allMerged = members.every((m) => merged[m.ref.short])
    const key = `${tree.key}:stack:${l.ref.short}`
    const isOpen = view.filtering || (open[key] ?? (l.holdsHead || !allMerged))
    rows.push({
      type: 'folder',
      key,
      label: l.label,
      depth,
      open: isOpen,
      count: view.leafCount(l),
      holdsHead: l.holdsHead,
      remote: false,
      stack: true,
      note: allMerged ? 'merged' : undefined,
      members: members.map((m) => m.ref),
      startsGroup,
    })
    if (!isOpen) return
    for (const m of members) {
      if (view.leafCount(m) === 0) continue
      const kind = m.ref.branchKind || undefined
      rows.push({
        type: 'ref',
        ref: m.ref,
        label: m.label,
        depth: depth + 1,
        stacked: m !== l,
        railed: true,
        context: view.filtering && !view.matches(m),
        mark: kind && kind !== markedKind ? kind : undefined,
        startsGroup: false,
      })
    }
  }

  const addLeaf = (l: Leaf, depth: number, stacked: boolean, markedKind: string | undefined, startsGroup: boolean) => {
    if (!stacked && l.total >= STACK_GROUP_SIZE) {
      addStack(l, depth, markedKind, startsGroup)
      return
    }
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
    const folders = f.folders.filter((c) => view.folderCount(c) > 0)
    const groupStart = () => depth === 0 && rows.length > 0

    const addChild = (c: Folder) => {
      const isOpen = view.filtering || (open[c.key] ?? c.kind !== 'bot')
      rows.push({
        type: 'folder',
        key: c.key,
        label: c.name,
        depth,
        open: isOpen,
        count: view.folderCount(c),
        holdsHead: c.holdsHead,
        kind: c.kind,
        remote: c.remote,
        startsGroup: groupStart(),
      })
      if (isOpen) addFolder(c, depth + 1, c.kind ?? markedKind)
    }
    const addGroup = (leaves: Leaf[]) => {
      let first = groupStart()
      for (const l of leaves) {
        addLeaf(l, depth, false, markedKind, first)
        first = false
      }
    }

    // The checked-out branch, or the folder or stack holding it, always leads.
    const headLeaf = shown.find((l) => l.holdsHead)
    const headFolder = folders.find((c) => c.holdsHead)
    const rest = shown.filter((l) => l !== headLeaf)
    if (headLeaf) addLeaf(headLeaf, depth, false, markedKind, false)
    if (headFolder) addChild(headFolder)
    addGroup(rest.filter((l) => l.ref.branchKind === 'trunk'))
    for (const c of folders) if (c !== headFolder) addChild(c)
    addGroup(rest.filter((l) => l.ref.branchKind !== 'trunk'))
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

export interface RefStatus {
  /** What the row shows. */
  text: string
  /** The same thing in words, for screen readers. */
  spoken: string
  tone: 'warning' | 'muted' | 'link'
}

/** The one status a row shows, most pressing first. The tooltip lists the rest. mergedInto is
 * the branch already holding this one's work, if any: then there's nothing left to do but
 * delete it, whatever else is true. */
export function refStatus(r: Ref, mergedInto?: string): RefStatus | undefined {
  if (mergedInto) return { text: 'merged', spoken: `merged into ${mergedInto}`, tone: 'muted' }
  if (r.stackParentMerged) return { text: 'parent merged', spoken: 'parent merged', tone: 'warning' }
  if (r.gone) return { text: 'gone', spoken: 'upstream gone', tone: 'muted' }
  if (r.ahead || r.behind) {
    const text = (r.ahead ? `↑${r.ahead}` : '') + (r.behind ? `↓${r.behind}` : '')
    const spoken = [r.ahead ? `${r.ahead} ahead` : '', r.behind ? `${r.behind} behind` : ''].filter(Boolean).join(', ')
    return { text, spoken, tone: 'link' }
  }
  return undefined
}

/** Where a branch sits in its stack, in words, or '' when it isn't in one. */
export function stackPlace(r: Ref): string {
  if (!r.stackDepth || !r.stackSize) return ''
  const place = `${r.stackDepth} of ${r.stackSize}`
  return r.stackParent ? `Stacked on ${r.stackParent} (${place})` : `The bottom of a stack (${place})`
}

/** Everything known about a row, one fact per line, since the row itself shows one status. */
export function refTooltip(r: Ref, mergedInto?: string): string {
  const lines = [r.kind === 'stash' ? r.subject || r.short : r.short]
  if (r.head) lines.push('Checked out')
  if (r.worktree) lines.push(`Checked out in the worktree at ${r.worktree}`)
  if (mergedInto) lines.push(`Its work is already on ${mergedInto}, so it's safe to delete`)
  else if (r.stackParentMerged) lines.push(`${r.stackParent} was merged. Right-click, Restack, to move this branch onto the Trunk.`)
  if (stackPlace(r)) lines.push(stackPlace(r))
  if (r.gone) lines.push('Upstream branch was deleted')
  else if (r.ahead || r.behind) lines.push(`${r.ahead} ahead, ${r.behind} behind ${r.upstream}`)
  if (r.focused) lines.push('The graph is focused on this')
  if (r.hidden) lines.push('Hidden: shown for now')
  return lines.join('\n')
}

/** Splits a row's label around the filter's match, for highlighting. No match in the label
 * (it can match earlier in the full name) leaves it all in pre. */
export function matchParts(label: string, query: string): { pre: string; hit: string; post: string } {
  const q = query.trim().toLowerCase()
  const i = q ? label.toLowerCase().indexOf(q) : -1
  if (i < 0) return { pre: label, hit: '', post: '' }
  return { pre: label.slice(0, i), hit: label.slice(i, i + q.length), post: label.slice(i + q.length) }
}
