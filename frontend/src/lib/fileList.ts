/** A run of files in one folder, in the order they came. */
export interface FileGroup<F> {
  /** The folder's path, or '' for the top of the repository. */
  folder: string
  files: F[]
}

export const fileName = (path: string) => path.slice(path.lastIndexOf('/') + 1)

export const fileFolder = (path: string) => path.slice(0, Math.max(0, path.lastIndexOf('/')))

/** Groups files by folder, so a list reads by filename. Folders keep the order they first
 * appear in, and a folder's files stay together even where a subfolder sorts among them. */
export function fileGroups<F extends { path: string }>(files: F[]): FileGroup<F>[] {
  const groups = new Map<string, FileGroup<F>>()
  for (const f of files) {
    const folder = fileFolder(f.path)
    const group = groups.get(folder)
    if (group) group.files.push(f)
    else groups.set(folder, { folder, files: [f] })
  }
  return [...groups.values()]
}

/** A row of a file list: a folder heading, or a file. */
export type FileRow<F> = { type: 'folder'; key: string; name: string; path: string; depth: number; open: boolean; collapsible: boolean } | { type: 'file'; key: string; file: F; depth: number }

export interface FileRowOptions {
  /** Keeps only files whose path has this in it, ignoring case. */
  query?: string
  /** Nests folders one level at a time. Otherwise each folder is one heading with its whole path. */
  tree?: boolean
  /** In a tree, the folders that are closed, by path. */
  closed?: ReadonlySet<string>
}

/** The rows of a file list. The filter opens every folder, so a match is never left hidden. */
export function fileRows<F extends { path: string }>(files: F[], { query = '', tree = false, closed = new Set() }: FileRowOptions = {}): FileRow<F>[] {
  const q = query.trim().toLowerCase()
  const shown = q ? files.filter((f) => f.path.toLowerCase().includes(q)) : files
  const rows: FileRow<F>[] = []
  if (!tree) {
    for (const g of fileGroups(shown)) {
      if (g.folder) rows.push({ type: 'folder', key: `folder:${g.folder}`, name: `${g.folder}/`, path: g.folder, depth: 0, open: true, collapsible: false })
      for (const file of g.files) rows.push({ type: 'file', key: file.path, file, depth: 0 })
    }
    return rows
  }
  const seen = new Set<string>()
  for (const file of shown) {
    const parts = file.path.split('/')
    let folder = ''
    let hidden = false
    for (let depth = 0; depth < parts.length - 1; depth++) {
      folder = folder ? `${folder}/${parts[depth]}` : parts[depth]
      const open = !!q || !closed.has(folder)
      if (!hidden && !seen.has(folder)) {
        seen.add(folder)
        rows.push({ type: 'folder', key: `folder:${folder}`, name: parts[depth], path: folder, depth, open, collapsible: true })
      }
      hidden ||= !open
    }
    if (!hidden) rows.push({ type: 'file', key: file.path, file, depth: parts.length - 1 })
  }
  return rows
}

/** Lists shorter than this have nothing to gain from a filter. */
export const FILTER_FROM = 5

const TREE_KEY = 'gitrevenge.fileTree'

export function readTreeChoice(): boolean {
  try {
    return localStorage.getItem(TREE_KEY) === 'on'
  } catch {
    return false
  }
}

export function saveTreeChoice(tree: boolean) {
  try {
    localStorage.setItem(TREE_KEY, tree ? 'on' : 'off')
  } catch {
    // Storage unavailable; the choice lasts for this session only.
  }
}

/** The file before or after the open one in a list, if there is one. */
export function adjacentFile<F extends { path: string }>(rows: FileRow<F>[], current: string | undefined, step: 1 | -1): F | undefined {
  const files = rows.flatMap((r) => (r.type === 'file' ? [r.file] : []))
  const at = files.findIndex((f) => f.path === current)
  return at < 0 ? undefined : files[at + step]
}

/** Arrow keys, Home and End move between a list's file rows (marked data-file-row). */
export function moveThroughRows(e: KeyboardEvent, list: HTMLElement) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key) || e.altKey) return
  const rows = [...list.querySelectorAll<HTMLElement>('[data-file-row]')]
  const at = rows.indexOf(e.target as HTMLElement)
  if (at < 0) return
  const to = e.key === 'Home' ? 0 : e.key === 'End' ? rows.length - 1 : at + (e.key === 'ArrowDown' ? 1 : -1)
  e.preventDefault()
  rows[to]?.focus()
}
